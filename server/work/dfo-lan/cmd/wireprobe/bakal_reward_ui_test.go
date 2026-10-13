package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"testing"
	"time"
)

func TestBakalNativeFrozenRewardPresentation(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	w.bakalRecruitment = &protocol.RaidRecruitment{ID: 1, Leader: w.role.WireID, LeaderName: w.role.Name, MemberCount: 1, MemberMax: 12, Create: protocol.RaidCreateRequest{Kind: 8, Title: "test"}}
	w.loot = &loot.Service{Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{10418036: {ID: 10418036, Kind: "stackable"}}}}
	p := workflow.BakalRewardPlan{Eligible: true, Cards: map[string][]loot.Award{
		"squad_item": {{Template: 10418036, Amount: 17}}, "party_card": {{Template: 10418036, Amount: 3}},
	}}
	ui, err := w.prepareBakalRewardUI(p, now)
	if err != nil || ui == nil {
		t.Fatalf("prepare %v %v", ui, err)
	}
	// Change the source after the persisted draw: presentation must not redraw.
	p.Cards["squad_item"][0].Amount = 999
	if f := ui.advance(ui.readyAt.Add(-time.Nanosecond), w.bakalRules); len(f) != 0 {
		t.Fatal("reward page before movie deadline")
	}
	f := ui.advance(ui.readyAt, w.bakalRules)
	if len(f) != 8 || f[0].ID != 574 || !bytes.Equal(f[0].Body, []byte{4, 0}) || f[2].ID != 587 || f[2].Body[0] != 0 || f[3].Body[0] != 3 || f[4].Body[0] != 6 || f[6].Body[0] != 7 {
		t.Fatalf("reward order %+v", f)
	}
	if len(f[2].Body) != 58 || f[2].Body[26] != 17 {
		t.Fatalf("frozen material changed %x", f[2].Body)
	}
	for _, i := range []int{1, 5, 7} {
		if f[i].ID != 578 || binary.LittleEndian.Uint32(f[i].Body[4:]) != 2 {
			t.Fatal("missing owned state cache")
		}
	}
	w.bakalRecruitment.State = 7
	w.bakalRewardUI = ui
	if len(ui.advance(ui.readyAt.Add(-time.Nanosecond), w.bakalRules)) != 0 {
		t.Fatal("bidding wait too soon")
	}
	f = ui.advance(ui.readyAt, w.bakalRules)
	if len(f) != 0 {
		t.Fatal("auction opened without frozen lot table")
	}
	if len(ui.advance(ui.readyAt.Add(time.Hour), w.bakalRules)) != 0 {
		t.Fatal("repeated UI publication")
	}
}

func TestBakalFrozenRewardRejectsUnboundPreview(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	w.loot = &loot.Service{Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{1: {ID: 1, Kind: "equipment"}}}}
	p := workflow.BakalRewardPlan{Eligible: true, Cards: map[string][]loot.Award{"squad_item": {{Template: 1, Amount: 1}}, "party_card": {}}}
	if _, err := w.prepareBakalRewardUI(p, now); err == nil {
		t.Fatal("invented equipment preview")
	}
	if w.bakalRewardUI != nil {
		t.Fatal("invalid preview published")
	}
}
