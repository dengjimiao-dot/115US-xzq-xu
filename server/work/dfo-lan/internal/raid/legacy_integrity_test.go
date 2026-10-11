package raid

import (
	"dfolan/internal/catalog"
	"reflect"
	"testing"
)

func TestLegacyStartClockAndSourceRoute(t *testing.T) {
	r := legacyFixture()
	r.Symbols["[SINGLE RAID TYPE]"] = 4
	r.Modes["normal"][0].Events[0].Trigger = legacyTestSections("[IF]", "[SINGLE RAID TYPE]", 2)
	p, err := NewLegacyPhaseAt(r, "normal", 0, "run", LegacyStart{Now: 1700000000000, SingleType: 2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.View().Timers[LegacyTimerKey{1, 2}].Due; got != 1700000002000 {
		t.Fatalf("timer due=%d", got)
	}
	if err = p.Apply("run", "source", 1700000001999, LegacyEvent{}); err != nil {
		t.Fatal(err)
	}
	if p.View().Symbols["[COUNT]"] != 0 {
		t.Fatal("timer fired early")
	}
	if err = p.Apply("run", "source", 1700000002000, LegacyEvent{}); err != nil {
		t.Fatal(err)
	}
	if p.View().Symbols["[COUNT]"] != 1 {
		t.Fatal("timer not fired")
	}
	delete(r.Symbols, "[SINGLE RAID TYPE]")
	if _, err = NewLegacyPhaseAt(r, "normal", 0, "run", LegacyStart{SingleType: 2}, nil); err == nil {
		t.Fatal("undeclared route accepted")
	}
}

func bindingFixture(t *testing.T, binding LegacyBinding) *LegacyPhase {
	t.Helper()
	r := legacyFixture()
	r.Modes["normal"][0].Events[1].Behavior = legacyTestSections("[SOURCE SPECIAL ACTION]")
	p, err := NewLegacyPhase(r, "normal", 0, "run", map[string]LegacyBinding{"source special action": binding})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLegacyInvalidBindingStateRollsBack(t *testing.T) {
	cases := map[string]func(*LegacyView){
		"past timer":          func(v *LegacyView) { v.Timers[LegacyTimerKey{3, 4}] = LegacyTimer{Due: v.Now - 1} },
		"zero delay":          func(v *LegacyView) { v.Timers[LegacyTimerKey{3, 4}] = LegacyTimer{Due: v.Now} },
		"unknown room":        func(v *LegacyView) { v.Occupants[99] = []byte{1} },
		"capacity":            func(v *LegacyView) { v.Occupants[10] = []byte{1, 2} },
		"invalid state":       func(v *LegacyView) { v.Dungeons[10] = "typo" },
		"invalid reservation": func(v *LegacyView) { v.Timers[LegacyTimerKey{-1, 99}] = LegacyTimer{Due: v.Now + 1000, State: "open"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := bindingFixture(t, func(v *LegacyView, _ LegacyInstruction) error { v.Symbols["[HP]"] = 0; mutate(v); return nil })
			before := p.View()
			if err := p.Apply("run", "source", 2000, LegacyEvent{}); err == nil {
				t.Fatal("invalid binding accepted")
			}
			if !reflect.DeepEqual(before, p.View()) {
				t.Fatal("partial mutation committed")
			}
		})
	}
}

func TestLegacyBindingAllowsOtherSimultaneousTimer(t *testing.T) {
	r := legacyFixture()
	r.Modes["normal"][0].Events[0].Behavior = append(r.Modes["normal"][0].Events[0].Behavior, legacyTestSections("[SET TIMER]", 1, 3, 2)...)
	r.Modes["normal"][0].Events[1].Behavior = legacyTestSections("[SOURCE SPECIAL ACTION]")
	r.Modes["normal"][0].Events = append(r.Modes["normal"][0].Events, catalog.LegacyRaidEvent{Trigger: legacyTestSections("[CHECK TIMER END]", 1, 3), Behavior: legacyTestSections("[SET]", "[COUNT]", "[+]", 1)})
	p, err := NewLegacyPhase(r, "normal", 0, "run", map[string]LegacyBinding{"source special action": func(v *LegacyView, _ LegacyInstruction) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Apply("run", "source", 2000, LegacyEvent{}); err != nil {
		t.Fatal(err)
	}
	if p.View().Symbols["[COUNT]"] != 1 {
		t.Fatal("simultaneous timer lost")
	}
}
