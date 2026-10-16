package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"

	"github.com/stretchr/testify/require"
)

// 衣柜选中套号持久化（2026-10-14 缺口①）的性质钉死。
//
// 上游默认实现只 ACK 不存 ⇒ 重登选中栏必跳回第一套。这里钉住：
// ① NOTI1076 第二个 u32 真的带上选中号（官抓该位=0 时与官方逐字节一致）；
// ② 存取只动顶层键、不碰 inventory；③ 幂等键分得清"重发"与"绕一圈"；
// ④ 真 SQLite 端到端：落库、同 key 重发幂等、绕一圈每次都能落。

// ① NOTI1076：默认（选中第一套）必须与官方 16B 逐字节一致；选中号只动第 2 个 u32。
func TestAvatarClosetInfoCarriesSelected(t *testing.T) {
	// 官方抓包定值：slots=1、selected=0。
	official, err := protocol.AvatarClosetInfo(1, 0)
	require.NoError(t, err)
	require.Equal(t, []byte{1, 0, 0, 0, 0, 0, 0, 0, 0xae, 0x6b, 0xcc, 0x64, 0x3c, 0, 0, 0}, official,
		"selected=0 必须与官抓帧逐字节一致")

	// 4 套衣柜选中第 3 套：第 2 个 u32 = 2（小端），其余字节不动。
	body, err := protocol.AvatarClosetInfo(4, 2)
	require.NoError(t, err)
	require.Equal(t, byte(4), body[0], "第 1 个 u32 = 套数（总槽数 = 套数 × 12）")
	require.Equal(t, []byte{2, 0, 0, 0}, body[4:8], "第 2 个 u32 = 选中套号（ReadU32 小端）")
	require.Equal(t, []byte{0xae, 0x6b, 0xcc, 0x64, 0x3c, 0, 0, 0}, body[8:], "尾部定值不动")

	// 越界必须拒绝，绝不放出一个客户端读不懂的帧。
	_, err = protocol.AvatarClosetInfo(0, 0)
	require.Error(t, err, "套数 0 必须拒绝")
	_, err = protocol.AvatarClosetInfo(5, 0)
	require.Error(t, err, "套数超过上限必须拒绝")
	_, err = protocol.AvatarClosetInfo(2, 2)
	require.Error(t, err, "选中号 >= 套数必须拒绝")
	_, err = protocol.AvatarClosetInfo(1, 3)
	require.Error(t, err, "选中号越界必须拒绝")
}

