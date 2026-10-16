package character_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

func TestSymbolEmblemOfficialSourceFame(t *testing.T) {
	if os.Getenv("SYMBOL_GROWTH_CANDIDATE") == "" {
		t.Skip("explicit candidate source required")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: os.Getenv("SYMBOL_GROWTH_CANDIDATE"), MaxBytes: 1024 * 1024 * 1024}, "")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	rules, e := inventory.ImportSymbolEmblemRules(a)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := a.ReadRaw(inventory.SymbolEmblemSourceDirectory + "ui/equipemblemwindow.xui")
	if e != nil {
		t.Fatal(e)
	}
	layout, e := inventory.ParseSymbolSocketLayout(a.Snapshot().Checksum, raw)
	if e != nil {
		t.Fatal(e)
	}
	layout.Rules = rules
	oracle := []uint32{12, 24, 36, 48, 60, 78, 96, 114, 132, 150, 175, 200, 225, 250, 275, 300}
	for _, color := range []string{"red_symbol", "yellow_symbol", "green_symbol", "blue_symbol", "multi_symbol"} {
		for i, want := range oracle {
			grade, level := uint32(i/5), uint32(i%5+1)
			got, e := inventory.SymbolStarFame(layout, color, inventory.SymbolProgress{Grade: grade, Level: level})
			if e != nil {
				t.Fatal(e)
			}
			if got != want {
				t.Errorf("official star fame %s %d/%d got=%d want=%d", color, grade, level, got, want)
			}
		}
	}
	t.Log("80 actual SymbolStarFame results match official April 21 table")
	growth, e := inventory.ImportEmblemGrowthRules(a)
	if e != nil {
		t.Fatal(e)
	}
	index, e := catalog.ResolveScript(a, "list/stackable.lst")
	if e != nil {
		t.Fatal(e)
	}
	entries, e := catalog.ParseIndex(index.Cells)
	if e != nil {
		t.Fatal(e)
	}
	imp, e := character.NewFameItemImport(a)
	if e != nil {
		t.Fatal(e)
	}
	selected := map[uint32]uint32{}
	for _, entry := range entries {
		if _, ok := growth.ByItem[entry.ID]; !ok && entry.ID != 2500007 {
			continue
		}
		path := entry.Path
		if !strings.HasPrefix(path, "stackable/") {
			path = "stackable/" + path
		}
		path = catalog.ResolveScriptPath(a, path)
		script, e := catalog.ReadScript(a, path)
		if e != nil {
			t.Fatal(e)
		}
		body, e := a.ReadRaw(path)
		if e != nil {
			t.Fatal(e)
		}
		e = imp.Consume(catalog.ItemScript{Item: catalog.ItemIndexEntry{ID: entry.ID, Path: path, Kind: "stackable"}, Raw: body, Cells: script.Cells, Exact: true})
		if e != nil {
			t.Fatal(e)
		}
		var grade uint32
		red := false
		for i, c := range script.Cells {
			if c.Type == 3 && c.Text == "[grade]" && i+1 < len(script.Cells) {
				grade = uint32(script.Cells[i+1].Value)
			}
			if c.Type == 6 && c.Text == "[A socket]" {
				red = true
			}
		}
		if red && (grade == 1 || grade == 2 || grade == 3 || grade == 7 || grade == 8) {
			if _, exists := selected[grade]; !exists {
				selected[grade] = entry.ID
			}
		}
	}
	fame, e := imp.Finish()
	if e != nil {
		t.Fatal(e)
	}
	restore, e := character.InstallFameRules(fame)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	t.Logf("actual source importer retained %d item fame entries from 981 emblems", len(fame.Items))
	svc := character.Service{Equipment: &inventory.EquipmentCatalog{Source: a.Snapshot()}, SymbolSockets: layout}
	base := json.RawMessage(`{"level":115,"inventory":{"version":"ordinary-bag-v1","gold":0}}`)
	before, e := svc.EquipmentFameBreakdown(base)
	if e != nil {
		t.Fatal(e)
	}
	wants := map[uint32]uint32{1: 30, 2: 35, 3: 50, 7: 65, 8: 80}
	grades := []int{}
	for r := range selected {
		grades = append(grades, int(r))
	}
	sort.Ints(grades)
	for _, r := range grades {
		id := selected[uint32(r)]
		saved, e := inventory.ReadSymbolSockets(base, layout)
		if e != nil {
			t.Fatal(e)
		}
		saved.Groups["red_symbol"][0] = inventory.SymbolSocketItem{Template: id}
		var state map[string]json.RawMessage
		json.Unmarshal(base, &state)
		state["symbol_emblem_sockets"], e = json.Marshal(saved)
		if e != nil {
			t.Fatal(e)
		}
		with, _ := json.Marshal(state)
		after, e := svc.EquipmentFameBreakdown(with)
		if e != nil {
			t.Fatal(e)
		}
		got := after.Total - before.Total
		want := wants[uint32(r)]
		t.Logf("source emblem grade=%d id=%d actual fame delta=%d official=%d", r, id, got, want)
		if got != want {
			t.Errorf("MISSING EMBLEM FAME: grade=%d id=%d got=%d official=%d", r, id, got, want)
		}
	}
	if len(selected) != 5 {
		t.Fatalf("expected all 5 official emblem grades; found %v", selected)
	}
	// Count active sockets once after a save/read roundtrip; expired sockets do
	// not contribute, and removing all emblems restores the original total.
	saved, e := inventory.ReadSymbolSockets(base, layout)
	if e != nil {
		t.Fatal(e)
	}
	for _, group := range []string{"red_symbol", "multi_symbol"} {
		for i := range saved.Groups[group] {
			saved.Groups[group][i] = inventory.SymbolSocketItem{Template: selected[8]}
		}
	}
	encode := func() json.RawMessage {
		var state map[string]json.RawMessage
		if e := json.Unmarshal(base, &state); e != nil {
			t.Fatal(e)
		}
		state["symbol_emblem_sockets"], e = json.Marshal(saved)
		if e != nil {
			t.Fatal(e)
		}
		out, e := json.Marshal(state)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	checkDelta := func(want uint32) {
		state := encode()
		saved, e = inventory.ReadSymbolSockets(state, layout)
		if e != nil {
			t.Fatal(e)
		}
		after, e := svc.EquipmentFameBreakdown(encode())
		if e != nil {
			t.Fatal(e)
		}
		if got := after.Total - before.Total; got != want {
			t.Fatalf("roundtrip fame delta=%d want=%d", got, want)
		}
	}
	count := len(saved.Groups["red_symbol"]) + len(saved.Groups["multi_symbol"])
	checkDelta(uint32(count) * 80)
	saved.Groups["red_symbol"][0].ExpireTime = 1
	checkDelta(uint32(count-1) * 80)
	for group := range saved.Groups {
		for i := range saved.Groups[group] {
			saved.Groups[group][i] = inventory.SymbolSocketItem{}
		}
	}
	checkDelta(0)
	t.Logf("%d active sockets counted once; expiration, save/read, and removal passed", count)
	// Change the actual item's native field and re-import it. Runtime fame must
	// follow that field rather than a parallel Go grade-to-fame table.
	for _, entry := range entries {
		if entry.ID != selected[8] {
			continue
		}
		path := entry.Path
		if !strings.HasPrefix(path, "stackable/") {
			path = "stackable/" + path
		}
		path = catalog.ResolveScriptPath(a, path)
		script, e := catalog.ReadScript(a, path)
		if e != nil {
			t.Fatal(e)
		}
		body, e := a.ReadRaw(path)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for i, c := range script.Cells {
			if c.Type == 3 && c.Text == "[fame value]" && i+1 < len(script.Cells) {
				script.Cells[i+1].Value = 91
				binary.LittleEndian.PutUint32(body[(i+1)*5+1:], 91)
				found = true
				break
			}
		}
		if !found {
			t.Fatal("missing source fame field")
		}
		changed, e := character.NewFameItemImport(a)
		if e != nil {
			t.Fatal(e)
		}
		e = changed.Consume(catalog.ItemScript{Item: catalog.ItemIndexEntry{ID: entry.ID, Path: path, Kind: "stackable"}, Raw: body, Cells: script.Cells, Exact: true})
		if e != nil {
			t.Fatal(e)
		}
		f, e := changed.Finish()
		if e != nil {
			t.Fatal(e)
		}
		undo, e := character.InstallFameRules(f)
		if e != nil {
			t.Fatal(e)
		}
		defer undo()
		saved.Groups["red_symbol"][0] = inventory.SymbolSocketItem{Template: entry.ID}
		checkDelta(91)
		t.Log("modified native [fame value] drives actual character calculation")
	}
}
