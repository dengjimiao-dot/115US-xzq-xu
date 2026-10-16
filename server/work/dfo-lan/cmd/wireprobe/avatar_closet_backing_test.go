package main

import (
	"encoding/json"
	"testing"

	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 兜底行表（"洞位"覆盖隐患根治，2026-10-14 下午）。
//
// 钉住：JSON 存取往返无损、Put/Take/Swap33 与账本折叠语义一一对应、扶贫只补
// 解析得到的存量件、Take 腾空的槽不会被扶贫补回。

// ① 存取往返：顶层键 avatar_closet_backing，缺省空表；行内容 JSON 无损。
func TestAvatarClosetBackingRoundTrip(t *testing.T) {
	base := []byte(`{"level":115}`)
	backing, err := readAvatarClosetBacking(base)
	require.NoError(t, err)
	require.Empty(t, backing.Items, "没写过 ⇒ 空表，不是错误")

	row := inventory.BagEquipment{Slot: 41, Template: 0x00010407,
		Record:        make([]byte, protocol.CurrentItemRecordSize),
		AvatarOptions: []byte{1, 2, 3}, AvatarSockets: []byte{4, 5}}
	backing = avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 3, BagSlot: 41, Row: row},
	}}
	state, err := writeAvatarClosetBacking(base, backing)
	require.NoError(t, err)
	got, err := readAvatarClosetBacking(state)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, uint16(3), got.Items[0].ClosetSlot)
	require.Equal(t, row, got.Items[0].Row, "整行（含 Record/Options/Sockets）必须无损往返")

	var probe map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(state, &probe))
	require.Contains(t, probe, "level", "顶层键不能被挤掉")
	require.Contains(t, probe, avatarClosetBackingKey)
}

// ② 搬移同步：Put 记行 / 覆盖 / Take 删行 / Swap33 互换（含单边空 = 单向搬运）。
func TestApplyAvatarClosetBackingMoveKinds(t *testing.T) {
	bag := inventory.Bag{Special: map[byte][]inventory.BagEquipment{
		protocol.AvatarContainerID: {
			{Slot: 41, Template: 0x00010407},
			{Slot: 42, Template: 0x00010511},
		},
	}}
	backing := avatarClosetBacking{Version: 1}

	// Put：记下整行，并记住它来自哪个装扮栏槽。
	next, changed := applyAvatarClosetBacking(backing, bag, avatarClosetLedgerPut, 5, 41, 0x00010407)
	require.True(t, changed)
	require.Len(t, next.Items, 1)
	require.Equal(t, uint16(5), next.Items[0].ClosetSlot)
	require.Equal(t, uint16(41), next.Items[0].BagSlot)
	require.Equal(t, uint16(41), next.Items[0].Row.Slot)
	require.Equal(t, uint32(0x00010407), next.Items[0].Row.Template)

	// 同一笔重复应用 ⇒ 无变化（客户端重发/重放进同格的幂等）。
	again, changed := applyAvatarClosetBacking(next, bag, avatarClosetLedgerPut, 5, 41, 0x00010407)
	require.False(t, changed)
	require.Equal(t, next, again)

	// 覆盖 Put：同柜槽放新货 ⇒ 备份跟着换成新行。
	next2, changed := applyAvatarClosetBacking(next, bag, avatarClosetLedgerPut, 5, 42, 0x00010511)
	require.True(t, changed)
	require.Len(t, next2.Items, 1)
	require.Equal(t, uint32(0x00010511), next2.Items[0].Row.Template)
	require.Equal(t, uint16(42), next2.Items[0].BagSlot)

	// Take：删行；再 Take 一次 ⇒ 无变化。
	after, changed := applyAvatarClosetBacking(next2, bag, avatarClosetLedgerTake, 5, 42, 0x00010511)
	require.True(t, changed)
	require.Empty(t, after.Items)
	_, changed = applyAvatarClosetBacking(after, bag, avatarClosetLedgerTake, 5, 42, 0x00010511)
	require.False(t, changed)

	// Swap33：两个有货柜槽真交换。
	a, _ := applyAvatarClosetBacking(backing, bag, avatarClosetLedgerPut, 1, 41, 0x00010407)
	b, _ := applyAvatarClosetBacking(a, bag, avatarClosetLedgerPut, 2, 42, 0x00010511)
	swapped, changed := applyAvatarClosetBacking(b, bag, avatarClosetLedgerSwap33, 1, 2, 0x00010407)
	require.True(t, changed)
	idx := backingIndex(swapped)
	require.Equal(t, uint32(0x00010511), idx[1].Row.Template, "1 号槽应换成原 2 号槽的货")
	require.Equal(t, uint32(0x00010407), idx[2].Row.Template, "2 号槽应换成原 1 号槽的货")
	require.Equal(t, uint16(1), idx[1].ClosetSlot)
	require.Equal(t, uint16(2), idx[2].ClosetSlot)

	// 单边为空的 swap = 单向搬运：2 号有货、3 号空 ⇒ 货去 3 号，2 号腾空。
	moved, changed := applyAvatarClosetBacking(b, bag, avatarClosetLedgerSwap33, 2, 3, 0x00010511)
	require.True(t, changed)
	idx = backingIndex(moved)
	require.Equal(t, uint32(0x00010511), idx[3].Row.Template)
	require.NotContains(t, idx, uint16(2), "源柜槽必须腾空")

	// 装扮栏里找不到这件（异常时序）⇒ 不建备份，下次登录仍走主路径。
	_, changed = applyAvatarClosetBacking(backing, bag, avatarClosetLedgerPut, 7, 41, 0x0001DEAD)
	require.False(t, changed)
}

