package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"path/filepath"
	"testing"

	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 衣柜「change」（CMD1102）—— 把选中套穿到身上、把身上那套换进柜子。
//
// 协议侧证据见 avatar_closet_change.go 头注。本文件钉住**服务端计划**的每条不变量：
// 只置换不增删、柜槽/穿戴槽配对、临时槽、noti1077 条目、账本与兜底行、失败即放弃。

// changeState 拼一份"装扮栏里还留着寄存行 + 有穿戴栏"的角色存档。
func changeState(t *testing.T, ledger avatarClosetLedger, backing avatarClosetBacking,
	expansion byte, bar []inventory.BagEquipment, worn []inventory.BagEquipment) []byte {
	t.Helper()
	base, err := json.Marshal(character.State{Level: 115,
		SourceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	require.NoError(t, err)
	state, err := inventory.SaveBag(base, inventory.Bag{
		Version:         "ordinary-bag-v1",
		ClosetExpansion: expansion,
		Special:         map[byte][]inventory.BagEquipment{protocol.AvatarContainerID: bar},
		Worn:            worn,
	})
	require.NoError(t, err)
	if len(ledger.Items) > 0 {
		state, err = writeAvatarClosetLedger(state, ledger)
		require.NoError(t, err)
	}
	if len(backing.Items) > 0 {
		state, err = writeAvatarClosetBacking(state, backing)
		require.NoError(t, err)
	}
	return state
}

// slotMap 把"模板 → 穿戴槽"查表包装成换装目录（默认都不是克隆件、分组 1）。
func slotMap(m map[uint32]uint16) avatarClosetChangeCatalog {
	return avatarClosetChangeCatalog{
		WornSlotOf: func(template uint32) (uint16, bool) {
			slot, ok := m[template]
			return slot, ok
		},
		GroupOf: func(uint32) byte { return 1 },
	}
}

// cloneCatalog 同上，但把 clones 里的模板标成克隆件（分组 0、不许进柜）。
func cloneCatalog(m map[uint32]uint16, clones ...uint32) avatarClosetChangeCatalog {
	c := slotMap(m)
	set := map[uint32]bool{}
	for _, t := range clones {
		set[t] = true
	}
	c.IsClone = func(template uint32) bool { return set[template] }
	c.GroupOf = func(template uint32) byte {
		if set[template] {
			return 0
		}
		return 1
	}
	return c
}

// ① 基本换装：柜槽 0（帽子）换到穿戴槽 0（身上空着）、柜槽 3（上衣）与身上的上衣互换。
func TestAvatarClosetChangeSwapsSetWithWorn(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 40, Item: 0x1001},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{
			{Slot: 40, Template: 0x1001}, // 寄存在柜槽 0
			{Slot: 41, Template: 0x1002}, // 寄存在柜槽 3
			{Slot: 5, Template: 0x2001},  // 真在背包里
		},
		[]inventory.BagEquipment{
			{Slot: 1, Template: 0x3002, Group: 1}, // 头发，柜里没有 ⇒ 不动
			{Slot: 3, Template: 0x3001, Group: 1}, // 上衣 ⇒ 要被换进柜子
		})

	plan, err := planAvatarClosetChange(state, 0,
		slotMap(map[uint32]uint16{0x1001: 0, 0x1002: 3, 0x3001: 3, 0x3002: 1}))
	require.NoError(t, err)
	require.Len(t, plan.Pairs, 3, "整套互换：柜里两件 + 身上一件（柜里没有那件也要收进柜）")

	// 部位 0（柜槽 0，帽子）：身上该部位空着 ⇒ 只穿上，不写 Put。
	require.Equal(t, uint16(0), plan.Pairs[0].WornSlot)
	require.Equal(t, uint16(0), plan.Pairs[0].ClosetSlot)
	require.True(t, plan.Pairs[0].HasCloset)
	require.False(t, plan.Pairs[0].HasWorn)
	// 部位 1（头发）：柜里没货 ⇒ 收进本套第一个空格（第 1 号槽），身上随之空出来。
	require.Equal(t, uint16(1), plan.Pairs[1].WornSlot)
	require.False(t, plan.Pairs[1].HasCloset)
	require.True(t, plan.Pairs[1].Deposit)
	require.Equal(t, uint16(1), plan.Pairs[1].ClosetSlot)
	// 部位 3（上衣）：柜里有、身上有 ⇒ 原地互换。
	require.Equal(t, uint16(3), plan.Pairs[2].WornSlot)
	require.Equal(t, uint16(3), plan.Pairs[2].ClosetSlot)
	require.True(t, plan.Pairs[2].HasCloset)
	require.Equal(t, uint32(0x3001), plan.Pairs[2].WornRow.Template)

	// 下发行 = 客户端侧装扮栏（寄存行全部剔除；克隆件才会被放回栏里，这里没有）。
	require.Equal(t, []inventory.BagEquipment{{Slot: 5, Template: 0x2001}}, plan.BarRows)

	// 临时槽 = 装扮栏里的空闲槽（5 被占 ⇒ 0、1、2）。
	require.Equal(t, uint16(0), plan.Pairs[0].Scratch)
	require.Equal(t, uint16(1), plan.Pairs[1].Scratch)
	require.Equal(t, uint16(2), plan.Pairs[2].Scratch)
	require.Equal(t, []protocol.AvatarClosetSetEntry{
		{Container: protocol.AvatarClosetContainerID, Slot: 0, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 0},
		{Container: protocol.AvatarClosetContainerID, Slot: 1, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 1},
		{Container: protocol.AvatarClosetContainerID, Slot: 3, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 2},
	}, plan.Entries)

	// 预备快照 = 装扮栏 + 可寄存的身上件放到临时槽（柜里没货的头发也算）。
	require.Equal(t, []inventory.BagEquipment{
		{Slot: 5, Template: 0x2001},
		{Slot: 1, Template: 0x3002, Group: 1},
		{Slot: 2, Template: 0x3001, Group: 1},
	}, plan.PrepRows)

	// 新穿戴栏：槽 0 穿上帽子、槽 3 换成柜里那件；槽 1 空出来（头发进柜了）。
	require.Len(t, plan.Worn, 2)
	bySlot := map[uint16]inventory.BagEquipment{}
	for _, row := range plan.Worn {
		bySlot[row.Slot] = row
	}
	require.Equal(t, uint32(0x1001), bySlot[0].Template, "柜槽 0 的帽子穿上")
	require.Equal(t, uint32(0x1002), bySlot[3].Template, "柜槽 3 的上衣穿上")
	require.Equal(t, byte(1), bySlot[0].Group)
	_, stillWorn := bySlot[1]
	require.False(t, stillWorn, "柜里没货的部位 ⇒ 身上那件被收进柜子")
	require.Len(t, plan.WornBase, 2)

	// 账本终态：柜槽 0 被取空；柜槽 1、3 装进身上那两件。
	folded := foldAvatarClosetLedger(plan.Ledger)
	require.Len(t, folded, 2)
	require.Equal(t, uint32(0x3002), folded[1].Item)
	require.Equal(t, uint32(0x3001), folded[3].Item)
	require.Equal(t, avatarClosetNoBarSlot, folded[3].BagSlot)

	// 兜底行：柜槽 1、3 换成身上那两件的整行；柜槽 0 的备份删掉。
	index := backingIndex(plan.Backing)
	require.Len(t, index, 2)
	require.Equal(t, uint32(0x3001), index[3].Row.Template)
	require.Equal(t, avatarClosetNoBarSlot, index[3].BagSlot)
	_, exists := index[0]
	require.False(t, exists)
}

