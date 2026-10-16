package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 第一步「时装橱窗无法保存」的性质钉死（2026-10-13 从零开始修）。
//
// 上游只回 ACK、不落任何东西 ⇒ 服务端不知道玩家的柜子长什么样。
// 本文件钉住：哪些搬移该记账、记成什么样、幂等语义、以及**绝不能**破坏的东西。

// makeItemMoveBody 造一个 29 字节的 cmd19 请求体（对应 protocol.DecodeItemMove 的当前形状）。
func makeItemMoveBody(t *testing.T, srcList byte, srcSlot uint16, srcItem uint32,
	count uint32, dstList byte, dstSlot uint16) []byte {
	t.Helper()
	body := make([]byte, 29)
	body[0] = srcList
	binary.LittleEndian.PutUint16(body[1:], srcSlot)
	binary.LittleEndian.PutUint32(body[3:], srcItem)
	binary.LittleEndian.PutUint32(body[7:], count)
	body[11] = dstList
	binary.LittleEndian.PutUint16(body[12:], dstSlot)
	return body
}

func decodeMove(t *testing.T, body []byte) protocol.ItemMoveRequest {
	t.Helper()
	r, err := protocol.DecodeItemMove(body)
	require.NoError(t, err)
	return r
}

// ① 只有 1↔33 与 33↔33 才算橱窗摆设；其余搬移与本账本无关。
func TestAvatarClosetLedgerOnlyTracksClosetMoves(t *testing.T) {
	put := makeItemMoveBody(t, 1, 5, 0x00010407, 1, 33, 3)
	take := makeItemMoveBody(t, 33, 3, 0x00010407, 1, 1, 5)
	swap33 := makeItemMoveBody(t, 33, 3, 0x00010407, 1, 33, 9)
	for name, tc := range map[string]struct {
		body []byte
		want byte
	}{
		"放进橱窗":      {put, avatarClosetLedgerPut},
		"从橱窗取出":     {take, avatarClosetLedgerTake},
		"橱窗内部换位":    {swap33, avatarClosetLedgerSwap33},
		"普通搬进背包":    {makeItemMoveBody(t, 0, 1, 1, 1, 1, 2), 0},
		"从背包丢弃":     {makeItemMoveBody(t, 1, 2, 1, 1, 0, 1), 0},
		"宠物饲料":       {makeItemMoveBody(t, 7, 3, 1, 1, 7, 4), 0},
		"跨到账号金库":    {makeItemMoveBody(t, 1, 2, 1, 1, 12, 3), 0},
	} {
		op, ok := avatarClosetLedgerMoveKind(decodeMove(t, tc.body))
		if tc.want == 0 {
			require.False(t, ok, "%s 不该记账", name)
			continue
		}
		require.True(t, ok, "%s 应记账", name)
		require.Equal(t, tc.want, op, "%s 方向", name)
	}
}

// ② 槽位映射：放进＝目的是橱窗槽；取出＝源是橱窗槽；柜内换位两个都是橱窗槽。
func TestAvatarClosetLedgerSlotMapping(t *testing.T) {
	put := decodeMove(t, makeItemMoveBody(t, 1, 41, 0x00010407, 1, 33, 7))
	closet, bag := avatarClosetLedgerSlots(put, avatarClosetLedgerPut)
	require.Equal(t, uint16(7), closet, "放进：橱窗槽应取目的槽")
	require.Equal(t, uint16(41), bag, "放进：装扮栏行应取源槽")

	take := decodeMove(t, makeItemMoveBody(t, 33, 7, 0x00010407, 1, 1, 41))
	closet, bag = avatarClosetLedgerSlots(take, avatarClosetLedgerTake)
	require.Equal(t, uint16(7), closet, "取出：橱窗槽应取源槽")
	require.Equal(t, uint16(41), bag, "取出：装扮栏行应取目的槽")
}

// ③ 幂等键：同一次提交重试必须同 key；玩家重复同一个动作必须不同 key。
// ★ 用常量键会让第二次记账被吞（10-12 旧方案的坑）；只用内容摘要会把
//   "玩家又做了一次一模一样的动作"也吞掉。
func TestAvatarClosetLedgerKeyDistinguishesRetryFromRepeat(t *testing.T) {
	raw := makeItemMoveBody(t, 1, 5, 0x00010407, 1, 33, 3)
	same := makeItemMoveBody(t, 1, 5, 0x00010407, 1, 33, 3)

	require.Equal(t,
		avatarClosetLedgerKey(6, 3, raw),
		avatarClosetLedgerKey(6, 3, same),
		"同一账本长度下的重发必须同 key（幂等）")

	require.NotEqual(t,
		avatarClosetLedgerKey(6, 3, raw),
		avatarClosetLedgerKey(6, 4, raw),
		"账本长度变了必须换 key（否则玩家重复动作会被吞）")

	require.NotEqual(t,
		avatarClosetLedgerKey(6, 3, raw),
		avatarClosetLedgerKey(8, 3, raw),
		"不同角色必须换 key")

	// key 必须稳定（同样输入永远同样输出）—— 否则回放/重启后无法对上账。
	want := "avatar-closet-ledger:6:3:" + shortHash(raw)
	require.Equal(t, want, avatarClosetLedgerKey(6, 3, raw))
}