// ③ 扶贫：账本终态里还能在装扮栏解析到、但兜底表缺行的柜槽补上备份；
//    行已丢的槽补不上；补过一遍再补 ⇒ 无变化。
func TestBackfillAvatarClosetBackingForLegacyItems(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 42, Item: 0x00010511},
	}}
	bag := inventory.Bag{Special: map[byte][]inventory.BagEquipment{
		protocol.AvatarContainerID: {{Slot: 41, Template: 0x00010407}}, // 42 那件的行已经没了
	}}
	backing := avatarClosetBacking{Version: 1}
	next, filled := backfillAvatarClosetBacking(backing, ledger, bag)
	require.True(t, filled)
	require.Len(t, next.Items, 1, "解析不到的槽补不上，只补现况还在的")
	require.Equal(t, uint16(0), next.Items[0].ClosetSlot)
	require.Equal(t, uint16(41), next.Items[0].Row.Slot)

	again, filled := backfillAvatarClosetBacking(next, ledger, bag)
	require.False(t, filled)
	require.Equal(t, next, again, "已有备份的槽不重复补")
}

// ④ 事务同步：Take 的柜槽必须保持腾空——账本终态（追加本条之后的）里已经没有
//    它，扶贫不许把它补回来；同时给仍占用的槽补上缺失的备份。
func TestSyncAvatarClosetBackingTakeIsNotBackfilled(t *testing.T) {
	after := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 42, Item: 0x00010511},
		{Seq: 3, Op: avatarClosetLedgerTake, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	rows := []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
		{Slot: 42, Template: 0x00010511},
	}
	state := closetState(t, after, 0, rows)
	state, err := writeAvatarClosetBacking(state, avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 0, BagSlot: 41, Row: inventory.BagEquipment{Slot: 41, Template: 0x00010407}},
	}})
	require.NoError(t, err)

	out, changed, err := syncAvatarClosetBacking(state, after, avatarClosetLedgerTake, 0, 41, 0x00010407)
	require.NoError(t, err)
	require.True(t, changed)
	got, err := readAvatarClosetBacking(out)
	require.NoError(t, err)
	require.Len(t, got.Items, 1, "Take 的槽必须保持腾空，不能被扶贫补回")
	require.Equal(t, uint16(1), got.Items[0].ClosetSlot)
	require.Equal(t, uint16(42), got.Items[0].Row.Slot, "扶贫把仍占用柜槽的现值补上")
}

// ⑤ 事务同步（Put）：没有备份的存档放第一件 ⇒ 备份写进存档；完全无变化时
//    存档字节不变（不写无谓的提交）。
func TestSyncAvatarClosetBackingPutWritesThrough(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	rows := []inventory.BagEquipment{{Slot: 41, Template: 0x00010407}}
	state := closetState(t, ledger, 0, rows)

	out, changed, err := syncAvatarClosetBacking(state, ledger, avatarClosetLedgerPut, 0, 41, 0x00010407)
	require.NoError(t, err)
	require.True(t, changed)
	got, err := readAvatarClosetBacking(out)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, uint16(0), got.Items[0].ClosetSlot)
	require.Equal(t, uint16(41), got.Items[0].Row.Slot)

	// 同一笔再来一遍（幂等重放）⇒ 无变化、字节原样。
	same, changed, err := syncAvatarClosetBacking(out, ledger, avatarClosetLedgerPut, 0, 41, 0x00010407)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, string(out), string(same), "无变化不该改动存档")
}
