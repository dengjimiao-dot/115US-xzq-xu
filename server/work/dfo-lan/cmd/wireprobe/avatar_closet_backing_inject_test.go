package main

import (
	"encoding/binary"
	"testing"

	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 兜底行注入（"洞位"覆盖隐患根治，2026-10-14 下午）。
//
// 场景：货存进柜后客户端腾空装扮栏槽，但服务端 backing row 仍在；玩家往这个
// "看起来空了"的槽拖新货 ⇒ 行被覆盖 ⇒ 下次登录该柜槽在装扮栏里解析不出模板。
// 对策：Put 时存的备份行在登录时合成一个空闲装扮栏槽，条目引用的就是合成槽，
// 行由调用方并进 NOTI13 装扮栏快照，客户端再由 noti1077 搬进柜槽。

// closetStateWithBacking 在 closetState 之上再写一份兜底行表。
func closetStateWithBacking(t *testing.T, ledger avatarClosetLedger, backing avatarClosetBacking,
	expansion byte, rows []inventory.BagEquipment) []byte {
	t.Helper()
	state := closetState(t, ledger, expansion, rows)
	if len(backing.Items) == 0 {
		return state
	}
	out, err := writeAvatarClosetBacking(state, backing)
	require.NoError(t, err)
	return out
}

// ① 洞位被覆盖：柜槽 0 的件在装扮栏里已找不到（41 号槽被新货占了）⇒ 用备份
//    行合成最小空闲槽；行就是当初那一整行（Record/Options 原样）。
func TestAvatarClosetSetPlanInjectsBackingRowForHole(t *testing.T) {
	record := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(record[2:], 0x00010407)
	stored := inventory.BagEquipment{Slot: 41, Template: 0x00010407, Record: record,
		AvatarOptions: []byte{9, 9}, AvatarSockets: []byte{7}}
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 0, BagSlot: 41, Row: stored},
	}}
	// 玩家往"腾空的洞"（41 号槽）拖了新货 ⇒ 旧行被 MoveOrdinary 覆盖。
	state := closetStateWithBacking(t, ledger, backing, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010511},
	})

	live, injected, err := avatarClosetSetPlan(state)
	require.NoError(t, err)
	require.Empty(t, live, "现查不到 ⇒ 一条 live 都不该有")
	require.Len(t, injected, 1)
	require.Equal(t, uint16(0), injected[0].Entry.Slot)
	require.Equal(t, protocol.AvatarClosetContainerID, injected[0].Entry.Container)
	require.Equal(t, protocol.AvatarClosetSyncMode, injected[0].Entry.Mode)
	require.Equal(t, uint16(0), injected[0].Entry.LinkedSlot, "41 被占 ⇒ 合成最小空闲槽 0")
	require.Equal(t, stored.Template, injected[0].Row.Template)
	require.Equal(t, record, injected[0].Row.Record, "实例字节必须原样带回")
	require.Equal(t, []byte{9, 9}, injected[0].Row.AvatarOptions)
	require.Equal(t, uint16(0), injected[0].Row.Slot, "行的槽必须改成合成槽")
}

// ② live + injected 合并：wrapper 按柜槽升序输出，引用合成槽的条目排在中间。
func TestAvatarClosetSetEntriesMergesLiveAndInjected(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 42, Item: 0x00010511},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 3, BagSlot: 42, Row: inventory.BagEquipment{Slot: 42, Template: 0x00010511}},
	}}
	// 0 号柜槽的件还在装扮栏 41；3 号柜槽的件行被覆盖（41 的新货是它）。
	state := closetStateWithBacking(t, ledger, backing, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
	})

	entries, err := avatarClosetSetEntries(state)
	require.NoError(t, err)
	require.Equal(t, []protocol.AvatarClosetSetEntry{
		{Container: protocol.AvatarClosetContainerID, Slot: 0, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 41},
		{Container: protocol.AvatarClosetContainerID, Slot: 3, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 0},
	}, entries, "柜槽升序；注入条目引用合成槽 0（41 被 live 占了）")
}

