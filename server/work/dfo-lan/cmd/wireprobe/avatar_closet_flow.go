package main

import (
	"context"
	"encoding/json"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
)

// Avatar Closet（寄存衣柜）的 C→S 命令处理：
//   - cmd 1103 ENUM_CMDPACKET_SELECT_AVATAR_CLOSET   （切换衣橱栏位/套装）
//   - cmd 1102 ENUM_CMDPACKET_CHANGE_AVATAR_CLOSET_SET（提交当前栏位号）
//
// 反汇编客户端（DFO.exe）的 opcode→handler 注册表（0x14003c650）确认：
//   - 1103 的解析器读 1 个字节（选中的栏位号）写入衣柜对象 +0x10，并打开 UI 0x225；
//   - 1102 的解析器不读字段。通用命令分发先消费一个“成功字节”，故回包体 = [0x01] + 字段。
//
// 选中套号与柜**内容**都落角色存档（avatar_closet_selected / avatar_closet_ledger），
// 登录时分别经 NOTI1076 第 2 个 u32 与 NOTI1077 条目回给客户端（缺口①②），
// 治"重登选中栏跳回第一套""重登衣柜空了、衣柜之间会串"。
func (client *gameConnection) dispatchAvatarCloset(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !requestData.verified {
		return dispatchNext
	}
	switch requestData.frame.ID {
	case 1103:
		var selected byte
		if len(requestData.plaintext) > 0 {
			selected = requestData.plaintext[0]
		}
		// 落库先于 ACK（2026-10-14 缺口①）：上游只 ACK 不存 ⇒ 重登选中栏必跳回
		// 第一套。现在选中套号无条件进存档，登录时经 NOTI1076 第 2 个 u32 回给客户端。
		// 落库失败只记事件，绝不影响 ACK。
		client.persistAvatarClosetSelected(selected)
		if err := client.output.send(1, 1103, []byte{1, selected}); err != nil {
			client.event(map[string]any{"kind": "avatar_closet_select_error", "error": err.Error()})
			return dispatchClose
		}
		client.event(map[string]any{"kind": "avatar_closet_selected", "selected": selected})
		return dispatchHandled
	case 1102:
		// CMD1102 = 衣柜「change」按钮：把选中套的寄存件穿到身上、把身上那套换进柜子。
		// 客户端链路与帧语义见 avatar_closet_change.go 头注（反汇编钉死：包体 1 字节套号，
		// 回包 handler 不读任何字段、只清 pending）。
		// ★ 先 ACK 再下发状态帧（与装备穿戴 equipment_flow.go 同序）：ACK 是客户端
		//   生命线，换装计划失败时也只记事件、绝不改存档。
		change := client.avatarClosetChange(requestData)
		if err := client.output.send(1, 1102, []byte{1}); err != nil {
			client.event(map[string]any{"kind": "avatar_closet_set_error", "error": err.Error()})
			return dispatchClose
		}
		if len(change) > 0 {
			// 带 body 日志回调：换装帧必须能在 events.jsonl 里逐帧对账（业主实机验收用）。
			if e := client.sendPlan(change, client.logWorldResponseBody); e != nil {
				client.event(map[string]any{"kind": "avatar_closet_change_send_error", "error": e.Error()})
				return dispatchClose
			}
		}
		// ★ 这里**不补推 NOTI1077**（2026-10-14 缺口②定案）：反汇编客户端
		//   reader（0x1449e8100 + 0x145af7970）证实 mode 1 条目是"柜槽↔装扮栏槽"
		//   的搬运——登录帧已把货存进柜槽，此刻客户端柜槽有货，再推同一批条目会把
		//   货原路搬回装扮栏，衣柜重新变空（业主报的"重登衣柜空了"同款事故）。
		//   柜内容只在登录下发一次；会话内的拖进/拖出客户端本地已生效，服务端记账即可。
		client.event(map[string]any{"kind": "avatar_closet_set_acked"})
		return dispatchHandled
	}
	return dispatchNext
}

// avatarClosetMoveAckOnly：客户端把时装拖进衣柜走通用 cmd19（目标 List=0x21=33）。
// 回一个成功 ACK —— **绝不发 NOTI13(space 33) 容器帧**（客户端遇到不认识的容器
// 空间号会崩；2026-10-10 事故）。
//
// 2026-10-13 记账（业主："从零开始修时装橱窗，第一步是无法保存"）：
// 上游原本只 ACK、不落任何东西 ⇒ 服务端完全不知道玩家的柜子长什么样。
// 现在在 ACK 之前把这一笔**无条件记进角色存档**（avatar_closet_ledger）。
// ★ 记账失败**只记事件、绝不影响 ACK** —— ACK 是客户端生命线，账可以补，连接不能断。
// ★ 这里一帧 wire 都不多发：柜内容的重新下发是第二步，且必须先拿到真实帧形状
//   （官方 space33 空帧 8B，我们旧实现的 184B 头部字段数不对，实测卡进图）。
const avatarClosetProbeSpace byte = 33