// ② 存取往返：只动顶层键 avatar_closet_selected(_seq)，**inventory 一字节不碰**。
func TestAvatarClosetSelectedRoundTripLeavesInventoryAlone(t *testing.T) {
	state, err := json.Marshal(character.State{Level: 115, SourceSHA256: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"})
	require.NoError(t, err)
	state, err = inventory.SaveBag(state, inventory.Bag{
		Version: "ordinary-bag-v1",
		Special: map[byte][]inventory.BagEquipment{1: {{Slot: 41, Template: 0x00010407}}},
	})
	require.NoError(t, err)
	before := inventoryBytes(t, state)

	// 没有键 ⇒ (0, 0)：第一套、序号从零开始。
	selected, seq, err := readAvatarClosetSelected(state)
	require.NoError(t, err)
	require.Equal(t, byte(0), selected)
	require.Equal(t, uint64(0), seq)

	next, err := writeAvatarClosetSelected(state, 3, 1)
	require.NoError(t, err)
	require.Equal(t, before, inventoryBytes(t, next), "存选中号绝不能改动 inventory")

	selected, seq, err = readAvatarClosetSelected(next)
	require.NoError(t, err)
	require.Equal(t, byte(3), selected)
	require.Equal(t, uint64(1), seq)

	// 再写一次别的值 ⇒ 覆盖（它是"当前值"，不是流水）。
	next, err = writeAvatarClosetSelected(next, 1, 2)
	require.NoError(t, err)
	selected, seq, err = readAvatarClosetSelected(next)
	require.NoError(t, err)
	require.Equal(t, byte(1), selected)
	require.Equal(t, uint64(2), seq)

	// 坏存档 ⇒ 报错拒绝，绝不静默吞掉。
	_, _, err = readAvatarClosetSelected([]byte(`{"avatar_closet_selected":"x"}`))
	require.Error(t, err, "类型不对的存档必须报错")
}

// ③ 幂等键：序号是唯一单调分量。
// 同序号同 key（重试幂等）；序号变必须换 key（绕一圈 1→0→1 的第二"回 1"
// 请求体和播放前现值都与第一次相同，只有序号能区分）；不同角色必须换 key。
func TestAvatarClosetSelectedKeyDistinguishesRetryFromRepeat(t *testing.T) {
	require.Equal(t,
		avatarClosetSelectedKey(6, 0),
		avatarClosetSelectedKey(6, 0),
		"同 wireID 同序号必须同 key（重试幂等）")

	require.NotEqual(t,
		avatarClosetSelectedKey(6, 0),
		avatarClosetSelectedKey(6, 1),
		"序号不同必须换 key（否则绕一圈会被吞）")

	require.NotEqual(t,
		avatarClosetSelectedKey(6, 0),
		avatarClosetSelectedKey(8, 0),
		"不同角色必须换 key（防止跨角色串）")
}

// ④ 真 SQLite 端到端：落库、同 key 重发幂等、绕一圈每次都能落。
func TestPersistAvatarClosetSelectedThroughRealStore(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite,
		SQLitePath: filepath.Join(t.TempDir(), "closet-selected.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		require.NoError(t, migrate(ctx))
	}
	account, err := store.DevelopmentAccount(ctx, "closet-selected")
	require.NoError(t, err)
	const source = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	base, err := json.Marshal(character.State{Level: 115, SourceSHA256: source})
	require.NoError(t, err)
	state, err := inventory.SaveBag(base, inventory.Bag{Version: "ordinary-bag-v1"})
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ClosetSelected",
		Profession: 0, ConfigVersion: source, Request: []byte{0}, State: state}, 25)
	require.NoError(t, err)

	// 复刻 persistAvatarClosetSelected 的事务体（与线上同一读法）。
	apply := func(seq uint64, to byte) (database.Character, bool) {
		key := avatarClosetSelectedKey(int64(role.WireID), seq)
		saved, changed, cErr := store.CommitCharacterEvent(ctx, account, role.ID, source, key,
			avatarClosetSelectedModel,
			func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				kept, currentSeq, readErr := readAvatarClosetSelected(current.State)
				if readErr != nil {
					return nil, nil, readErr
				}
				value := kept
				if kept != to {
					value = to
				}
				out, writeErr := writeAvatarClosetSelected(current.State, value, currentSeq+1)
				if writeErr != nil {
					return nil, nil, writeErr
				}
				receipt, _ := json.Marshal(map[string]any{"applied": kept != to, "selected": value})
				return out, receipt, nil
			})
		require.NoError(t, cErr)
		return saved, changed
	}

	// 选第 2 套（序号 0）。
	saved, changed := apply(0, 1)
	require.True(t, changed)
	selected, seq, err := readAvatarClosetSelected(saved.State)
	require.NoError(t, err)
	require.Equal(t, byte(1), selected)
	require.Equal(t, uint64(1), seq)

	// 同 key 重发（序号还是 0）⇒ 事件层幂等，不重复结算。
	_, changed = apply(0, 1)
	require.False(t, changed, "同 key 重发必须幂等")

	// 落库后的重发（序号已变成 1，事件层认不出）⇒ apply 内 kept==to 兜底，值不变。
	saved, changed = apply(1, 1)
	require.True(t, changed, "新 key 事件本身会记录")
	reread, seq, err := readAvatarClosetSelected(saved.State)
	require.NoError(t, err)
	require.Equal(t, byte(1), reread, "与现值相同不得改存档")

	// 玩家绕一圈：1 → 0 → 1。两次的请求体与播放前现值都相同，只有序号不同
	// ⇒ 每次都是新 key，必须都真的落（否则玩家最后的选择丢失）。
	saved, changed = apply(seq, 0)
	require.True(t, changed)
	selected, seq, err = readAvatarClosetSelected(saved.State)
	require.NoError(t, err)
	require.Equal(t, byte(0), selected)
	back, changed := apply(seq, 1)
	require.True(t, changed, "绕一圈后的同一目标必须重新落库")
	final, finalSeq, err := readAvatarClosetSelected(back.State)
	require.NoError(t, err)
	require.Equal(t, byte(1), final)
	require.Equal(t, seq+1, finalSeq, "每次真实落库序号 +1")
}
