package main

import (
	"encoding/hex"
	"testing"
)

func TestBakalLivingArenaPortalUsesSourcePermissionWithoutAuthoringDeath(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := bakalNativeBossGrid(t, w, "bakal")
	id := uint32(w.bakalRules.NormalPhase.EnterBakalDungeon)
	if _, e := w.bakalDungeonEntry(id, 0, &grid, now, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	if len(w.activeDungeon.LivingMonsters()) != 1 {
		t.Fatal("missing living owned boss")
	}
	// Official2070 record shape; destination comes from the source maze.
	p, _ := hex.DecodeString("4030bea80100000079509d4501000000000000000001000000ffffbc02fa00ffff640064000000000000000000000000")
	_, _, e := c.bakalScriptWarp(p)
	if e != nil {
		t.Fatal("source live-room portal refused", e)
	}
	if w.bakal.IsCleared(id) {
		t.Fatal("portal falsely cleared boss")
	}
	for _, dead := range w.activeDungeon.Dead {
		if dead {
			t.Fatal("portal authored a death")
		}
	}
	if w.activeDungeon.Definition.ID != id || w.activeDungeon.Loaded {
		t.Fatal("portal escaped source dungeon/loading boundary")
	}
}
