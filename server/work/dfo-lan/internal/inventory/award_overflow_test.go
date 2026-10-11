package inventory

import (
	"encoding/binary"
	"encoding/json"
	"testing"

	"dfolan/internal/catalog"
	"github.com/stretchr/testify/require"
)

func yuanmaAwarder(t *testing.T) *Awarder {
	a := testGrantAwarder(t)
	a.Rules.EquipmentSlots = [2]uint16{9, 10}
	a.Rules.Slots = map[string][2]uint16{"[material]": {121, 121}}
	a.Rules.QuickSlots = [2]uint16{0, 8}
	a.Catalog.Items[900] = catalog.LootItem{ID: 900, Kind: "stackable", StackableType: "[material]", StackLimit: 10}
	return a
}

func TestYuanmaOverflowStackConservationAndPeriod(t *testing.T) {
	for _, initial := range []uint32{0, 4, 10} {
		t.Run(string(rune('a'+initial)), func(t *testing.T) {
			a := yuanmaAwarder(t)
			bag := Bag{Version: "ordinary-bag-v1"}
			if initial > 0 {
				bag.Items = []BagItem{{Slot: 121, Template: 900, Amount: initial, ExpireTime: GrantExpireTime}}
			}
			raw, err := SaveBag(json.RawMessage(`{"future_character":{"keep":true}}`), bag)
			require.NoError(t, err)
			before := append([]byte(nil), raw...)
			out, _, mail, err := a.GrantOrMail(raw, 900, 8)
			require.NoError(t, err)
			require.Equal(t, before, []byte(raw))
			next, err := ReadBag(out)
			require.NoError(t, err)
			var landed, mailed uint32
			for _, item := range next.Items {
				landed += item.Amount
				require.EqualValues(t, GrantExpireTime, item.ExpireTime)
			}
			for _, item := range mail {
				mailed += item.Stack.Amount
				row, err := item.Row()
				require.NoError(t, err)
				require.EqualValues(t, GrantExpireTime, binary.LittleEndian.Uint32(row[56:60]))
			}
			require.Equal(t, initial+8, landed+mailed)
			require.Contains(t, string(out), `"future_character"`)
		})
	}
}

func TestYuanmaOverflowSkipsExpiredStack(t *testing.T) {
	a := yuanmaAwarder(t)
	raw, err := SaveBag(json.RawMessage(`{}`), Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 121, Template: 900, Amount: 1, ExpireTime: 12}}})
	require.NoError(t, err)
	out, _, mail, err := a.GrantOrMail(raw, 900, 8)
	require.NoError(t, err)
	next, err := ReadBag(out)
	require.NoError(t, err)
	require.EqualValues(t, 1, next.Items[0].Amount)
	require.Len(t, mail, 1)
	require.EqualValues(t, 8, mail[0].Stack.Amount)
	// The same instance can be claimed using the existing mailbox path.
	next.Items = nil
	next, err = next.AddMailItem(a.Catalog, a.Rules, a.Equipment, mail[0])
	require.NoError(t, err)
	require.EqualValues(t, GrantExpireTime, next.Items[0].ExpireTime)
}

func TestYuanmaOverflowEquipmentUsesRemainingSlotAndPreservesInstance(t *testing.T) {
	a := yuanmaAwarder(t)
	raw, err := SaveBag(json.RawMessage(`{"future_character":42}`), Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 20002, Period: 5}}})
	require.NoError(t, err)
	out, receipt, mail, err := a.GrantOrMail(raw, 20002, 2)
	require.NoError(t, err)
	require.Equal(t, []uint16{10}, receipt.Slots)
	require.Len(t, mail, 1)
	next, err := ReadBag(out)
	require.NoError(t, err)
	require.Len(t, next.Equipment, 2)
	require.EqualValues(t, 5, next.Equipment[0].Period)
	require.EqualValues(t, GrantExpireTime, mail[0].Equipment.Period)
	require.Equal(t, next.Equipment[1].Durability, mail[0].Equipment.Durability)
	fullItem := *mail[0].Equipment
	next.Equipment = nil
	next, err = next.AddMailItem(a.Catalog, a.Rules, a.Equipment, mail[0])
	require.NoError(t, err)
	fullItem.Slot = next.Equipment[0].Slot
	require.Equal(t, fullItem, next.Equipment[0])
}

func TestYuanmaOverflowInvalidSourceAndAmountsNeverMail(t *testing.T) {
	a := yuanmaAwarder(t)
	for _, v := range [][2]uint32{{900, 0}, {900, 11}, {999999999, 1}, {20002, 3}} {
		_, _, mail, err := a.GrantOrMail(json.RawMessage(`{}`), v[0], v[1])
		require.Error(t, err)
		require.Empty(t, mail)
	}
}

func TestYuanmaOverflowEquipmentRetainsTrainingSeal(t *testing.T) {
	SetInBoostTraining(func(json.RawMessage) bool { return true })
	t.Cleanup(func() { SetInBoostTraining(nil) })
	a := yuanmaAwarder(t)
	raw, err := SaveBag(json.RawMessage(`{"training":true}`), Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 20002}, {Slot: 10, Template: 20002}}})
	require.NoError(t, err)
	_, _, mail, err := a.GrantOrMail(raw, 20002, 1)
	require.NoError(t, err)
	require.Len(t, mail, 1)
	require.True(t, mail[0].Equipment.TutorialLocked)
}
