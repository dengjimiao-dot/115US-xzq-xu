package main

import (
	"encoding/json"
	"testing"

	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 缺口②「重登衣柜空了」的终态下发（2026-10-14）。
//
// 账本记流水，登录下发的是**终态**；帧条目语义由反汇编钉死：
// {A=33, B=柜槽, mode=1, D=装扮栏槽}，客户端把装扮栏 D 槽的货搬进柜槽 B
// （login restore 方向：客户端衣橱刚初始化必为空）。
// 本文件钉住：折叠终态、LinkedSlot 现查、越界/缺失降级、帧字节精确。

// closetState 拼一份角色存档：顶层账本 + inventory（装扮栏 container 1 的行）。
// expansion = 已购衣柜档位（0..3 ⇒ 1..4 套，每套 12 槽，总容量 12..48）。
func closetState(t *testing.T, ledger avatarClosetLedger, expansion byte, rows []inventory.BagEquipment) []byte {
	t.Helper()
	base, err := json.Marshal(character.State{Level: 115,
		SourceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	require.NoError(t, err)
	state, err := inventory.SaveBag(base, inventory.Bag{
		Version:         "ordinary-bag-v1",
		ClosetExpansion: expansion,
		Special:         map[byte][]inventory.BagEquipment{protocol.AvatarContainerID: rows},
	})
	require.NoError(t, err)
	if len(ledger.Items) == 0 {
		return state
	}
	state, err = writeAvatarClosetLedger(state, ledger)
	require.NoError(t, err)
	return state
}

// ① 折叠：Put 覆盖同槽、Take 取空、Swap33 真交换（含单边为空的单向搬运）。
func TestFoldAvatarClosetLedgerTerminalState(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 42, Item: 0x00010511},
		{Seq: 3, Op: avatarClosetLedgerSwap33, ClosetSlot: 0, BagSlot: 1, Item: 0x00010407},
		{Seq: 4, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 43, Item: 0x00010613},
		{Seq: 5, Op: avatarClosetLedgerTake, ClosetSlot: 1, BagSlot: 42, Item: 0x00010407},
		{Seq: 6, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 44, Item: 0x00010717},
	}}
	folded := foldAvatarClosetLedger(ledger)
	require.Len(t, folded, 2)
	require.Equal(t, uint32(0x00010613), folded[0].Item, "后来的 Put 必须覆盖同槽")
	require.Equal(t, uint16(43), folded[0].BagSlot)
	require.Equal(t, uint32(0x00010717), folded[1].Item, "取出后重新放进必须生效")

	// 单边为空的 swap = 单向搬运：2 号槽有货、3 号空 ⇒ 货搬到 3 号，2 号空。
	single := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 2, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerSwap33, ClosetSlot: 2, BagSlot: 3, Item: 0x00010407},
	}}
	folded = foldAvatarClosetLedger(single)
	require.Len(t, folded, 1)
	require.Equal(t, uint32(0x00010407), folded[3].Item)
	require.NotContains(t, folded, uint16(2), "源柜槽必须腾空")
}

// ② 登录编译：空账本 ⇒ 无条目（调用方发 4 个零字节，与旧行为逐字节一致）。
func TestAvatarClosetSetEntriesEmptyLedger(t *testing.T) {
	entries, err := avatarClosetSetEntries(closetState(t, avatarClosetLedger{}, 0, nil))
	require.NoError(t, err)
	require.Empty(t, entries)
}

// ③ 编译出条目：按柜槽排序、容器恒 33、mode 恒 1、LinkedSlot 取账本记录的装扮栏行。
func TestAvatarClosetSetEntriesBasic(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 42, Item: 0x00010511},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	state := closetState(t, ledger, 1, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
		{Slot: 42, Template: 0x00010511},
	})
	entries, err := avatarClosetSetEntries(state)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, []protocol.AvatarClosetSetEntry{
		{Container: protocol.AvatarClosetContainerID, Slot: 0, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 41},
		{Container: protocol.AvatarClosetContainerID, Slot: 1, Mode: protocol.AvatarClosetSyncMode, LinkedSlot: 42},
	}, entries, "柜槽升序；LinkedSlot 必须现查装扮栏行")
}

// ④ 装扮栏整理后 D 跟随：账本记的 BagSlot 已物是人非 ⇒ 按模板现查当前槽。
func TestAvatarClosetSetEntriesFollowsRearrangedAvatarBar(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
	}}
	// 玩家把 41 的那件整理到了 43（MoveOrdinary 按槽持久化），账本没跟着改。
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 43, Template: 0x00010407},
	})
	entries, err := avatarClosetSetEntries(state)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, uint16(43), entries[0].LinkedSlot, "D 必须按模板现查，不能沿用账本旧 BagSlot")
}