// ② 落库往返：worn / 装扮栏 / 账本 / 兜底行四处都写对，且装扮栏里不再留寄存行。
func TestApplyAvatarClosetChangeStateRoundTrip(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}, {Slot: 5, Template: 0x2001}},
		[]inventory.BagEquipment{{Slot: 3, Template: 0x3001, Group: 1}})

	plan, err := planAvatarClosetChange(state, 0, slotMap(map[uint32]uint16{0x1002: 3, 0x3001: 3}))
	require.NoError(t, err)
	next, err := applyAvatarClosetChangeState(state, plan)
	require.NoError(t, err)

	bag, err := inventory.ReadBag(next)
	require.NoError(t, err)
	require.Equal(t, []inventory.BagEquipment{{Slot: 5, Template: 0x2001}}, bag.Special[protocol.AvatarContainerID],
		"寄存行必须离开装扮栏")
	require.Len(t, bag.Worn, 1)
	require.Equal(t, uint32(0x1002), bag.Worn[0].Template, "柜里的件穿到身上")
	require.Equal(t, uint16(3), bag.Worn[0].Slot)

	// 重登编译：柜槽 3 的新件（哨兵 BagSlot）走兜底行合成，不按模板去装扮栏乱认。
	hidden := avatarClosetHiddenBarSlots(next)
	require.Empty(t, hidden, "装扮栏里已经没有寄存行")
	live, injected, err := avatarClosetSetPlan(next)
	require.NoError(t, err)
	require.Empty(t, live)
	require.Len(t, injected, 1, "哨兵条目必须走兜底行合成")
	require.Equal(t, uint16(3), injected[0].Entry.Slot)
	require.Equal(t, uint32(0x3001), injected[0].Row.Template)
}

