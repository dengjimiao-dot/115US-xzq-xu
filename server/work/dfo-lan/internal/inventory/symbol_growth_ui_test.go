package inventory

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSymbolNativeUIProjectionFollowsParsedSource(t *testing.T) {
	s, state, counts := growthFixture(t)
	rule := s.SymbolSockets.Growth.Normal[2]
	rule.Material.Amount = 17
	rule.Results = []EmblemGradeWeight{{2, 7000}, {3, 3000}}
	s.SymbolSockets.Growth.Normal[2] = rule
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(state, &fields))
	fields["symbol_growth"], _ = json.Marshal(SymbolGrowthState{Points: map[uint32]uint32{2: 43}, Lights: []uint32{0, 0, 1, 1, 2, 2, 3, 3, 4, 4}})
	state, _ = json.Marshal(fields)
	display, err := s.SymbolGrowthDisplay(state, counts, 100)
	require.NoError(t, err)
	require.NotNil(t, display.UI)
	require.Equal(t, uint32(30001), display.UI.MaterialTemplate)
	require.Equal(t, uint64(100), display.UI.MaterialBalance)
	require.Len(t, display.UI.Lights, 10)
	found := false
	for _, o := range display.Options {
		if o.Request.Operation == SymbolNormalUpgrade {
			found = true
			require.Equal(t, uint32(17), o.UI.MaterialAmount)
			require.Equal(t, uint32(43), o.UI.Points)
			require.Equal(t, [][2]uint32{{2, 7000}, {3, 3000}}, o.UI.Results)
			require.Equal(t, uint32(10000), o.UI.RateUnit)
		}
	}
	require.True(t, found)
}

func TestSymbolNativeResultReflectsCommittedOutcome(t *testing.T) {
	s, state, counts := growthFixture(t)
	zero := func(uint32) (uint32, error) { return 0, nil }
	req := SymbolGrowthRequest{Source: s.Catalog.Source.Checksum, Operation: SymbolNormalUpgrade, Slot: 289, Template: 20001}
	next, after, _, err := s.PrepareSymbolGrowth(state, counts, req, 100, zero)
	require.NoError(t, err)
	g, err := ReadSymbolGrowth(next)
	require.NoError(t, err)
	require.Equal(t, &SymbolGrowthVisual{Operation: 4, Outcome: 4, Item: 20001, Amount: 1}, g.Visual)
	display, err := s.SymbolGrowthDisplay(next, after, 100)
	require.NoError(t, err)
	require.Equal(t, g.Visual, display.Visual)
	// A rejected action produces no visual receipt and no replacement state.
	req.Sequence = 0
	refused, _, _, err := s.PrepareSymbolGrowth(next, after, req, 100, zero)
	require.Error(t, err)
	require.Nil(t, refused)
	req.Sequence = 1
	req.Operation = SymbolDisjoint
	req.Amount = 1
	next, _, _, err = s.PrepareSymbolGrowth(next, after, req, 100, zero)
	require.NoError(t, err)
	g, err = ReadSymbolGrowth(next)
	require.NoError(t, err)
	require.Equal(t, &SymbolGrowthVisual{Operation: 3, Outcome: 2, Item: 30001, Amount: 3}, g.Visual)
}