// ③ 装扮栏合满（120+8×tier 全占）⇒ 合成不了，注入条目整体跳过（显示空格，
//    不串货），live 不受影响。
func TestAvatarClosetSetPlanSkipsInjectionWhenBarFull(t *testing.T) {
	rows := make([]inventory.BagEquipment, 0, 120)
	for slot := 0; slot < 120; slot++ {
		rows = append(rows, inventory.BagEquipment{Slot: uint16(slot), Template: 0x0001DEAD + uint32(slot)})
	}
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 0, BagSlot: 41, Row: inventory.BagEquipment{Slot: 41, Template: 0x00010407}},
	}}
	state := closetStateWithBacking(t, ledger, backing, 0, rows)

	live, injected, err := avatarClosetSetPlan(state)
	require.NoError(t, err)
	require.Empty(t, live)
	require.Empty(t, injected, "装扮栏没有空闲槽 ⇒ 合不了，只能跳过")
}

// ④ 撞车去重遇到备份：两条同模板柜槽条目、装扮栏只剩一行 ⇒ 柜槽小者走 live，
//    另一条用备份合成（而不是像从前那样被跳过）。
func TestAvatarClosetSetPlanInjectionResolvesDedupCollision(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 77, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 2, BagSlot: 88, Item: 0x00010407},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 0, BagSlot: 77, Row: inventory.BagEquipment{Slot: 77, Template: 0x00010407}},
		{ClosetSlot: 2, BagSlot: 88, Row: inventory.BagEquipment{Slot: 88, Template: 0x00010407}},
	}}
	state := closetStateWithBacking(t, ledger, backing, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
	})

	live, injected, err := avatarClosetSetPlan(state)
	require.NoError(t, err)
	require.Len(t, live, 1)
	require.Equal(t, uint16(0), live[0].Slot)
	require.Equal(t, uint16(41), live[0].LinkedSlot)
	require.Len(t, injected, 1, "撞车的第二条有备份 ⇒ 合成，不再被跳过")
	require.Equal(t, uint16(2), injected[0].Entry.Slot)
	require.Equal(t, uint16(0), injected[0].Entry.LinkedSlot, "41 被 live 占了 ⇒ 合成空闲槽 0")
	require.Equal(t, uint32(0x00010407), injected[0].Row.Template)
}

// ⑤ 坏数据护栏：备份行模板与账本终态对不上 ⇒ 不注入（宁可空格，不串货）。
func TestAvatarClosetSetPlanRejectsMismatchedBacking(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 0, BagSlot: 41, Row: inventory.BagEquipment{Slot: 41, Template: 0x00099999}},
	}}
	state := closetStateWithBacking(t, ledger, backing, 0, nil)

	live, injected, err := avatarClosetSetPlan(state)
	require.NoError(t, err)
	require.Empty(t, live)
	require.Empty(t, injected, "备份与账本对不上 ⇒ 不注入")
}

// ⑥ 注入行能并进 NOTI13 装扮栏快照：合并后的行集合通过 EquipmentPayload 的
//    全部校验（无重复槽、Record 模板自洽），行数含注入行。
func TestInjectedRowsMergeIntoAvatarReadyPayload(t *testing.T) {
	record := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(record[2:], 0x00010407)
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 42, Item: 0x00010511},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 1, BagSlot: 42, Row: inventory.BagEquipment{Slot: 42, Template: 0x00010511}},
	}}
	// 0 号柜槽的件还在（41），1 号柜槽的件行被 41 号槽的新货覆盖。
	state := closetStateWithBacking(t, ledger, backing, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407, Record: func() []byte {
			r := make([]byte, protocol.CurrentItemRecordSize)
			binary.LittleEndian.PutUint32(r[2:], 0x00010407)
			return r
		}()},
	})

	live, injected, err := avatarClosetSetPlan(state)
	require.NoError(t, err)
	require.Len(t, live, 1)
	require.Len(t, injected, 1)

	bag, err := inventory.ReadBag(state)
	require.NoError(t, err)
	rows := make([]inventory.BagEquipment, 0, len(bag.Special[protocol.AvatarContainerID])+len(injected))
	rows = append(rows, bag.Special[protocol.AvatarContainerID]...)
	for _, item := range injected {
		rows = append(rows, item.Row)
	}
	payload, err := inventory.EquipmentPayload(1, rows, true, bag.AvatarExpansion)
	require.NoError(t, err, "合并行必须通过 EquipmentPayload 的全部校验")
	require.Equal(t, byte(1), payload[0], "space 1")
	require.Equal(t, uint16(len(rows)), binary.LittleEndian.Uint16(payload[3:5]), "行数 = 存档行 + 注入行")
}