// ③ 哨兵 BagSlot：绝不按模板去装扮栏乱认（同模板撞车会藏起玩家真正的背包件）。
func TestAvatarClosetLinkedSlotSkipsSentinel(t *testing.T) {
	entry := avatarClosetLedgerEntry{Op: avatarClosetLedgerPut, ClosetSlot: 0,
		BagSlot: avatarClosetNoBarSlot, Item: 0x1001}
	rows := []inventory.BagEquipment{{Slot: 7, Template: 0x1001}}
	_, ok := avatarClosetLinkedSlot(rows, entry)
	require.False(t, ok, "哨兵条目不做现查")

	// 对照：正常的账本 BagSlot 仍然现查得到。
	normal := avatarClosetLedgerEntry{Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 7, Item: 0x1001}
	slot, ok := avatarClosetLinkedSlot(rows, normal)
	require.True(t, ok)
	require.Equal(t, uint16(7), slot)
}

// ④ 衣柜里没衣服也要能换：把身上那套收进柜子（业主 2026-10-14 明确要求）。
func TestAvatarClosetChangeDepositsWornWhenClosetSetEmpty(t *testing.T) {
	state := changeState(t, avatarClosetLedger{}, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 5, Template: 0x2001}},
		[]inventory.BagEquipment{{Slot: 3, Template: 0x3001, Group: 1}})
	plan, err := planAvatarClosetChange(state, 0, slotMap(map[uint32]uint16{0x3001: 3}))
	require.NoError(t, err)
	require.Len(t, plan.Pairs, 1)
	require.False(t, plan.Pairs[0].HasCloset)
	require.True(t, plan.Pairs[0].Deposit)
	require.Equal(t, uint16(3), plan.Pairs[0].ClosetSlot, "本套第 3 号槽正好空着 ⇒ 收进那里")

	// 身上空出来、装扮栏不多东西（件进了柜）。
	require.Empty(t, plan.Worn)
	require.Equal(t, []inventory.BagEquipment{{Slot: 5, Template: 0x2001}}, plan.BarRows)
	require.Equal(t, []inventory.BagEquipment{
		{Slot: 5, Template: 0x2001},
		{Slot: plan.Pairs[0].Scratch, Template: 0x3001, Group: 1},
	}, plan.PrepRows)

	folded := foldAvatarClosetLedger(plan.Ledger)
	require.Len(t, folded, 1)
	require.Equal(t, uint32(0x3001), folded[3].Item)
	require.Equal(t, avatarClosetNoBarSlot, folded[3].BagSlot)
	require.Len(t, plan.Backing.Items, 1)
}

// ④b 两侧都空 ⇒ 无事可做，拒绝（只回 ACK，存档不动）。
func TestAvatarClosetChangeRefusesWhenBothSidesEmpty(t *testing.T) {
	state := changeState(t, avatarClosetLedger{}, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 5, Template: 0x2001}}, nil)
	_, err := planAvatarClosetChange(state, 0, slotMap(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "both empty")
}

// ⑤ 套号超出已购容量 ⇒ 拒绝。
func TestAvatarClosetChangeRefusesSetOutsideCapacity(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 13, BagSlot: 41, Item: 0x1002},
	}}
	// expansion 0 ⇒ 1 套（12 槽），柜槽 13 属于第 2 套。
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}}, nil)
	_, err := planAvatarClosetChange(state, 1, slotMap(map[uint32]uint16{0x1002: 3}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "capacity")
}

// ⑥ 部位解析不出 ⇒ 退回"套内第 k 槽 ↔ 穿戴槽 k"。
func TestAvatarClosetChangeFallsBackToSetLocalIndex(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}}, nil)
	plan, err := planAvatarClosetChange(state, 0, slotMap(nil))
	require.NoError(t, err)
	require.Len(t, plan.Pairs, 1)
	require.Equal(t, uint16(1), plan.Pairs[0].WornSlot, "套内第 1 槽 → 穿戴槽 1")
}

