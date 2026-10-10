package legion

import (
	"bytes"
	"fmt"
	"testing"
)

func TestBakalFinalActorClearExecutesSourceReturnAndRetainsRewardGate(t *testing.T) {
	o, now := runtimeNative(t, false)
	final := uint32(o.script.FinalClearDungeon)
	o.stage = BakalOpeningFinal
	o.current = final
	o.loaded[final] = true
	if e := o.ClearFinalDungeon(now); e != nil {
		t.Fatal(e)
	}
	if !o.cleared[final] || o.runtime.states[final] != "clear" || !o.NeedsCampReturn() || o.current != 0 || !o.SettleDue(now) {
		t.Fatal("source final clear/return was not executed")
	}
	name := fmt.Sprintf("[%d DGN HP]", final)
	if o.symbolValues[name] != 1 {
		t.Fatal("final dungeon completion symbol missing")
	}
	expected := BakalSourceSymbolFrame(o.rules.Symbols[name], 1)
	found := false
	for _, f := range o.runtime.frames {
		if f.ID == 570 && bytes.Equal(f.Body, expected) {
			found = true
		}
	}
	if !found {
		t.Fatal("native final completion not published")
	}
	n := len(o.runtime.frames)
	if e := o.ClearFinalDungeon(now); e != nil || len(o.runtime.frames) != n {
		t.Fatal("duplicate final death repeated event", e)
	}
	o.Tick(now)
	if o.stage != BakalOpeningFinal {
		t.Fatal("settled without durable reward preparation")
	}
}
