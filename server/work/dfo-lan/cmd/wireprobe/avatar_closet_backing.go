package main

import (
	"encoding/json"

	"dfolan/internal/inventory"
)

// 寄存衣柜的**兜底行存储**（2026-10-14 下午，"洞位"覆盖隐患的根治）。
//
// 隐患机理：货从装扮栏存进柜子后，客户端把 1.D 腾空，但服务端 Special[1] 的
// backing row **仍在 D**（cmd19 只记账、不动背包）—— 登录时 noti1077 的
// {33,B,1,D} 正是把这行搬进柜槽 B 完成恢复，所以它暂时有用。
// 但玩家此后往这个"看起来空了"的 D 槽拖新货，服务端 MoveOrdinary 按槽匹配
// 会**覆盖**被存物品的行 ⇒ 下次登录该柜槽解析不出模板 ⇒ 件丢（串号/丢失同类事故）。
//
// 修法：Put 时把整行（含 AvatarOptions/AvatarsSockets/Record 等全部字段，
// BagEquipment 的 JSON 往返无损）复制一份按柜槽存档；登录编译时若装扮栏里
// 已找不到这件（行被覆盖），就用备份行**合成一个空闲槽行**随 NOTI13 装扮栏
// 快照下发，客户端再由 noti1077 把它搬进柜槽 —— 全程只走已验证安全的
// 「NOTI13 space 1 + noti1077」路径，不给客户端发任何容器 33 的帧。
//
// 与账本的关系：同一个 CommitCharacterEvent 事务内一起写（幂等键/模型沿用
// 账本的），失败只记事件绝不影响 ACK。备份**只增不改语义**：条目解析永远
// 优先信装扮栏现况，备份只在现况缺失时兜底。
const avatarClosetBackingKey = "avatar_closet_backing"

// avatarClosetBackingEntry 是一个柜槽的兜底行。
type avatarClosetBackingEntry struct {
	ClosetSlot uint16 `json:"closet_slot"`
	BagSlot    uint16 `json:"bag_slot"` // 当初放进柜子时所在的装扮栏槽（溯源/展示用）
	Row        inventory.BagEquipment `json:"row"`
}

// avatarClosetBacking 是兜底行表。保序（与账本一致的操作顺序观感）。
type avatarClosetBacking struct {
	Version int                        `json:"version"`
	Items   []avatarClosetBackingEntry `json:"items"`
}

// readAvatarClosetBacking 从角色存档读兜底行表（没有则为空表）。
func readAvatarClosetBacking(state json.RawMessage) (avatarClosetBacking, error) {
	var probe struct {
		Backing json.RawMessage `json:"avatar_closet_backing"`
	}
	if err := json.Unmarshal(state, &probe); err != nil {
		return avatarClosetBacking{}, err
	}
	if len(probe.Backing) == 0 {
		return avatarClosetBacking{Version: 1}, nil
	}
	var backing avatarClosetBacking
	if err := json.Unmarshal(probe.Backing, &backing); err != nil {
		return avatarClosetBacking{}, err
	}
	return backing, nil
}

// writeAvatarClosetBacking 把兜底行表写回角色存档顶层键（不动 inventory）。
func writeAvatarClosetBacking(state json.RawMessage, backing avatarClosetBacking) (json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(state, &root); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(backing)
	if err != nil {
		return nil, err
	}
	root[avatarClosetBackingKey] = raw
	return json.Marshal(root)
}

// backingIndex 按柜槽索引兜底行（同槽只应有一条；坏数据取首条并容忍）。
func backingIndex(backing avatarClosetBacking) map[uint16]avatarClosetBackingEntry {
	index := make(map[uint16]avatarClosetBackingEntry, len(backing.Items))
	for _, item := range backing.Items {
		if _, dup := index[item.ClosetSlot]; !dup {
			index[item.ClosetSlot] = item
		}
	}
	return index
}

// syncAvatarClosetBacking 在**记账的同一事务**里同步兜底行表：
//   1. 应用本次搬移（Put 记行 / Take 删行 / Swap33 互换，语义与账本折叠对应）；
//   2. 扶贫补齐：账本终态里还能在装扮栏解析到、但兜底表缺行的柜槽补上备份
//      （覆盖"功能上线前摆的件"——它们的行还在装扮栏，只是没备份）。
//
// ledger 必须传**追加本条之后**的账本：Take 删掉的柜槽不能被扶贫又补回来。
// 失败只返错（调用方记事件、不阻断记账）：账本是资产，备份是兜底。
func syncAvatarClosetBacking(state json.RawMessage, ledger avatarClosetLedger,
	op byte, closetSlot, bagSlot uint16, item uint32) (json.RawMessage, bool, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return state, false, err
	}
	backing, err := readAvatarClosetBacking(state)
	if err != nil {
		return state, false, err
	}
	next, changed := applyAvatarClosetBacking(backing, bag, op, closetSlot, bagSlot, item)
	next, filled := backfillAvatarClosetBacking(next, ledger, bag)
	if !changed && !filled {
		return state, false, nil
	}
	out, err := writeAvatarClosetBacking(state, next)
	if err != nil {
		return state, false, err
	}
	return out, changed || filled, nil
}