// ⑦ 同一部位在柜里有两件 ⇒ 跳过靠后的那件（留在柜里，该部位不动）。
func TestAvatarClosetChangeSkipsDuplicateWornSlot(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 40, Item: 0x1001},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 41, Item: 0x1003},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 40, Template: 0x1001}, {Slot: 41, Template: 0x1003}}, nil)
	// 两件都是帽子（穿戴槽 0）。
	plan, err := planAvatarClosetChange(state, 0,
		slotMap(map[uint32]uint16{0x1001: 0, 0x1003: 0}))
	require.NoError(t, err)
	require.Len(t, plan.Pairs, 1)
	require.Equal(t, uint16(0), plan.Pairs[0].ClosetSlot, "只换柜槽小的那件")
	require.Len(t, plan.Entries, 1)
}

// ⑧ 柜里某格的行已经找不回来（洞位被覆盖且没备份）⇒ **只跳过那一格**，
//    其余部位照常换（不再因为一格坏数据让整次换装失效）。
func TestAvatarClosetChangeSkipsUnresolvableRow(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 40, Item: 0x1001},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	// 装扮栏里没有 0x1001（0 号柜槽的行丢了），也没有它的兜底行；3 号柜槽的件在。
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}},
		[]inventory.BagEquipment{{Slot: 3, Template: 0x3001, Group: 1}})
	plan, err := planAvatarClosetChange(state, 0,
		slotMap(map[uint32]uint16{0x1002: 3, 0x3001: 3}))
	require.NoError(t, err)
	require.Equal(t, []uint16{0}, plan.Skipped)
	require.Len(t, plan.Pairs, 1)
	require.Equal(t, uint16(3), plan.Pairs[0].ClosetSlot)
	require.Equal(t, uint32(0x1002), plan.Pairs[0].ClosetRow.Template)
	require.Equal(t, uint32(0x3001), plan.Pairs[0].WornRow.Template)
}

// ⑧b 柜里的格子坏掉、身上也没穿 ⇒ 无事可做，拒绝（不因坏数据丢件）。
func TestAvatarClosetChangeRefusesWhenOnlyBadRows(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 40, Item: 0x1001},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 5, Template: 0x2001}}, nil)
	_, err := planAvatarClosetChange(state, 0, slotMap(map[uint32]uint16{0x1001: 0}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "both empty")
}

// ⑨ 装扮栏塞满（120 行且没有寄存行可腾）⇒ 没有临时槽，拒绝（绝不做半个换装）。
func TestAvatarClosetChangeRefusesWhenBarFull(t *testing.T) {
	rows := make([]inventory.BagEquipment, 0, 120)
	for slot := 0; slot < 120; slot++ {
		rows = append(rows, inventory.BagEquipment{Slot: uint16(slot), Template: 0x0001DEAD + uint32(slot)})
	}
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 40, Item: 0x1001},
	}}
	// 0x1001 只在兜底行里（装扮栏 120 行全是别的货，一个空槽都没有）。
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 0, BagSlot: 40, Row: inventory.BagEquipment{Slot: 40, Template: 0x1001}},
	}}
	state := changeState(t, ledger, backing, 0, rows, nil)
	_, err := planAvatarClosetChange(state, 0, slotMap(map[uint32]uint16{0x1001: 0}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "no free slot")
}

// ⑩ 穿到身上的那件，分组必须取自目录（GroupOf），与 MoveOrdinary 的 itemGroup 同口径。
func TestAvatarClosetChangeUsesCatalogGroup(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}}, nil)
	catalog := slotMap(map[uint32]uint16{0x1002: 3})
	catalog.GroupOf = func(uint32) byte { return 0 }
	plan, err := planAvatarClosetChange(state, 0, catalog)
	require.NoError(t, err)
	require.Len(t, plan.Worn, 1)
	require.Equal(t, byte(0), plan.Worn[0].Group)
}