func (client *gameConnection) avatarClosetMoveAckOnly(requestData *clientRequest) (bool, error) {
	if requestData.frame.ID != 19 {
		return false, nil
	}
	r, err := protocol.DecodeItemMove(requestData.plaintext)
	if err != nil || (r.SourceList != avatarClosetProbeSpace && r.DestinationList != avatarClosetProbeSpace) {
		return false, nil
	}
	// 记账先于 ACK：账进了存档，这一笔才算真的发生了。
	client.recordAvatarClosetMove(requestData, r)
	if e := client.output.send(1, 19, protocol.ItemMoveSuccess(r, 1)); e != nil {
		return true, e
	}
	client.event(map[string]any{"kind": "avatar_closet_move_ack_only", "src": r.SourceList,
		"src_slot": r.SourceSlot, "dst": r.DestinationList, "dst_slot": r.DestinationSlot,
		"item": r.SourceItem})
	return true, nil
}

// recordAvatarClosetMove 把一次橱窗摆设落进角色存档。**无条件执行**（无开关）：
// 摆设记录是玩家资产，10-12 的旧方案把它放在开关后面，导致玩家在开关关闭期间摆的
// 件全部丢失（业主 17:36 摆了 18 件，全丢）。失败只记 avatar_closet_ledger_error。
func (client *gameConnection) recordAvatarClosetMove(requestData *clientRequest, r protocol.ItemMoveRequest) {
	if client.gameStore == nil || client.worldState == nil {
		return
	}
	op, ok := avatarClosetLedgerMoveKind(r)
	if !ok {
		return
	}
	closetSlot, bagSlot := avatarClosetLedgerSlots(r, op)
	role := client.worldState.role
	raw, _ := json.Marshal(requestData.plaintext)
	key := avatarClosetLedgerKey(int64(role.WireID), avatarClosetLedgerLen(role.State), raw)
	ctx := context.Background()
	// backingErr：兜底行表同步失败（与账本同一事务，但绝不因它回滚记账）。
	var backingErr error
	var backingChanged bool
	saved, _, err := client.gameStore.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion,
		key, avatarClosetLedgerModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			ledger, readErr := readAvatarClosetLedger(current.State)
			if readErr != nil {
				return nil, nil, readErr
			}
			next, applied, appErr := appendAvatarClosetLedger(ledger, avatarClosetLedgerEntry{
				Op:         op,
				ClosetSlot: closetSlot,
				BagSlot:    bagSlot,
				Item:       r.SourceItem,
				WireID:     role.WireID,
				At:         time.Now().UTC().Format(time.RFC3339Nano),
			})
			if appErr != nil {
				return nil, nil, appErr
			}
			if !applied {
				// 与末笔完全相同（客户端重发）⇒ 幂等，不改存档。
				return current.State, json.RawMessage(`{"applied":false}`), nil
			}
			state, writeErr := writeAvatarClosetLedger(current.State, next)
			if writeErr != nil {
				return nil, nil, writeErr
			}
			// 兜底行（"洞位"覆盖隐患根治，2026-10-14 下午）：与账本同一事务。
			// 失败不阻断——账本已写进 state，客户端这一笔本地已生效，不能回滚。
			state, backingChanged, backingErr = syncAvatarClosetBacking(state, next, op, closetSlot, bagSlot, r.SourceItem)
			receipt, _ := json.Marshal(map[string]any{"applied": true,
				"seq": next.Items[len(next.Items)-1].Seq, "op": op, "backing": backingChanged})
			return state, receipt, nil
		})
	if err != nil {
		client.event(map[string]any{"kind": "avatar_closet_ledger_error",
			"op": op, "closet_slot": closetSlot, "bag_slot": bagSlot, "error": err.Error()})
		return
	}
	if backingErr != nil {
		// 账本次 committed（上面的 err==nil）；备份没同步上只会让下次登录少一层
		// 兜底，绝不串货——条目的主路径永远是装扮栏现查。
		client.event(map[string]any{"kind": "avatar_closet_backing_error",
			"op": op, "closet_slot": closetSlot, "bag_slot": bagSlot, "error": backingErr.Error()})
	}
	// 落库成功后必须回写会话内的 role：幂等键带**账本条数**前缀，不回写的话
	// 同一会话第二笔的键与第一笔重合，被事件层当重发放掉 ⇒ 玩家连摆两件只记一笔
	// （2026-10-14 缺口②修复时同款 w.role = saved 纪律，见 avatar_closet_selected.go）。
	// WireID 是运行时字段（DB 行不带），显式带上，别让回写把它清零。
	if saved.ID != 0 {
		saved.WireID = role.WireID
		client.worldState.role = saved
	}
	client.event(map[string]any{"kind": "avatar_closet_ledger_applied", "op": op,
		"closet_slot": closetSlot, "bag_slot": bagSlot, "item": r.SourceItem})
	// 规矩：**clone 时装不能放进时装柜**（业主 2026-10-14）。客户端 UI 本来就不给拖，
	// 真出现说明有别的入口；这里只做诊断 —— 件照旧记进账本（绝不丢），但要能在
	// events.jsonl 里一眼看到。
	if op == avatarClosetLedgerPut && client.avatarIsClone(r.SourceItem) {
		client.event(map[string]any{"kind": "avatar_closet_clone_put", "closet_slot": closetSlot,
			"bag_slot": bagSlot, "item": r.SourceItem})
	}
}

// avatarClosetLedgerLen 只读地取当前账本条数（用于幂等键的前置摘要）。
// 读失败按 0 处理 —— 那只会让幂等键少一层区分，不影响记账正确性。
func avatarClosetLedgerLen(state json.RawMessage) int {
	ledger, err := readAvatarClosetLedger(state)
	if err != nil {
		return 0
	}
	return len(ledger.Items)
}