func shortHash(raw []byte) string {
	sum := sha256.Sum256(raw)
	// 与 avatarClosetLedgerKey 的实现严格一致：sha256 取前 8 字节 → 16 个十六进制字符。
	return fmt.Sprintf("%x", sum[:8])
}

// ④ 账本追加：保序、序号接续、末笔完全相同判为重发。
func TestAppendAvatarClosetLedgerKeepsOrderAndDedupsTail(t *testing.T) {
	ledger, applied, err := appendAvatarClosetLedger(avatarClosetLedger{}, avatarClosetLedgerEntry{Op: 1, ClosetSlot: 3, BagSlot: 41, Item: 0x00010407})
	require.NoError(t, err)
	require.True(t, applied)
	require.Len(t, ledger.Items, 1)
	require.Equal(t, uint64(1), ledger.Items[0].Seq)

	// 换一件再放进同一槽 ⇒ 序号 2。
	ledger, _, err = appendAvatarClosetLedger(ledger, avatarClosetLedgerEntry{Op: 1, ClosetSlot: 3, BagSlot: 42, Item: 0x00010511})
	require.NoError(t, err)
	require.Len(t, ledger.Items, 2)
	require.Equal(t, uint64(2), ledger.Items[1].Seq)

	// 与末笔完全相同 ⇒ 判重发，不改账本。
	_, applied, err = appendAvatarClosetLedger(ledger, avatarClosetLedgerEntry{Op: 1, ClosetSlot: 3, BagSlot: 42, Item: 0x00010511})
	require.NoError(t, err)
	require.False(t, applied, "与末笔完全相同必须判为重发")

	// 取出 ⇒ 序号 3（同样内容但方向不同，必须记）。
	ledger, applied, err = appendAvatarClosetLedger(ledger, avatarClosetLedgerEntry{Op: 2, ClosetSlot: 3, BagSlot: 42, Item: 0x00010511})
	require.NoError(t, err)
	require.True(t, applied)
	require.Len(t, ledger.Items, 3)
	require.Equal(t, uint64(3), ledger.Items[2].Seq)

	// 序号损坏 ⇒ 拒绝写，绝不让坏账进存档。
	broken := avatarClosetLedger{Items: []avatarClosetLedgerEntry{{Seq: 7}, {Seq: 2}}}
	_, _, err = appendAvatarClosetLedger(broken, avatarClosetLedgerEntry{Op: 1})
	require.Error(t, err, "序号不接续必须报错")
}

// ⑤ 读写往返：账本落在顶层键，**不碰 inventory**（玩家的背包一个字节都不能动）。
func TestAvatarClosetLedgerRoundTripLeavesInventoryAlone(t *testing.T) {
	source := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	state, err := json.Marshal(character.State{Level: 115, SourceSHA256: source})
	require.NoError(t, err)
	state, err = inventory.SaveBag(state, inventory.Bag{
		Version: "ordinary-bag-v1",
		Special: map[byte][]inventory.BagEquipment{1: {{Slot: 41, Template: 0x00010407}},},
	})
	require.NoError(t, err)
	before := inventoryBytes(t, state)

	ledger, _, err := appendAvatarClosetLedger(avatarClosetLedger{}, avatarClosetLedgerEntry{Op: 1, ClosetSlot: 3, BagSlot: 41, Item: 0x00010407})
	require.NoError(t, err)
	next, err := writeAvatarClosetLedger(state, ledger)
	require.NoError(t, err)

	require.Equal(t, before, inventoryBytes(t, next), "记账绝不能改动 inventory")

	got, err := readAvatarClosetLedger(next)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, avatarClosetLedgerPut, got.Items[0].Op)
	require.Equal(t, uint16(3), got.Items[0].ClosetSlot)
	require.Equal(t, uint16(41), got.Items[0].BagSlot)

	// 没有账本的存档 ⇒ 空账本（不是错误）。
	empty, err := readAvatarClosetLedger([]byte(`{"inventory":{}}`))
	require.NoError(t, err)
	require.Empty(t, empty.Items)
}

// inventoryBytes 取存档里的 inventory 原文（[]byte 与 json.RawMessage 都能收：
// 二者底层类型相同、按赋值规则互通，这里统一按 []byte 收，避免装箱后类型断言失败）。
func inventoryBytes(t *testing.T, state []byte) string {
	t.Helper()
	var probe struct {
		Inventory json.RawMessage `json:"inventory"`
	}
	require.NoError(t, json.Unmarshal(state, &probe))
	return string(probe.Inventory)
}