// ⑩b 身上那个部位是**克隆件** ⇒ 它不进柜，而是放回装扮栏；柜槽照样腾空、柜里那件照样穿上。
func TestAvatarClosetChangeKeepsWornCloneOutOfCloset(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}, {Slot: 5, Template: 0x2001}},
		[]inventory.BagEquipment{{Slot: 3, Template: 0x9001, Group: 0}}) // 0x9001 = 克隆上衣
	catalog := cloneCatalog(map[uint32]uint16{0x1002: 3, 0x9001: 3}, 0x9001)
	plan, err := planAvatarClosetChange(state, 0, catalog)
	require.NoError(t, err)
	require.Len(t, plan.Pairs, 1)
	require.False(t, plan.Pairs[0].Deposit, "克隆件不可寄存")
	require.True(t, plan.Pairs[0].HasWorn)

	// 柜里那件穿上身。
	require.Len(t, plan.Worn, 1)
	require.Equal(t, uint32(0x1002), plan.Worn[0].Template)
	require.Equal(t, uint16(3), plan.Worn[0].Slot)

	// 克隆件放回装扮栏的临时槽（不占别人的槽、也不进预备快照）。
	require.Len(t, plan.ClonesToBar, 1)
	require.Equal(t, uint32(0x9001), plan.ClonesToBar[0].Template)
	require.Equal(t, plan.Pairs[0].Scratch, plan.ClonesToBar[0].Slot)
	require.Len(t, plan.PrepRows, 1, "克隆件不放进预备快照的临时槽（否则 noti1077 会把它搬进柜）")
	require.Equal(t, uint32(0x2001), plan.PrepRows[0].Template)

	// 最终装扮栏 = 原装扮栏 + 克隆件。
	require.Len(t, plan.BarRows, 2)
	slots := map[uint16]uint32{}
	for _, row := range plan.BarRows {
		slots[row.Slot] = row.Template
	}
	require.Equal(t, uint32(0x2001), slots[5])
	require.Equal(t, uint32(0x9001), slots[plan.Pairs[0].Scratch])

	// 柜槽被腾空（只有 Take，没有 Put），兜底行删掉。
	folded := foldAvatarClosetLedger(plan.Ledger)
	require.Empty(t, folded)
	require.Empty(t, plan.Backing.Items)
}

// ⑩c 柜槽里若是克隆件（异常数据）⇒ 整个柜槽跳过：不穿上、也不动它。
func TestAvatarClosetChangeSkipsCloneInCloset(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 40, Item: 0x9001},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 40, Template: 0x9001}, {Slot: 41, Template: 0x1002}}, nil)
	catalog := cloneCatalog(map[uint32]uint16{0x9001: 0, 0x1002: 3}, 0x9001)
	plan, err := planAvatarClosetChange(state, 0, catalog)
	require.NoError(t, err)
	require.Len(t, plan.Pairs, 1, "克隆柜槽整格跳过")
	require.Equal(t, uint16(3), plan.Pairs[0].ClosetSlot)

	// 克隆那格在账本终态里原封不动。
	folded := foldAvatarClosetLedger(plan.Ledger)
	require.Equal(t, uint32(0x9001), folded[0].Item)
	require.Len(t, folded, 1)
}

// ⑪ 账本序号完整性：换装追加的条目必须让整本账保持 1..N 连续（否则登录折叠会拒读）。
func TestAvatarClosetChangeLedgerStaysSequential(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	state := changeState(t, ledger, avatarClosetBacking{}, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002}},
		[]inventory.BagEquipment{{Slot: 3, Template: 0x3001, Group: 1}})
	plan, err := planAvatarClosetChange(state, 0, slotMap(map[uint32]uint16{0x1002: 3, 0x3001: 3}))
	require.NoError(t, err)
	require.Len(t, plan.Ledger.Items, 3, "原 1 条 + Take + Put")
	for i, item := range plan.Ledger.Items {
		require.Equal(t, uint64(i+1), item.Seq)
	}
	// 写回后仍可读（序号校验是登录折叠的硬门禁）。
	next, err := applyAvatarClosetChangeState(state, plan)
	require.NoError(t, err)
	back, err := readAvatarClosetLedger(next)
	require.NoError(t, err)
	require.Len(t, back.Items, 3)
}

