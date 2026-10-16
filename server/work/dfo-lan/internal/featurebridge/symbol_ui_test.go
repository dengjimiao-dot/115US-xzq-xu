package featurebridge

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestSymbolNativeUIQuoteBoundsAndLegacyPrefix(t *testing.T) {
	v := SymbolView{Role: 2, Source: strings.Repeat("a", 64), Groups: []SymbolGroup{{Name: "red_symbol", Mask: 1, Items: []SymbolItem{{}}}}, Growth: &SymbolGrowthView{Options: []SymbolGrowthOption{{Operation: 4, Slot: 289, Template: 20001, UI: SymbolGrowthUIQuote{MaterialTemplate: 30001, MaterialAmount: 45, Points: 71, SuccessRate: 1200, RateUnit: 10000, Results: [][2]uint32{{3, 8800}, {7, 1200}}}}}}}
	legacy, err := v.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	v.Growth.UI = &SymbolGrowthUI{MaterialTemplate: 30001, FailRewardTemplate: 30002, MaterialBalance: 1 << 33, FailRewardBalance: 8, Lights: []uint32{0, 1, 2, 3, 4, 0, 1, 2, 3, 4}}
	p, err := v.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p[:len(legacy)], legacy) || string(p[len(legacy):len(legacy)+4]) != "SGU1" {
		t.Fatal("legacy prefix changed")
	}
	if binary.LittleEndian.Uint64(p[len(legacy)+12:]) != 1<<33 {
		t.Fatal("material balance truncated")
	}
	v.Growth.Visual = &SymbolGrowthVisual{Operation: 4, Outcome: 4, Item: 20001, Amount: 1}
	visual, err := v.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(visual[:len(p)], p) || string(visual[len(p):len(p)+4]) != "SGV1" || binary.LittleEndian.Uint32(visual[len(p)+8:]) != 20001 {
		t.Fatal("typed result or legacy prefix changed")
	}
	v.Growth.Visual.Outcome = 5
	if _, err := v.Bytes(); err == nil {
		t.Fatal("invalid result outcome accepted")
	}
	v.Growth.Visual = nil
	mutate := []func(){func() { v.Growth.UI.Lights = v.Growth.UI.Lights[:9] }, func() { v.Growth.UI.Lights[0] = 5 }, func() { v.Growth.UI.MaterialTemplate = 1 }, func() { v.Growth.Options[0].UI.Points = 101 }, func() { v.Growth.Options[0].UI.RateUnit = 1199 }, func() { v.Growth.Options[0].UI.Results = make([][2]uint32, 9) }}
	for i, bad := range mutate {
		v.Growth.UI = &SymbolGrowthUI{MaterialTemplate: 30001, FailRewardTemplate: 30002, Lights: []uint32{0, 1, 2, 3, 4, 0, 1, 2, 3, 4}}
		v.Growth.Options[0].UI = SymbolGrowthUIQuote{Points: 71, SuccessRate: 1200, RateUnit: 10000}
		bad()
		if _, err := v.Bytes(); err == nil {
			t.Fatalf("accepted malformed UI case %d", i)
		}
	}
}
