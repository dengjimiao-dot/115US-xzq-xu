package main

import (
	"context"
	"encoding/json"
	"fmt"

	"dfolan/internal/database"
)

// 衣柜选中套号的持久化（2026-10-14，缺口①："重登选中栏跳回第一套"）。
//
// 客户端切套装走 CMD1103（SELECT_AVATAR_CLOSET，1 字节 = 套装号，反汇编
// DFO.exe 0x1449E7F90 实证），上游默认实现只回 ACK 不落盘 ⇒ 登录时 NOTI1076
// 的第 2 个 u32 恒 0，客户端把选中栏重置回第一套。
//
// 本文件把选中套号存进角色存档顶层键 avatar_closet_selected，登录时喂给
// protocol.AvatarClosetInfo 的第二个 u32（官抓该位就是 0，客户端 handler
// 0x1449E8060 读它写进衣柜对象 +0x10）。
//
// ★ 与 ledger 同一纪律：无条件执行（无开关）、失败只记事件绝不阻塞 ACK。
// ★ 幂等键为什么带自增序号（而不是帧摘要或 from/to）：
//   帧摘要(ledger 旧坑)与 from/to 都分不清"网络重发"和"玩家绕一圈 1→0→1"
//   —— 后者的请求体与前一次完全相同、连播放前现值都一样。只有**单调递增**
//   的序号对每次真实落库都不同，重发（序号没动）才天然同 key。
const avatarClosetSelectedModel = "avatar-closet-selected-v1"

// avatarClosetSelectedKey 幂等键：`<wireID>:<序号>`。序号取自播放前存档，
// 只在真实落库时 +1 ⇒ 重发同 key、绕一圈换 key、跨角色不串。
func avatarClosetSelectedKey(wireID int64, seq uint64) string {
	return fmt.Sprintf("avatar-closet-selected:%d:%d", wireID, seq)
}

// readAvatarClosetSelected 读当前选中的套装号与单调序号。
// 没有键 ⇒ (0, 0)：第一套、序号从零开始。
func readAvatarClosetSelected(state json.RawMessage) (selected byte, seq uint64, err error) {
	var probe struct {
		Selected json.RawMessage `json:"avatar_closet_selected"`
		Seq      json.RawMessage `json:"avatar_closet_selected_seq"`
	}
	if err := json.Unmarshal(state, &probe); err != nil {
		return 0, 0, err
	}
	if len(probe.Selected) > 0 {
		if err := json.Unmarshal(probe.Selected, &selected); err != nil {
			return 0, 0, err
		}
	}
	if len(probe.Seq) > 0 {
		if err := json.Unmarshal(probe.Seq, &seq); err != nil {
			return 0, 0, err
		}
	}
	return selected, seq, nil
}

// writeAvatarClosetSelected 把选中套号与序号写回角色存档顶层键（不动 inventory）。
func writeAvatarClosetSelected(state json.RawMessage, selected byte, seq uint64) (json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(state, &root); err != nil {
		return nil, err
	}
	sel, err := json.Marshal(selected)
	if err != nil {
		return nil, err
	}
	next, err := json.Marshal(seq)
	if err != nil {
		return nil, err
	}
	root["avatar_closet_selected"] = sel
	root["avatar_closet_selected_seq"] = next
	return json.Marshal(root)
}

// persistAvatarClosetSelected 把 CMD1103 提交的选中套号落进角色存档。
// ★ 先落库再回 ACK（调用方保证）：存档认了，这一笔才算真的发生了。
// ★ 落库成功后刷新会话内的 role.State，否则同一会话里连改两次会读到旧序号、
//   第二次的幂等键与第一次重合、被事件层吞掉（预设券流程同款 w.role = saved 纪律）。
func (client *gameConnection) persistAvatarClosetSelected(selected byte) {
	if client.gameStore == nil || client.worldState == nil {
		return
	}
	role := client.worldState.role
	// from/seq = 播放前的存档现值（只读，失败按零 —— 只让幂等键少一层区分）。
	_, seq, _ := readAvatarClosetSelected(role.State)
	key := avatarClosetSelectedKey(int64(role.WireID), seq)
	ctx := context.Background()
	saved, _, err := client.gameStore.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion,
		key, avatarClosetSelectedModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			kept, currentSeq, readErr := readAvatarClosetSelected(current.State)
			if readErr != nil {
				return nil, nil, readErr
			}
			value := kept
			if kept != selected {
				value = selected
			}
			// 序号**每次 apply 都 +1**（哪怕值没变）：这次提交的 key 必须被消费掉，
			// 否则下一次真实变更会复用同一个 key、被事件层吞掉。
			state, writeErr := writeAvatarClosetSelected(current.State, value, currentSeq+1)
			if writeErr != nil {
				return nil, nil, writeErr
			}
			receipt, _ := json.Marshal(map[string]any{"applied": kept != selected, "selected": value})
			return state, receipt, nil
		})
	if err != nil {
		client.event(map[string]any{"kind": "avatar_closet_selected_error",
			"selected": selected, "error": err.Error()})
		return
	}
	if saved.ID != 0 {
		client.worldState.role = saved
	}
	client.event(map[string]any{"kind": "avatar_closet_selected_saved", "selected": selected})
}
