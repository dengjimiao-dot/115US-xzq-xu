package main

import (
	"encoding/binary"
	"testing"

	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 装扮栏下发剔除寄存行（2026-10-14 晚，业主现场"点整理后寄存件又回到背包"根治）。
//
// 现场证据（会话 roles_..._20261014_145617_703554_next37）：
//   登录帧 118 行 ⇒ 客户端 perm 只作用在它**真正持有**的行上（104 行）；
//   服务端把 perm 作用到 118 行并整帧重推 ⇒ 多推 14 行 = perm(寄存槽) = {85,105..117}
//   ⇒ 寄存的 14 件重新出现在背包里，且客户端的衣柜链接/实例记账错位
//   （"时装栏不显示穿在身上的时装"）。
//
// 本文件钉住：无账本逐字节不变、寄存行被剔除、账本 BagSlot 过时也能跟住、
// 洞位（行被覆盖）不误剔、容量闸口、同模板撞车去重、整理后仍与客户端一致。

// ① 无账本 ⇒ 与旧路径逐字节相同，且不做任何拷贝（老存档路径零行为变化）。
func TestAvatarBarRestorePayloadMatchesLegacyWithoutLedger(t *testing.T) {
	rows := []inventory.BagEquipment{
		{Slot: 0, Template: 0x00010407},
		{Slot: 41, Template: 0x00010511},
	}
	state := closetState(t, avatarClosetLedger{}, 0, rows)

	want, err := inventory.SpecialEquipmentRestorePayload(state, 1)
	require.NoError(t, err)
	got, err := avatarBarRestorePayload(state)
	require.NoError(t, err)
	require.Equal(t, want, got, "无账本 ⇒ 逐字节等同于 SpecialEquipmentRestorePayload")

	require.Nil(t, avatarClosetHiddenBarSlots(state))
	require.True(t, &rows[0] == &avatarBarRowsWithoutCloset(rows, nil)[0],
		"剔除集为空时必须原样返回入参切片（不拷贝、不改写）")
}

// ② 有账本 ⇒ 装扮栏里那件寄存件从下发行里剔除，其余行原样保留。
func TestAvatarBarDeliveryRowsExcludesClosetRows(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407}, // 已寄存在柜槽 0
		{Slot: 50, Template: 0x00010511}, // 真的在背包里
	})

	require.Equal(t, map[uint16]bool{41: true}, avatarClosetHiddenBarSlots(state))

	bag, err := inventory.ReadBag(state)
	require.NoError(t, err)
	got := avatarBarDeliveryRows(state, bag)
	require.Len(t, got, 1)
	require.Equal(t, uint16(50), got[0].Slot)
	require.Equal(t, uint32(0x00010511), got[0].Template)

	payload, err := avatarBarRestorePayload(state)
	require.NoError(t, err)
	require.Equal(t, byte(1), payload[0], "space 1")
	require.Equal(t, uint16(1), binary.LittleEndian.Uint16(payload[3:5]),
		"行数 = 存档行 - 寄存行（头部形状不变）")
}

// ③ 账本 BagSlot 过时（整理把寄存行挪走了）⇒ 按模板现查，跟住新槽。
func TestAvatarClosetHiddenBarSlotsFollowsRowAfterResort(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	// 整理之后：寄存件被 perm 挪到 77，41 号槽换成了别的货。
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010511},
		{Slot: 77, Template: 0x00010407},
	})

	require.Equal(t, map[uint16]bool{77: true}, avatarClosetHiddenBarSlots(state),
		"账本旧 BagSlot 已失效 ⇒ 必须按模板现查")

	bag, err := inventory.ReadBag(state)
	require.NoError(t, err)
	got := avatarBarDeliveryRows(state, bag)
	require.Len(t, got, 1)
	require.Equal(t, uint16(41), got[0].Slot, "被挪走的是 41 号槽原来的那件")
}

// ④ 洞位：寄存行已被"洞里的新货"覆盖（装扮栏里再也找不到）⇒ 一条都不剔，
//    下发行原样——否则会把玩家真正的背包件藏起来。
func TestAvatarBarDeliveryRowsKeepsRowsWhenClosetRowMissing(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010511}, // 新货顶掉了旧行
	})

	require.Empty(t, avatarClosetHiddenBarSlots(state))

	bag, err := inventory.ReadBag(state)
	require.NoError(t, err)
	require.Len(t, avatarBarDeliveryRows(state, bag), 1)
}

// ⑤ 容量闸口：柜槽越界（扩容前存的大号槽 / 坏账）不参与剔除——客户端也不会把它
//    当柜内容（登录编译同样丢弃越界槽），两边一致。
func TestAvatarClosetHiddenBarSlotsHonoursCapacity(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 23, BagSlot: 41, Item: 0x00010407},
	}}
	// expansion 0 ⇒ 1 套 = 12 槽，柜槽 23 越界。
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
	})

	require.Empty(t, avatarClosetHiddenBarSlots(state))

	// 同一份账本在 expansion 1（24 槽）下就合法了。
	wide := closetState(t, ledger, 1, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
	})
	require.Equal(t, map[uint16]bool{41: true}, avatarClosetHiddenBarSlots(wide))
}

// ⑥ 同模板撞车：两个柜槽条目、装扮栏只剩一行 ⇒ 只剔那一个槽（去重与登录编译一致）。
func TestAvatarClosetHiddenBarSlotsDedupsSharedTemplate(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 2, BagSlot: 88, Item: 0x00010407},
	}}
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
		{Slot: 60, Template: 0x00010511},
	})

	require.Equal(t, map[uint16]bool{41: true}, avatarClosetHiddenBarSlots(state),
		"撞车只占一个槽，不能剔两次（更不能把 60 号槽误剔）")

	bag, err := inventory.ReadBag(state)
	require.NoError(t, err)
	got := avatarBarDeliveryRows(state, bag)
	require.Len(t, got, 1)
	require.Equal(t, uint16(60), got[0].Slot)
}

// ⑦ 复现业主现场：perm 之后服务端 Special[1] 里寄存行落在"客户端空着的槽"，
//    下发行必须只剩客户端真正持有的行。
func TestAvatarBarRestorePayloadAfterSortMatchesClientContainer(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 0, Item: 0x00010407},
	}}
	// 服务端把 perm 作用到全部行（含寄存行）：寄存行 0 → 3，可见行 3 → 0。
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 3, Template: 0x00010407}, // 寄存行
		{Slot: 0, Template: 0x00010511}, // 可见行
	})

	bag, err := inventory.ReadBag(state)
	require.NoError(t, err)
	got := avatarBarDeliveryRows(state, bag)
	require.Len(t, got, 1)
	require.Equal(t, uint16(0), got[0].Slot)
	require.Equal(t, uint32(0x00010511), got[0].Template,
		"寄存行必须留在服务端，只有客户端持有的行可以下发")
}

// ⑧ 空装扮栏 ⇒ 下发行空、载荷仍是合法的 0 行容器（调用方按 len(body)>0 判空）。
func TestAvatarBarRestorePayloadEmptyBar(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 0, Item: 0x00010407},
	}}
	state := closetState(t, ledger, 0, nil)

	payload, err := avatarBarRestorePayload(state)
	require.NoError(t, err)
	require.NotEmpty(t, payload)
	require.Equal(t, byte(1), payload[0])
	require.Equal(t, uint16(0), binary.LittleEndian.Uint16(payload[3:5]))
}
