package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/featurebridge"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"strings"
	"testing"
)

// Explicit real-source integration; it uses a temporary in-memory character.
func TestSymbolGrowthCandidateSourceClosure(t *testing.T) {
	path := os.Getenv("SYMBOL_GROWTH_CANDIDATE")
	if path == "" {
		t.Skip("explicit candidate PVF integration")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, "")
	require.NoError(t, e)
	defer a.Close()
	growth, e := ImportEmblemGrowthRules(a)
	require.NoError(t, e)
	resonance, e := ImportSymbolEmblemRules(a)
	require.NoError(t, e)
	require.NotNil(t, growth)
	require.Len(t, growth.ByItem, 981)
	c := catalog.LootCatalog{Source: a.Snapshot(), Items: map[uint32]catalog.LootItem{}}
	index, e := catalog.ResolveScript(a, "list/stackable.lst")
	require.NoError(t, e)
	entries, e := catalog.ParseIndex(index.Cells)
	require.NoError(t, e)
	needed := map[uint32]bool{}
	needed[resonance.CostItem] = true
	needed[resonance.Tuning.FailRewardItem] = true
	for id := range growth.ByItem {
		needed[id] = true
	}
	for _, o := range growth.Shop.Shops["1"].Offers {
		needed[o.Template] = true
	}
	for _, d := range growth.Disjoint {
		needed[d.Template] = true
	}
	for _, r := range growth.Normal {
		needed[r.Material.Template] = true
	}
	for _, r := range growth.Special {
		needed[r.Material.Template] = true
		needed[r.Replace.Template] = true
	}
	for _, entry := range entries {
		if entry.ID < 2 || !needed[entry.ID] {
			continue
		}
		path := entry.Path
		if !strings.HasPrefix(path, "stackable/") {
			path = "stackable/" + path
		}
		path = catalog.ResolveScriptPath(a, path)
		script, e := catalog.ReadScript(a, path)
		require.NoError(t, e)
		kind := ""
		for i, cell := range script.Cells {
			if cell.Type == 3 && cell.Text == "[stackable type]" && i+1 < len(script.Cells) {
				kind = script.Cells[i+1].Text
			}
		}
		item, e := catalog.ImportStackableItem(a, catalog.ItemIndexEntry{ID: entry.ID, Path: path, Kind: "stackable", StackableType: kind})
		require.NoError(t, e, "item %d", entry.ID)
		item.Script = script
		c.Items[entry.ID] = item
	}
	for id := range growth.ByItem {
		item, ok := c.Items[id]
		require.True(t, ok, "missing %d", id)
		require.Equal(t, "[avatar emblem]", item.StackableType)
		grade, e := symbolItemScalar(c, id, "[grade]")
		require.NoError(t, e)
		require.Contains(t, growth.Groups[growth.ByItem[id]].Grades, grade)
		require.NotEmpty(t, growth.Names[id])
	}
	for _, o := range growth.Shop.Shops["1"].Offers {
		require.Contains(t, c.Items, o.Template)
		require.NotEmpty(t, growth.Names[o.Template])
		_, e := symbolItemNumbers(c, o.Template, "[need material]")
		require.NoError(t, e)
	}
	// A real source conversion coupon must keep its source B-material fee.
	change, e := symbolChangeRule(c, 490718840)
	require.NoError(t, e)
	require.NotEmpty(t, change.Inputs)
	require.NotEmpty(t, change.Results)
	require.NotEmpty(t, change.Materials)
	// Exercise the actual source quotes at the existing 72-cell emblem capacity.
	// Select the source emblem with the largest per-cell quote footprint.
	_, _, layout, _, _ := symbolFixture(t)
	layout.Source = growth.Source
	layout.Growth = growth
	layout.Rules = resonance
	service := ItemService{Catalog: c, SymbolSockets: layout}
	// Exercise a real source normal breakthrough. A controlled final bucket
	// selects the highest source result, without asserting a native RNG contract.
	_, bagRules, _, _, _ := symbolFixture(t)
	bagRules.Source = growth.Source
	service.BagRules = bagRules
	var input uint32
	for id := range growth.ByItem {
		grade, err := symbolItemScalar(c, id, "[grade]")
		require.NoError(t, err)
		if grade == 2 && (input == 0 || id < input) {
			input = id
		}
	}
	require.NotZero(t, input)
	fee := growth.Normal[2].Material
	upgradeState, e := json.Marshal(map[string]any{"inventory": Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 289, Template: input, Amount: 1}, {Slot: 121, Template: fee.Template, Amount: fee.Amount}}}})
	require.NoError(t, e)
	upgraded, _, _, e := service.PrepareSymbolGrowth(upgradeState, json.RawMessage(`{}`), SymbolGrowthRequest{Source: growth.Source, Operation: SymbolNormalUpgrade, Slot: 289, Template: input}, 100, func(n uint32) (uint32, error) { return n - 1, nil })
	require.NoError(t, e)
	resultBag, e := ReadBag(upgraded)
	require.NoError(t, e)
	var output uint32
	for _, row := range resultBag.Items {
		if c.Items[row.Template].StackableType == "[avatar emblem]" {
			output = row.Template
		}
	}
	require.NotZero(t, output)
	outputGrade, e := symbolItemScalar(c, output, "[grade]")
	require.NoError(t, e)
	require.Equal(t, uint32(8), outputGrade)
	outputFame, e := symbolItemScalar(c, output, "[fame value]")
	require.NoError(t, e)
	require.Equal(t, uint32(80), outputFame)
	t.Logf("actual source breakthrough %d -> %d grade=%d native fame=%d", input, output, outputGrade, outputFame)
	// Quote and execution consume the adjusted CN CTP rates.
	for _, bonus := range []uint32{100, 250} {
		for _, succeeds := range []bool{true, false} {
			t.Run(fmt.Sprintf("source-probability/bonus%d/success%v", bonus, succeeds), func(t *testing.T) {
				original := layout.Rules.Tuning.DuplicateButtonBonusRate
				defer func() { layout.Rules.Tuning.DuplicateButtonBonusRate = original }()
				require.Equal(t, uint32(100), original)
				layout.Rules.Tuning.DuplicateButtonBonusRate = bonus
				saved, err := ReadSymbolSockets(json.RawMessage(`{}`), layout)
				require.NoError(t, err)
				saved.Progress["red_symbol"] = SymbolProgress{Grade: 0, Level: 4}
				step := layout.Rules.Steps[SymbolStepKey{0, 4}]
				require.Equal(t, uint32(8500), step.SuccessRate)
				require.Equal(t, uint32(75), step.Cost)
				state, err := json.Marshal(map[string]any{"symbol_emblem_sockets": saved, "symbol_growth": SymbolGrowthState{Lights: []uint32{0, 0, 0, 3, 3, 3, 3, 3, 3, 3}}})
				require.NoError(t, err)
				counts, err := json.Marshal(AccountMaterials{Version: "account-materials-v1", CommonItems: map[uint32]uint32{layout.Rules.CostItem: 1000}})
				require.NoError(t, err)
				quote, err := service.SymbolGrowthDisplay(state, counts, 100)
				require.NoError(t, err)
				wantRate := uint32(8500) + 3*bonus
				var option SymbolGrowthOption
				for _, row := range quote.Options {
					if row.Request.Operation == SymbolResonate && row.Request.Argument == 0 {
						option = row
					}
				}
				require.Contains(t, option.Label, fmt.Sprintf("成功率%.2f%%", float64(wantRate)/100))
				calls := 0
				next, paid, _, err := service.PrepareSymbolGrowth(state, counts, option.Request, 100, func(unit uint32) (uint32, error) {
					calls++
					if calls > 1 {
						return unit - 1, nil
					}
					if succeeds {
						return wantRate - 1, nil
					}
					return wantRate, nil
				})
				require.NoError(t, err)
				progress, err := ReadSymbolSockets(next, layout)
				require.NoError(t, err)
				wantLevel := uint32(4)
				if succeeds {
					wantLevel++
				}
				require.Equal(t, wantLevel, progress.Progress["red_symbol"].Level)
				materials, err := ReadAccountMaterials(paid)
				require.NoError(t, err)
				require.Equal(t, uint32(925), materials.CommonItems[layout.Rules.CostItem])
			})
		}
	}
	makeState := func(id uint32, n int) json.RawMessage {
		bag := Bag{Version: "ordinary-bag-v1"}
		for j := 0; j < n; j++ {
			bag.Items = append(bag.Items, BagItem{Slot: uint16(289 + j), Template: id, Amount: 2})
		}
		state, err := json.Marshal(map[string]any{"inventory": bag})
		require.NoError(t, err)
		return state
	}
	worst, largest := uint32(0), 0
	for id := range growth.ByItem {
		display, err := service.SymbolGrowthDisplay(makeState(id, 1), json.RawMessage(`{}`), 100)
		require.NoError(t, err)
		size := 0
		for _, option := range display.Options {
			size += 18 + len(option.Label)
		}
		if size > largest {
			worst, largest = id, size
		}
	}
	display, e := service.SymbolGrowthDisplay(makeState(worst, 72), json.RawMessage(`{}`), 100)
	require.NoError(t, e)
	view := featurebridge.SymbolView{Role: 1, Source: growth.Source, Growth: &featurebridge.SymbolGrowthView{Materials: display.Materials, Result: display.Result, Progress: display.Progress, Lights: display.Lights}, Groups: []featurebridge.SymbolGroup{{Name: "red_symbol", Mask: 1, Items: []featurebridge.SymbolItem{{}}}}}
	if u := display.UI; u != nil {
		view.Growth.UI = &featurebridge.SymbolGrowthUI{MaterialTemplate: u.MaterialTemplate, FailRewardTemplate: u.FailRewardTemplate, MaterialBalance: u.MaterialBalance, FailRewardBalance: u.FailRewardBalance, Lights: u.Lights}
	}
	for _, option := range display.Options {
		r := option.Request
		view.Growth.Options = append(view.Growth.Options, featurebridge.SymbolGrowthOption{Operation: r.Operation, Slot: r.Slot, Template: r.Template, Argument: r.Argument, Amount: r.Amount, Label: option.Label, UI: featurebridge.SymbolGrowthUIQuote{MaterialTemplate: option.UI.MaterialTemplate, MaterialAmount: option.UI.MaterialAmount, Points: option.UI.Points, SuccessRate: option.UI.SuccessRate, RateUnit: option.UI.RateUnit, Results: option.UI.Results}})
	}
	payload, e := view.Bytes()
	require.NoError(t, e)
	require.Less(t, len(payload), featurebridge.MaxBody-8192, "reserve existing attribute and socket projection space")
	t.Logf("72-cell worst source emblem %d: %d options, %d growth bytes", worst, len(display.Options), len(payload))
	raw, e := json.Marshal(growth)
	require.NoError(t, e)
	t.Logf("candidate source %s: %d grouped emblems, %d merchant offers, %d parsed bytes", growth.Source, len(growth.ByItem), len(growth.Shop.Shops["1"].Offers), len(raw))
}

func TestSymbolGrowthSharedMaterialShopPayment(t *testing.T) {
	m, e := ReadAccountMaterials(json.RawMessage(`{"version":"account-materials-v1","common_items":{"30001":7}}`))
	require.NoError(t, e)
	_, paid, e := PayMaterialsWithStore(Bag{}, m, []MaterialCost{{Template: 30001, Count: 3}}, 2)
	require.NoError(t, e)
	require.Equal(t, uint32(1), paid.CommonItems[30001])
	require.Equal(t, uint32(7), m.CommonItems[30001])
	_, _, e = PayMaterialsWithStore(Bag{}, paid, []MaterialCost{{Template: 30001, Count: 2}}, 1)
	require.Error(t, e)
}
