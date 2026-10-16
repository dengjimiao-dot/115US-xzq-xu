package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func growthFixture(t *testing.T) (*ItemService, json.RawMessage, json.RawMessage) {
	source := strings.Repeat("a", 64)
	c, bag, l, m, state := symbolFixture(t)
	item := func(id uint32, kind string, grade, rarity int32, shared bool) catalog.LootItem {
		cells := []pvf.Token{{Type: 3, Text: "[grade]"}, {Type: 0, Value: grade}, {Type: 3, Text: "[rarity]"}, {Type: 0, Value: rarity}}
		if shared {
			cells = append(cells, pvf.Token{Type: 3, Text: "[stack in account common item inventory]"})
		}
		return catalog.LootItem{ID: id, Kind: "stackable", StackableType: kind, StackLimit: 1000, Script: catalog.ScriptRecord{Cells: cells}}
	}
	for _, id := range []uint32{20001, 20002, 20003} {
		c.Items[id] = item(id, "[avatar emblem]", 2, 2, false)
	}
	c.Items[20004] = item(20004, "[avatar emblem]", 3, 3, false)
	c.Items[30001] = item(30001, "[material]", 1, 2, true)
	c.Items[30002] = item(30002, "[material]", 1, 2, true)
	bag.Slots["[material]"] = [2]uint16{121, 125}
	bag.Slots["[avatar emblem]"] = [2]uint16{289, 292}
	l.Rules = &SymbolEmblemRules{SourceChecksum: source, CostItem: 30001, Tiers: map[SymbolTierKey]SymbolTier{}, Fixed: map[uint32]SymbolOptions{1: {}}, Steps: map[SymbolStepKey]SymbolResonanceStep{{0, 1}: {Cost: 3, SuccessRate: 10000, ButtonWeight: 10}, {0, 2}: {Cost: 4, SuccessRate: 0, ButtonWeight: 10}}, Tuning: SymbolTuningRules{ComboRates: map[uint32]uint32{1: 0}, MultiColorCost: 7, MultiColorAppearanceRate: 0, FailRewardItem: 30002, FailRewardCount: 1}}
	for typ := uint32(0); typ < 5; typ++ {
		l.Rules.Tiers[SymbolTierKey{typ, 0}] = SymbolTier{Levels: map[uint32]SymbolLevel{1: {OptionIndex: 1, FameCell: 1}, 2: {OptionIndex: 1, FameCell: 1}}}
		l.Rules.Tiers[SymbolTierKey{typ, 1}] = SymbolTier{Levels: map[uint32]SymbolLevel{1: {OptionIndex: 1, FameCell: 1}}}
	}
	l.Growth = &EmblemGrowthRules{Source: source, Normal: map[uint32]EmblemUpgradeRule{2: {Material: EmblemMaterial{30001, 5}, Results: []EmblemGradeWeight{{2, 10000}, {3, 0}}}}, Special: map[uint32]EmblemUpgradeRule{2: {Material: EmblemMaterial{30001, 9}, Results: []EmblemGradeWeight{{3, 10000}}}}, Guarantee: map[uint32]EmblemGuaranteeRule{2: {3, 25, 25}}, Disjoint: map[uint32]EmblemDisjointRule{2: {30001, 3, 7}}, ByItem: map[uint32]int{20001: 0}, Groups: []EmblemGrowthGroup{{Grades: map[uint32][]uint32{2: {20001}, 3: {20004}}}}}
	s := &ItemService{Catalog: c, BagRules: bag, SymbolSockets: l, EmblemInlay: m}
	return s, state, json.RawMessage(`{"version":"account-materials-v1","counts":{"363":9},"common_items":{"30001":100}}`)
}
func TestSymbolGrowthAtomicFailureAndExpiry(t *testing.T) {
	s, state, counts := growthFixture(t)
	zero := func(uint32) (uint32, error) { return 0, nil }
	req := SymbolGrowthRequest{Source: s.Catalog.Source.Checksum, Operation: SymbolSpecialUpgrade, Slot: 289, Template: 20001}
	// The output grade has no free emblem cell. Deducting first must still fail atomically.
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(state, &fields))
	var b Bag
	require.NoError(t, json.Unmarshal(fields["inventory"], &b))
	b.Items[0].Amount = 3
	b.Items = append(b.Items, BagItem{Slot: 291, Template: 20003, Amount: 1}, BagItem{Slot: 292, Template: 20002, Amount: 1})
	fields["inventory"], _ = json.Marshal(b)
	full, _ := json.Marshal(fields)
	next, after, _, e := s.PrepareSymbolGrowth(full, counts, req, 100, zero)
	require.Error(t, e)
	require.Nil(t, next)
	require.Nil(t, after)
	b.Items[0].ExpireTime = 99
	fields["inventory"], _ = json.Marshal(b)
	expired, _ := json.Marshal(fields)
	_, _, _, e = s.PrepareSymbolGrowth(expired, counts, req, 100, zero)
	require.ErrorContains(t, e, "变化")
	req.Source = strings.Repeat("b", 64)
	_, _, _, e = s.PrepareSymbolGrowth(state, counts, req, 100, zero)
	require.ErrorContains(t, e, "源")
}
func TestSymbolGrowthGuaranteeAndSharedPreservation(t *testing.T) {
	s, state, counts := growthFixture(t)
	zero := func(uint32) (uint32, error) { return 0, nil }
	req := SymbolGrowthRequest{Source: s.Catalog.Source.Checksum, Operation: SymbolNormalUpgrade, Slot: 289, Template: 20001}
	for i := 0; i < 5; i++ {
		req.Sequence = uint64(i)
		var e error
		state, counts, _, e = s.PrepareSymbolGrowth(state, counts, req, 100, zero)
		require.NoError(t, e)
	}
	b, e := ReadBag(state)
	require.NoError(t, e)
	found := false
	for _, i := range b.Items {
		if i.Template == 20004 {
			found = true
		}
	}
	require.True(t, found)
	growth, e := ReadSymbolGrowth(state)
	require.NoError(t, e)
	require.Zero(t, growth.Points[2])
	m, e := ReadAccountMaterials(counts)
	require.NoError(t, e)
	require.Equal(t, uint32(75), m.CommonItems[30001])
	m, _, e = m.Add(3033, 1)
	require.NoError(t, e)
	saved, e := m.Save()
	require.NoError(t, e)
	again, e := ReadAccountMaterials(saved)
	require.NoError(t, e)
	require.Equal(t, uint32(75), again.CommonItems[30001])
	require.Equal(t, uint32(10), again.Counts[363])
}
func TestSymbolGrowthColorProgressAndFailureCompensation(t *testing.T) {
	s, state, counts := growthFixture(t)
	zero := func(uint32) (uint32, error) { return 0, nil }
	req := SymbolGrowthRequest{Source: s.Catalog.Source.Checksum, Operation: SymbolGenerateLights}
	var e error
	state, counts, _, e = s.PrepareSymbolGrowth(state, counts, req, 100, zero)
	require.NoError(t, e)
	g, e := ReadSymbolGrowth(state)
	require.NoError(t, e)
	require.Len(t, g.Lights, 10)
	req.Sequence = 1
	_, _, _, e = s.PrepareSymbolGrowth(state, counts, req, 100, zero)
	require.ErrorContains(t, e, "完成")
	req.Operation = SymbolResonate
	state, counts, _, e = s.PrepareSymbolGrowth(state, counts, req, 100, zero)
	require.NoError(t, e)
	slots, e := ReadSymbolSockets(state, s.SymbolSockets)
	require.NoError(t, e)
	require.Equal(t, uint32(2), slots.Progress["red_symbol"].Level)
	require.Equal(t, uint32(1), slots.Progress["multi_symbol"].Level)
	fame, e := SymbolStarFame(s.SymbolSockets, "red_symbol", slots.Progress["red_symbol"])
	require.NoError(t, e)
	require.Equal(t, uint32(2), fame)
	fame, e = SymbolStarFame(s.SymbolSockets, "multi_symbol", slots.Progress["multi_symbol"])
	require.NoError(t, e)
	require.Equal(t, uint32(1), fame)
	// Force a saved red light at the second level, which has a zero base rate.
	g.Lights = []uint32{0, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(state, &fields))
	fields["symbol_growth"], _ = json.Marshal(g)
	state, _ = json.Marshal(fields)
	req.Sequence = 2
	state, counts, _, e = s.PrepareSymbolGrowth(state, counts, req, 100, zero)
	require.NoError(t, e)
	m, e := ReadAccountMaterials(counts)
	require.NoError(t, e)
	require.Equal(t, uint32(1), m.CommonItems[30002])
	require.Equal(t, uint32(93), m.CommonItems[30001])
	slots, e = ReadSymbolSockets(state, s.SymbolSockets)
	require.NoError(t, e)
	require.Equal(t, uint32(2), slots.Progress["red_symbol"].Level)
}
