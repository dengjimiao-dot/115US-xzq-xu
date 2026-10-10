package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestBakalEntrySceneNativeSource(t *testing.T) {
	r, err := ImportBakalRaid(OpenNativeArchive(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.DungeonCatalog.CloseMapSource()
	scene, ok := r.EntryScenes[100007434]
	if !ok || scene.InitialMap != 100007529 || scene.BattleMap != 100007434 || scene.Source == "" || len(scene.SHA256) != 64 {
		t.Fatalf("native intro relation missing: %+v", r.EntryScenes)
	}
	sc, err := r.DungeonCatalog.MapScript(scene.InitialMap)
	if err != nil {
		t.Fatal(err)
	}
	// A changed source target changes the parsed relation; no529/434 runtime table.
	sc.Cells = append([]pvf.Token(nil), sc.Cells...)
	changed := false
	for i, c := range sc.Cells {
		if c.Type == 0 && c.Value == 100007434 {
			sc.Cells[i].Value = 100009999
			changed = true
		}
	}
	if !changed {
		t.Fatal("no battle map in source table")
	}
	got, ok, err := parseBakalEntryScene(scene.InitialMap, sc)
	if err != nil || !ok || got.BattleMap != 100009999 {
		t.Fatalf("parser ignored source change: %+v %v", got, err)
	}
}