// barRowForItem 在装扮栏里按模板找行：账本槽命中优先，否则同模板最小槽。
func barRowForItem(rows []inventory.BagEquipment, template uint32, hint uint16) (inventory.BagEquipment, bool) {
	fallback := -1
	for i, row := range rows {
		if row.Template != template {
			continue
		}
		if row.Slot == hint {
			return row, true
		}
		if fallback < 0 || row.Slot < rows[fallback].Slot {
			fallback = i
		}
	}
	if fallback >= 0 {
		return rows[fallback], true
	}
	return inventory.BagEquipment{}, false
}

// applyAvatarClosetBacking 把一次搬移同步到兜底表，语义与账本折叠一一对应：
//   - Put（1→33）：该柜槽记下这件（行取自装扮栏现况；找不到行就不记，
//     下次登录仍走"装扮栏现查"主路径，没有备份也不串货）；
//   - Take（33→1）：该柜槽的兜底行删除；
//   - Swap33（33→33）：两个柜槽的兜底行互换（单边为空即单向搬运）。
func applyAvatarClosetBacking(backing avatarClosetBacking, bag inventory.Bag,
	op byte, closetSlot, bagSlot uint16, item uint32) (avatarClosetBacking, bool) {
	index := backingIndex(backing)
	rows := bag.Special[1]
	changed := false
	switch op {
	case avatarClosetLedgerPut:
		row, ok := barRowForItem(rows, item, bagSlot)
		if !ok {
			return backing, false // 行不在装扮栏（异常时序）：不建备份
		}
		row.Slot = bagSlot // 备份行记住它来自哪个装扮栏槽
		if prev, exists := index[closetSlot]; !exists || prev.Row.Template != row.Template ||
			prev.Row.Slot != row.Slot || string(mustRowJSON(prev.Row)) != string(mustRowJSON(row)) {
			index[closetSlot] = avatarClosetBackingEntry{ClosetSlot: closetSlot, BagSlot: bagSlot, Row: row}
			changed = true
		}
	case avatarClosetLedgerTake:
		if _, exists := index[closetSlot]; exists {
			delete(index, closetSlot)
			changed = true
		}
	case avatarClosetLedgerSwap33:
		from, hasFrom := index[closetSlot]
		to, hasTo := index[bagSlot]
		if hasFrom != hasTo || (hasFrom && (from.Row.Template != to.Row.Template || from.BagSlot != to.BagSlot)) {
			if hasFrom {
				from.ClosetSlot = bagSlot
				index[bagSlot] = from
			} else {
				delete(index, bagSlot)
			}
			if hasTo {
				to.ClosetSlot = closetSlot
				index[closetSlot] = to
			} else {
				delete(index, closetSlot)
			}
			changed = true
		}
	}
	if !changed {
		return backing, false
	}
	return rebuildAvatarClosetBacking(index), true
}

// mustRowJSON 兜底行比对用的序列化（BagEquipment 自定义 JSON 无损，失败按不等处理）。
func mustRowJSON(row inventory.BagEquipment) []byte {
	raw, err := json.Marshal(row)
	if err != nil {
		return nil
	}
	return raw
}

// rebuildAvatarClosetBacking 把按槽索引的兜底行转回保序切片（柜槽升序，输出确定）。
func rebuildAvatarClosetBacking(index map[uint16]avatarClosetBackingEntry) avatarClosetBacking {
	items := make([]avatarClosetBackingEntry, 0, len(index))
	for _, entry := range index {
		items = append(items, entry)
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].ClosetSlot < items[j-1].ClosetSlot; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	return avatarClosetBacking{Version: 1, Items: items}
}

// backfillAvatarClosetBacking 扶贫兜底：账本终态里还能在装扮栏解析到的柜槽，
// 若兜底表缺行就补上。覆盖"功能上线前摆的件"（它们的行还在装扮栏，但没备份）。
func backfillAvatarClosetBacking(backing avatarClosetBacking, ledger avatarClosetLedger,
	bag inventory.Bag) (avatarClosetBacking, bool) {
	index := backingIndex(backing)
	rows := bag.Special[1]
	changed := false
	for _, slot := range sortedFoldSlots(foldAvatarClosetLedger(ledger)) {
		if _, exists := index[slot]; exists {
			continue
		}
		entry := foldAvatarClosetLedger(ledger)[slot]
		row, ok := barRowForItem(rows, entry.Item, entry.BagSlot)
		if !ok {
			continue
		}
		row.Slot = entry.BagSlot
		index[slot] = avatarClosetBackingEntry{ClosetSlot: slot, BagSlot: entry.BagSlot, Row: row}
		changed = true
	}
	if !changed {
		return backing, false
	}
	return rebuildAvatarClosetBacking(index), true
}

// sortedFoldSlots 返回折叠终态的柜槽升序列表。
func sortedFoldSlots(folded map[uint16]avatarClosetLedgerEntry) []uint16 {
	slots := make([]uint16, 0, len(folded))
	for slot := range folded {
		slots = append(slots, slot)
	}
	for i := 1; i < len(slots); i++ {
		for j := i; j > 0 && slots[j] < slots[j-1]; j-- {
			slots[j], slots[j-1] = slots[j-1], slots[j]
		}
	}
	return slots
}
