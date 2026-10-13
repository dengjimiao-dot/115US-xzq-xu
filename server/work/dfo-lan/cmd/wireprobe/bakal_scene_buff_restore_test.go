package main

import (
	"bytes"
	"dfolan/internal/legion"
	"testing"
	"time"
)

func TestBakalSceneLoadRestoresEffectsWithoutRenewingDeadlines(t *testing.T) {
	c, _ := bakalNativeClient(t)
	w := c.worldState
	now := time.Now()
	// The native normal phase owns the stock; this existing commander effect
	// has a finite source duration and must survive the scene load unchanged.
	if _, err := w.bakal.UseRaidBuffAt(3, now); err != nil {
		t.Fatal(err)
	}
	grid := bakalNativeBossGrid(t, w, "bakal")
	id := uint32(w.bakalRules.NormalPhase.EnterBakalDungeon)
	if _, err := w.bakalDungeonEntry(id, 0, &grid, now, false); err != nil {
		t.Fatal(err)
	}
	before := w.bakal.PartyFrame(w.bakalLocation, now)
	if bytes.Equal(before[13:21], []byte{25, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatal("source effect not active in fixture")
	}
	_, plan, err := c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	restores := 0
	for _, p := range plan {
		if p.Name == "bakal_party_effects_restored" {
			restores++
			if p.ID != legion.NotiBakalPartyLocation || !bytes.Equal(p.Payload, before) {
				t.Fatal("scene changed raid effect identity/deadline/budget")
			}
		}
	}
	if restores != 1 {
		t.Fatalf("effect restoration count=%d", restores)
	}
	_, repeated, err := c.bakalLoadingDone(nil)
	if err != nil || len(repeated) != 1 || repeated[0].Kind != 1 {
		t.Fatalf("duplicate loading repeated effects: %v", err)
	}
	if after := w.bakal.PartyFrame(w.bakalLocation, now); !bytes.Equal(after, before) {
		t.Fatal("loading altered the owned source state")
	}
}
