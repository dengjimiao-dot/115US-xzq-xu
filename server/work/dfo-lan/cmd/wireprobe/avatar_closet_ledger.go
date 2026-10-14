package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"dfolan/internal/game/protocol"
)

// 时装橱窗记账（2026-10-13，业主："从零开始修时装橱窗，第一步是无法保存"）。
//
// 上游行为：客户端把时装拖进橱窗走通用 cmd19（目标 List=0x21=33），服务端只回 ACK、
// 不落任何东西。于是"玩家把哪一件放进了哪个橱窗槽"在服务端是空白 —— 服务端无从
// 知道玩家的柜子长什么样，这正是"时装橱窗无法保存"的直接原因。
//
// ★ 这一步**只记账，一帧 wire 都不多发**。记账与 wire 彻底分开，理由是：
//   - 记账是纯服务端状态，失败了只影响后续功能，不会让客户端卡住或闪退；
//   - 而 wire 下发一旦形状错了，客户端会卡在进图（2026-10-13 17:31 实测：登录下发
//     48 帧 space 33 后角色进不去赛丽亚房间，且官方空帧是 8B、我们的是 184B，
//     头部字段数根本不一致）⇒ 那是第二步的事，且必须先拿到真实帧形状。
//
// ★ 记账**无条件执行**（不受任何开关/配置控制）：摆设记录是**玩家资产**。
//   2026-10-12 的旧方案把落库放在开关后面，结果开关关闭期间玩家摆了 18 件全丢，
//   之后打开开关又按空表下发 ⇒ 柜子被清空（就是业主报的"重登会消失、柜子之间会串"）。
//   这条教训直接决定了本文件没有开关。
const avatarClosetLedgerModel = "avatar-closet-ledger-v1"

// avatarClosetLedgerOp 是摆设方向。
const (
	avatarClosetLedgerPut    byte = 1 // 放进橱窗（1 → 33）
	avatarClosetLedgerTake   byte = 2 // 从橱窗取出（33 → 1）
	avatarClosetLedgerSwap33 byte = 3 // 橱窗内部换位（33 → 33）
)

// avatarClosetLedgerEntry 是一次橱窗摆设的流水。
//
// Item 原样保存客户端报上来的制品 id（u32，含克隆/染色/日期等成分）—— 这里**不解释**
// 它的字节含义：解释需要帧格式证据（第二步才做），提前猜会再次踩坑。
type avatarClosetLedgerEntry struct {
	Seq        uint64 `json:"seq"`
	Op         byte   `json:"op"`
	ClosetSlot uint16 `json:"closet_slot"`
	BagSlot    uint16 `json:"bag_slot"`
	Item       uint32 `json:"item"`
	WireID     uint16 `json:"wire_id"`
	At         string `json:"at"`
}

// avatarClosetLedger 是账本。Items **保序**（玩家的操作顺序本身是信息：
// "哪一件后来占了哪个槽"要靠它还原），所以用切片而不是 map。
type avatarClosetLedger struct {
	Version int                       `json:"version"`
	Items   []avatarClosetLedgerEntry `json:"items"`
}

// avatarClosetLedgerMoveKind 判断一次搬移是否属于橱窗摆设（并给出方向）。
// 只有 1↔33 与 33↔33 才算；其余搬移（0→1、1→0、7→7…）与本账本无关。
func avatarClosetLedgerMoveKind(r protocol.ItemMoveRequest) (byte, bool) {
	switch {
	case r.SourceList == 1 && r.DestinationList == 33:
		return avatarClosetLedgerPut, true
	case r.SourceList == 33 && r.DestinationList == 1:
		return avatarClosetLedgerTake, true
	case r.SourceList == 33 && r.DestinationList == 33:
		return avatarClosetLedgerSwap33, true
	}
	return 0, false
}

// avatarClosetLedgerSlots 把搬移的源/目的映射成"橱窗槽 / 装扮栏行"。
// 放进：源是装扮栏行、目的是橱窗槽；取出：反过来；柜内换位：两个都是橱窗槽。
func avatarClosetLedgerSlots(r protocol.ItemMoveRequest, op byte) (closetSlot, bagSlot uint16) {
	switch op {
	case avatarClosetLedgerPut:
		return r.DestinationSlot, r.SourceSlot
	case avatarClosetLedgerTake:
		return r.SourceSlot, r.DestinationSlot
	}
	return r.SourceSlot, r.DestinationSlot
}

// avatarClosetLedgerKey 是幂等键。
//
// ★ 必须带**前置账本长度**，否则同一次提交的重试与"玩家重复做同一个动作"无法区分：
//   - 重试（网络重发）⇒ 账本长度没变 ⇒ key 相同 ⇒ 幂等回执，不重复记 ✓
//   - 玩家又做了一次一模一样的动作（放进 A、取出、再放进 A）⇒ 长度已变 ⇒ key 不同 ⇒ 记 ✓
// 只用常量键或只用内容摘要都会错（10-12 的旧方案用常量键，导致"只有第一次记账"）。
func avatarClosetLedgerKey(wireID int64, entries int, raw []byte) string {
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("avatar-closet-ledger:%d:%d:%x", wireID, entries, sum[:8])
}

// readAvatarClosetLedger 从角色存档里读账本（没有则为空账本）。
func readAvatarClosetLedger(state json.RawMessage) (avatarClosetLedger, error) {
	var probe struct {
		Ledger json.RawMessage `json:"avatar_closet_ledger"`
	}
	if err := json.Unmarshal(state, &probe); err != nil {
		return avatarClosetLedger{}, err
	}
	if len(probe.Ledger) == 0 {
		return avatarClosetLedger{}, nil
	}
	var ledger avatarClosetLedger
	if err := json.Unmarshal(probe.Ledger, &ledger); err != nil {
		return avatarClosetLedger{}, err
	}
	return ledger, nil
}

// writeAvatarClosetLedger 把账本写回角色存档的顶层键（不动 inventory 那一坨）。
func writeAvatarClosetLedger(state json.RawMessage, ledger avatarClosetLedger) (json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(state, &root); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(ledger)
	if err != nil {
		return nil, err
	}
	root["avatar_closet_ledger"] = raw
	out, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// appendAvatarClosetLedger 记账：把这一笔追加到账本末尾，返回新存档与"是否真的记了"。
//
// 逐项防御（10-12 的教训：宁可不动，也不能把玩家的账记乱）：
//   - 全量校验序号：每一项的 Seq 必须恰好等于"下标+1"（重号/跳号/首位非 1
//     ⇒ 账本已损坏，拒绝写）。只查末项会放过中间跳号（如 {7},{2}）；
//   - 同一笔**内容**不得重复出现在末尾（防客户端把同一个动作报了两次）。
func appendAvatarClosetLedger(ledger avatarClosetLedger, entry avatarClosetLedgerEntry) (avatarClosetLedger, bool, error) {
	for i, item := range ledger.Items {
		if item.Seq != uint64(i+1) {
			return ledger, false, fmt.Errorf("closet ledger 序号损坏: index=%d seq=%d want=%d", i, item.Seq, i+1)
		}
	}
	if n := len(ledger.Items); n > 0 {
		last := ledger.Items[n-1]
		if entry.Op == last.Op && entry.ClosetSlot == last.ClosetSlot &&
			entry.BagSlot == last.BagSlot && entry.Item == last.Item {
			// 与末笔完全相同 ⇒ 判定为重发，不作为新的一笔。
			return ledger, false, nil
		}
		entry.Seq = uint64(n + 1)
	} else {
		entry.Seq = 1
	}
	ledger.Items = append(ledger.Items, entry)
	return ledger, true, nil
}