// ⑥ 端到端：真 SQLite + CommitCharacterEvent —— 记完重读，账必须在。
// 并验证**同 key 重发不重复记账**（网络重试）与**不同 key 记第二笔**（玩家又搬一次）。
func TestAvatarClosetLedgerCommitsThroughRealStore(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite,
		SQLitePath: filepath.Join(t.TempDir(), "closet.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		require.NoError(t, migrate(ctx))
	}
	account, err := store.DevelopmentAccount(ctx, "closet-ledger")
	require.NoError(t, err)
	const source = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	base, err := json.Marshal(character.State{Level: 115, SourceSHA256: source})
	require.NoError(t, err)
	state, err := inventory.SaveBag(base, inventory.Bag{Version: "ordinary-bag-v1"})
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ClosetLedger",
		Profession: 0, ConfigVersion: source, Request: []byte{0}, State: state}, 24)
	require.NoError(t, err)

	commit := func(body []byte, entries int) (database.Character, bool) {
		raw, _ := json.Marshal(body)
		key := avatarClosetLedgerKey(int64(role.WireID), entries, raw)
		r := decodeMove(t, body)
		op, ok := avatarClosetLedgerMoveKind(r)
		require.True(t, ok)
		closet, bag := avatarClosetLedgerSlots(r, op)
		saved, changed, err := store.CommitCharacterEvent(ctx, account, role.ID, source, key,
			avatarClosetLedgerModel, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				ledger, readErr := readAvatarClosetLedger(current.State)
				if readErr != nil {
					return nil, nil, readErr
				}
				next, applied, appErr := appendAvatarClosetLedger(ledger, avatarClosetLedgerEntry{
					Op: op, ClosetSlot: closet, BagSlot: bag, Item: r.SourceItem, WireID: role.WireID})
				if appErr != nil || !applied {
					return current.State, json.RawMessage(`{"applied":false}`), appErr
				}
				out, writeErr := writeAvatarClosetLedger(current.State, next)
				return out, json.RawMessage(`{"applied":true}`), writeErr
			})
		require.NoError(t, err)
		return saved, changed
	}

	put := makeItemMoveBody(t, 1, 41, 0x00010407, 1, 33, 3)
	take := makeItemMoveBody(t, 33, 3, 0x00010407, 1, 1, 41)

	// ① 第一次记账：记账前账本为空 ⇒ 长度记 0。
	saved, changed := commit(put, 0)
	require.True(t, changed)
	ledger, err := readAvatarClosetLedger(saved.State)
	require.NoError(t, err)
	require.Len(t, ledger.Items, 1)

	// ② 落库前的重发（**记账前**长度与第一次相同 ⇒ 同 key）⇒ 事件层幂等，不重复记账。
	_, changed = commit(put, 0)
	require.False(t, changed, "同 key 重发必须幂等")

	// ③ 落库后的重发（当前长度已变成 1 ⇒ 新 key，事件层认不出）⇒ 靠**末笔去重**兜底：
	//    内容与末笔完全相同 ⇒ 不开新笔，账本长度不变。
	saved, changed = commit(put, 1)
	require.True(t, changed, "新 key 事件本身会被记录")
	reread, err := readAvatarClosetLedger(saved.State)
	require.NoError(t, err)
	require.Len(t, reread.Items, 1, "末笔完全相同必须判重发：重发不得让账本变长")

	// ④ 玩家真的又摆了一次：先取出（33 → 1）……
	saved, changed = commit(take, len(reread.Items))
	require.True(t, changed)
	ledger, err = readAvatarClosetLedger(saved.State)
	require.NoError(t, err)
	require.Len(t, ledger.Items, 2)

	// ⑤ ……再原样放回（1 → 33）。与①内容相同的请求体，但账本长度已不同 ⇒ 必须记。
	second, changed := commit(put, len(ledger.Items))
	require.True(t, changed, "玩家重复动作必须记账")
	ledger2, err := readAvatarClosetLedger(second.State)
	require.NoError(t, err)
	require.Len(t, ledger2.Items, 3)
	// 保序：放进 1、取出 2、放进回 3。
	require.Equal(t, uint64(1), ledger2.Items[0].Seq)
	require.Equal(t, avatarClosetLedgerPut, ledger2.Items[0].Op)
	require.Equal(t, uint64(2), ledger2.Items[1].Seq)
	require.Equal(t, avatarClosetLedgerTake, ledger2.Items[1].Op)
	require.Equal(t, uint64(3), ledger2.Items[2].Seq)
	require.Equal(t, avatarClosetLedgerPut, ledger2.Items[2].Op)
	require.Equal(t, uint16(3), ledger2.Items[2].ClosetSlot, "放回同一槽")
	require.Equal(t, uint16(41), ledger2.Items[2].BagSlot)
}