// ⑫ 换装后的穿戴行能通过 EquipmentPayload(space 3) 的全部校验（无重复槽、Record 自洽）。
func TestAvatarClosetChangeWornPayloadValid(t *testing.T) {
	record := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(record[2:], 0x1002)
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}}
	backing := avatarClosetBacking{Version: 1, Items: []avatarClosetBackingEntry{
		{ClosetSlot: 3, BagSlot: 41, Row: inventory.BagEquipment{Slot: 41, Template: 0x1002, Record: record}},
	}}
	state := changeState(t, ledger, backing, 0,
		[]inventory.BagEquipment{{Slot: 41, Template: 0x1002, Record: record}},
		[]inventory.BagEquipment{{Slot: 3, Template: 0x3001, Group: 1}})
	plan, err := planAvatarClosetChange(state, 0, slotMap(map[uint32]uint16{0x1002: 3, 0x3001: 3}))
	require.NoError(t, err)
	body, err := inventory.EquipmentPayload(3, plan.WornBase, false)
	require.NoError(t, err, "穿戴行必须通过 EquipmentPayload 校验")
	require.Equal(t, byte(3), body[0])
	require.Equal(t, uint16(len(plan.WornBase)), binary.LittleEndian.Uint16(body[1:3]))
}

// ⑬ 连续两次点 change（包体完全相同）必须**都生效**。
//
// 2026-10-14 实机现场：幂等键只带内容摘要 ⇒ 第一次生效、之后 24 次全被当重发静默丢弃
// （events.jsonl 里 `avatar_closet_change_applied` 一次、`avatar_closet_change_replayed`
// 24 次）。CMD1102 的包体只有 1 字节套号，所以键里必须带**账本条数**这个会随每次成功
// 换装变化的判别位。
func TestAvatarClosetChangeAppliesOnEveryClick(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite,
		SQLitePath: filepath.Join(t.TempDir(), "closet-change.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		require.NoError(t, migrate(ctx))
	}
	account, err := store.DevelopmentAccount(ctx, "closet-change")
	require.NoError(t, err)
	const source = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	base, err := json.Marshal(character.State{Level: 115, SourceSHA256: source})
	require.NoError(t, err)
	// 柜槽 3 寄着一件上衣；身上穿着另一件上衣。
	state, err := inventory.SaveBag(base, inventory.Bag{
		Version:         "ordinary-bag-v1",
		ClosetExpansion: 0,
		Special:         map[byte][]inventory.BagEquipment{protocol.AvatarContainerID: {{Slot: 41, Template: 0x1002}}},
		Worn:            []inventory.BagEquipment{{Slot: 3, Template: 0x3001, Group: 1}},
	})
	require.NoError(t, err)
	state, err = writeAvatarClosetLedger(state, avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 3, BagSlot: 41, Item: 0x1002},
	}})
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ClosetChange",
		Profession: 0, ConfigVersion: source, Request: []byte{0}, State: state}, 24)
	require.NoError(t, err)

	catalog := slotMap(map[uint32]uint16{0x1002: 3, 0x3001: 3})
	payload := []byte{0} // 两次点击的包体完全相同
	click := func() bool {
		key := avatarClosetChangeKey(int64(role.WireID), source,
			avatarClosetLedgerLen(role.State), 0, payload)
		saved, applied, e := store.CommitCharacterEvent(ctx, account, role.ID, source, key,
			avatarClosetChangeModel, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				p, planErr := planAvatarClosetChange(current.State, 0, catalog)
				if planErr != nil {
					return nil, nil, planErr
				}
				next, applyErr := applyAvatarClosetChangeState(current.State, p)
				if applyErr != nil {
					return nil, nil, applyErr
				}
				return next, json.RawMessage(`{"applied":true}`), nil
			})
		require.NoError(t, e)
		role = saved
		return applied
	}

	require.True(t, click(), "第一次点 change 必须生效")
	require.True(t, click(), "第二次点 change 也必须生效（幂等键要随账本变化）")

	// 换两次 = 换回来：身上又是原来那件，柜里又是原来那件。
	bag, err := inventory.ReadBag(role.State)
	require.NoError(t, err)
	require.Len(t, bag.Worn, 1)
	require.Equal(t, uint32(0x3001), bag.Worn[0].Template)
	folded := foldAvatarClosetLedger(mustReadLedger(t, role.State))
	require.Len(t, folded, 1)
	require.Equal(t, uint32(0x1002), folded[3].Item)
}

// mustReadLedger 读账本（测试用；读不出来直接失败）。
func mustReadLedger(t *testing.T, state json.RawMessage) avatarClosetLedger {
	t.Helper()
	ledger, err := readAvatarClosetLedger(state)
	require.NoError(t, err)
	return ledger
}
