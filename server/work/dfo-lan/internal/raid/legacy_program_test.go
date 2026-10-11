package raid

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func legacyTestSections(v ...any) []catalog.LegacyRaidSection {
	var ts []pvf.Token
	for _, x := range v {
		switch value := x.(type) {
		case int:
			ts = append(ts, pvf.Token{Type: 0, Value: int32(value)})
		case string:
			kind := byte(3)
			if value[0] != '[' {
				kind = 6
			}
			ts = append(ts, pvf.Token{Type: kind, Text: value})
		}
	}
	s, err := catalog.LegacyRaidSections(ts)
	if err != nil {
		panic(err)
	}
	return s
}
func legacyFixture() *catalog.LegacyRaidRules {
	return &catalog.LegacyRaidRules{Path: "test", SHA256: "source", Symbols: map[string]int32{"[INIT]": 1, "[HP]": 2, "[COUNT]": 3}, Modes: map[string][]catalog.LegacyRaidPhase{"normal": {{Dungeons: map[int32]catalog.LegacyRaidDungeon{10: {ID: 10, HPSymbol: "[HP]", HP: 100, InitialState: "open", PartyMax: 1}}, Events: []catalog.LegacyRaidEvent{
		{Trigger: legacyTestSections("[IF]", "[INIT]", 0), Behavior: legacyTestSections("[SET]", "[INIT]", 1, "[SET TIMER]", 1, 2, 2)},
		{Trigger: legacyTestSections("[CHECK TIMER END]", 1, 2), Behavior: legacyTestSections("[SET]", "[COUNT]", "[+]", 1)},
		{Trigger: legacyTestSections("[ON CLEAR DUNGEON]", 10), Behavior: legacyTestSections("[SET DUNGEON STATE]", 10, "clear", "[CLEAR PHASE]")},
	}}}}}
}
func TestLegacyCascadeTimersTerminalAndIsolation(t *testing.T) {
	r := legacyFixture()
	p, err := NewLegacyPhase(r, "normal", 0, "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Modes["normal"][0].Events[1].Behavior = nil
	if err = p.Apply("run", "source", 2000, LegacyEvent{}); err != nil {
		t.Fatal(err)
	}
	if p.View().Symbols["[COUNT]"] != 1 {
		t.Fatal("timer skipped or catalog alias")
	}
	saved := p.View()
	saved.Symbols["[COUNT]"] = 99
	if p.View().Symbols["[COUNT]"] != 1 {
		t.Fatal("view aliases state")
	}
	for _, e := range []struct {
		run, source string
		now         int64
		event       LegacyEvent
	}{{"old", "source", 2000, LegacyEvent{}}, {"run", "old", 2000, LegacyEvent{}}, {"run", "source", 1999, LegacyEvent{}}, {"run", "source", 2000, LegacyEvent{Name: "initial"}}} {
		before := p.View()
		if err = p.Apply(e.run, e.source, e.now, e.event); err == nil || !reflect.DeepEqual(before, p.View()) {
			t.Fatal("stale request changed state")
		}
	}
	if err = p.Apply("run", "source", 2001, LegacyEvent{Name: "on clear dungeon", A: 10}); err != nil || !p.View().Cleared {
		t.Fatal(err)
	}
	before := p.View()
	if err = p.Apply("run", "source", 2002, LegacyEvent{Name: "on clear dungeon", A: 10}); err == nil || !reflect.DeepEqual(before, p.View()) {
		t.Fatal("terminal event accepted")
	}
}
func TestLegacyBindingFailureRollsBackWholeCascade(t *testing.T) {
	r := legacyFixture()
	r.Modes["normal"][0].Events = append(r.Modes["normal"][0].Events, catalog.LegacyRaidEvent{Trigger: legacyTestSections("[ON CHANGE SYMBOL]", "[COUNT]"), Behavior: legacyTestSections("[SOURCE SPECIAL ACTION]")})
	if _, err := NewLegacyPhase(r, "normal", 0, "run", nil); err == nil {
		t.Fatal("missing action silently accepted")
	}
	p, err := NewLegacyPhase(r, "normal", 0, "run", map[string]LegacyBinding{"source special action": func(v *LegacyView, _ LegacyInstruction) error {
		v.Symbols["[HP]"] = 0
		return fmt.Errorf("unverified action")
	}})
	if err != nil {
		t.Fatal(err)
	}
	before := p.View()
	if err = p.Apply("run", "source", 3000, LegacyEvent{}); err == nil || !reflect.DeepEqual(before, p.View()) {
		t.Fatal("partial timer/cascade committed")
	}
}
func TestLegacyArithmeticOverflowRollsBack(t *testing.T) {
	r := legacyFixture()
	r.Modes["normal"][0].Events[1].Behavior = legacyTestSections("[SET]", "[COUNT]", 2147483647, "[SET]", "[COUNT]", "[+]", 1)
	p, err := NewLegacyPhase(r, "normal", 0, "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	before := p.View()
	if err = p.Apply("run", "source", 2000, LegacyEvent{}); err == nil || !reflect.DeepEqual(before, p.View()) {
		t.Fatal("overflow committed")
	}
}
func TestLegacyCurrentUSGrammar(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native source required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"etc/raid/siroco/siroco.etc", "contents/2021/ozma_raid/etc/ozma.etc"} {
		r, err := catalog.ImportLegacyRaidRules(a, path)
		if err != nil {
			t.Fatal(err)
		}
		for mode, phases := range r.Modes {
			for _, p := range phases {
				actions := map[string]int{}
				for n, e := range p.Events {
					if _, err := compileLegacy(r, e.Trigger, true); err != nil {
						t.Errorf("%s %s phase=%d event=%d trigger: %v", path, mode, p.Index, n, err)
					}
					code, err := compileLegacy(r, e.Behavior, false)
					if err != nil {
						t.Errorf("%s %s phase=%d event=%d action: %v", path, mode, p.Index, n, err)
					}
					for _, i := range code {
						actions[i.Op]++
					}
				}
				t.Logf("%s/%s/%d compiled events=%d actions=%v (grammar only; specialized bindings/wire pending)", path, mode, p.Index, len(p.Events), actions)
			}
		}
	}
}

func TestLegacySirocoAuthoredEqualValueDispatch(t *testing.T) {
	r := legacyFixture()
	r.Path = "etc/raid/siroco/siroco.etc"
	r.Modes["normal"][0].Events[1].Behavior = legacyTestSections("[SET]", "[HP]", 100)
	r.Modes["normal"][0].Events = append(r.Modes["normal"][0].Events, catalog.LegacyRaidEvent{Trigger: legacyTestSections("[ON CHANGE SYMBOL]", "[HP]"), Behavior: legacyTestSections("[SET]", "[COUNT]", "[+]", 1)})
	p, err := NewLegacyPhase(r, "normal", 0, "run", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.Apply("run", "source", 2000, LegacyEvent{}); err != nil {
		t.Fatal(err)
	}
	if p.View().Symbols["[COUNT]"] != 1 {
		t.Fatal("authored equal SET did not dispatch")
	}
}
