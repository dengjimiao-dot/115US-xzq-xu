package catalog

import "fmt"

// BakalEntryScene links a battle room to its native initial presentation.
// Client ACT completes PHASE SHIFT MAP without another server room entry.
type BakalEntryScene struct {
	Source, SHA256        string
	InitialMap, BattleMap uint32
}

func parseBakalEntryScene(id uint32, s ScriptRecord) (BakalEntryScene, bool, error) {
	var result BakalEntryScene
	blocks, err := bakalBlocks(s.Cells, "[phase shift map]", "[/phase shift map]")
	if err != nil {
		return result, false, err
	}
	if len(blocks) == 0 {
		return result, false, nil
	}
	if len(blocks) != 1 {
		return result, false, fmt.Errorf("ambiguous phase-map source %s", s.Path)
	}
	phases, err := bakalBlocks(blocks[0], "[phase]", "[/phase]")
	if err != nil {
		return result, false, err
	}
	initial := sectionCells(blocks[0], "[init phase]")
	immediate := sectionCells(blocks[0], "[init immediatly phase]")
	// Only project the witnessed two-phase initial-to-battle contract.
	if len(phases) != 2 || len(initial) != 1 || initial[0].Type != 0 || initial[0].Value != 0 || len(immediate) != 2 || immediate[0].Type != 0 || immediate[1].Type != 0 || immediate[0].Value != 0 || immediate[1].Value != 1 {
		return result, false, nil
	}
	maps := make([]uint32, 2)
	for i, phase := range phases {
		values := sectionCells(phase, "[map index]")
		if len(values) != 1 || values[0].Type != 0 || values[0].Value <= 0 {
			return result, false, fmt.Errorf("invalid phase map index in %s", s.Path)
		}
		maps[i] = uint32(values[0].Value)
	}
	if maps[0] != id || maps[0] == maps[1] {
		return result, false, nil
	}
	return BakalEntryScene{Source: s.Path, SHA256: s.SHA256, InitialMap: maps[0], BattleMap: maps[1]}, true, nil
}

func loadBakalEntryScenes(r *BakalRaidRules) error {
	r.EntryScenes = map[uint32]BakalEntryScene{}
	dc := r.DungeonCatalog
	for _, slot := range r.Slots {
		if slot.Type != "bakal" {
			continue
		}
		d, ok := dc.Dungeons[slot.Dungeon]
		if !ok {
			return fmt.Errorf("Bakal entry scene has no source dungeon")
		}
		starts := map[uint32]bool{}
		for _, maze := range d.Mazes {
			for _, room := range maze.Rooms {
				if [2]byte{room.X, room.Y} == maze.Start {
					starts[room.Map] = true
				}
			}
		}
		for _, id := range slot.Maps {
			sc, err := dc.MapScript(id)
			if err != nil {
				return err
			}
			scene, ok, err := parseBakalEntryScene(id, sc)
			if err != nil {
				return err
			}
			if !ok || !starts[scene.BattleMap] {
				continue
			}
			if _, owned := r.SlotForMap(slot.Dungeon, scene.BattleMap); !owned {
				return fmt.Errorf("Bakal phase scene targets an unowned map")
			}
			if old, exists := r.EntryScenes[scene.BattleMap]; exists && old.InitialMap != scene.InitialMap {
				return fmt.Errorf("ambiguous Bakal initial scene")
			}
			r.EntryScenes[scene.BattleMap] = scene
		}
	}
	return nil
}