// ⑤ 同模板两件并存：账本 BagSlot 命中优先，否则取最小槽（确定性兜底）。
func TestAvatarClosetLinkedSlotPrefersLedgerHint(t *testing.T) {
	rows := []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
		{Slot: 47, Template: 0x00010407},
	}
	slot, ok := avatarClosetLinkedSlot(rows, avatarClosetLedgerEntry{BagSlot: 47, Item: 0x00010407})
	require.True(t, ok)
	require.Equal(t, uint16(47), slot, "账本记的槽还在 ⇒ 优先用它（另一件同模板不能顶替）")

	slot, ok = avatarClosetLinkedSlot(rows, avatarClosetLedgerEntry{BagSlot: 99, Item: 0x00010407})
	require.True(t, ok)
	require.Equal(t, uint16(41), slot, "账本槽已失效 ⇒ 同模板取最小槽，输出确定")

	_, ok = avatarClosetLinkedSlot(rows, avatarClosetLedgerEntry{BagSlot: 47, Item: 0x0001DEAD})
	require.False(t, ok, "装扮栏没这件 ⇒ 报未找到，条目整体跳过")
}

// ⑥ 降级路径：货已不在装扮栏（分解/转移走）⇒ 该柜槽跳过，其余条目照发。
func TestAvatarClosetSetEntriesSkipsMissingItems(t *testing.T) {
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 1, BagSlot: 42, Item: 0x00010511},
	}}
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407}, // 42 那件已经没了
	})
	entries, err := avatarClosetSetEntries(state)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, uint16(0), entries[0].Slot)
}

// ⑦ 容量模型（2026-10-14 定案）：总容量 = 套数 × 每套 12 槽，不是套数本身。
//    业主实测：扩容买满（expansion=3 ⇒ 4 套 ⇒ 48 槽）后用了柜槽 0..11、23、45，
//    旧代码按 4 槽钳制把 10 件全丢了（"只保存了几件"的直接根因）。
func TestAvatarClosetSetEntriesCapacityIsSetsTimesTwelve(t *testing.T) {
	// expansion=0 ⇒ 12 槽：11 号在、12 号越界。
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 11, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 12, BagSlot: 42, Item: 0x00010511},
	}}
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
		{Slot: 42, Template: 0x00010511},
	})
	entries, err := avatarClosetSetEntries(state)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, uint16(11), entries[0].Slot, "首套占 0..11，12 号才是第 2 套首槽")

	// expansion=1 ⇒ 24 槽：23 号（业主真实用过的第 2 套末槽）必须保留。
	ledger = avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 23, BagSlot: 41, Item: 0x00010407},
	}}
	entries, err = avatarClosetSetEntries(closetState(t, ledger, 1, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
	}))
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// expansion=3 ⇒ 48 槽：45 号（业主真实用过的第 4 套）必须保留，49 号才越界。
	ledger = avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 45, BagSlot: 41, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 49, BagSlot: 42, Item: 0x00010511},
	}}
	entries, err = avatarClosetSetEntries(closetState(t, ledger, 3, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
		{Slot: 42, Template: 0x00010511},
	}))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, uint16(45), entries[0].Slot)
}

// ⑧ LinkedSlot 撞车去重：两条柜槽条目解析到同一个装扮栏槽时，客户端两记搬运会
//    把错误记录吸进第二个柜槽（第一记已把 1.D 换成柜槽旧内容）。柜槽小者优先，
//    撞车条目跳过（显示空格，不串货）。
func TestAvatarClosetSetEntriesDedupsLinkedSlot(t *testing.T) {
	// 两件同模板进了 0、2 号柜槽，装扮栏只剩一行 41（两件的账本 BagSlot 都失效）。
	ledger := avatarClosetLedger{Items: []avatarClosetLedgerEntry{
		{Seq: 1, Op: avatarClosetLedgerPut, ClosetSlot: 0, BagSlot: 77, Item: 0x00010407},
		{Seq: 2, Op: avatarClosetLedgerPut, ClosetSlot: 2, BagSlot: 88, Item: 0x00010407},
	}}
	state := closetState(t, ledger, 0, []inventory.BagEquipment{
		{Slot: 41, Template: 0x00010407},
	})
	entries, err := avatarClosetSetEntries(state)
	require.NoError(t, err)
	require.Len(t, entries, 1, "撞车的第二条必须跳过")
	require.Equal(t, uint16(0), entries[0].Slot)
	require.Equal(t, uint16(41), entries[0].LinkedSlot)
}

// ⑨ 帧字节精确：u32le(n) + n×[u8][u16le][u8][u16le]，每条 6B 定长；空列表 4 个零字节。
func TestAvatarClosetSetWithWireBytes(t *testing.T) {
	require.Equal(t, []byte{0, 0, 0, 0}, protocol.AvatarClosetSetWith(nil))
	require.Equal(t, []byte{0, 0, 0, 0}, protocol.AvatarClosetSet())

	body := protocol.AvatarClosetSetWith([]protocol.AvatarClosetSetEntry{
		{Container: 33, Slot: 1, Mode: 1, LinkedSlot: 0x0201},
		{Container: 33, Slot: 2, Mode: 1, LinkedSlot: 0x0102},
	})
	require.Equal(t, []byte{
		0x02, 0x00, 0x00, 0x00, // n=2
		33, 0x01, 0x00, 1, 0x01, 0x02, // 柜槽1 → 装扮栏0x0201
		33, 0x02, 0x00, 1, 0x02, 0x01, // 柜槽2 → 装扮栏0x0102
	}, body)
}
