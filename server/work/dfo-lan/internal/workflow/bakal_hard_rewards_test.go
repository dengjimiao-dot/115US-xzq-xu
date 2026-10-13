package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"testing"
	"time"
)

func TestBakalHardRewardsPersistSeparatePoolAndCards(t *testing.T) {
	role := database.Character{ID: 7, AccountID: 2, Name: "test", ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{}`)}
	s, _ := bakalTestService(t, role)
	s.Store = &biddingEventFake{role: role, receipts: map[string]json.RawMessage{}}
	s.Rules.HardRewards = []catalog.BakalRewardEntry{{Category: "party_card", Template: 10418036, Amount: 5, Weight: 1, Difficulty: 1}, {Category: "squad_item", Template: 10418036, Amount: 7, Weight: 1, Difficulty: 1}}
	ctx := context.Background()
	at := time.Unix(1791824700, 0)
	p, saved, e := s.FreezeDifficulty(ctx, role, "hard-run", 3, true, at)
	if e != nil || !p.Hard || !p.Eligible || p.Cards["party_card"][0].Amount != 5 || p.Cards["squad_item"][0].Amount != 7 {
		t.Fatalf("wrong hard plan: %+v %v", p, e)
	}
	if _, _, e := s.FreezeDifficulty(ctx, saved, "hard-run", 3, false, at); e == nil {
		t.Fatal("changed mode reused frozen receipt")
	}
	_, saved, e = s.Claim(ctx, saved, "hard-run", at)
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(saved.State)
	if e != nil || len(bag.Items) != 1 || bag.Items[0].Amount != 12 {
		t.Fatalf("hard products not paid once: %+v %v", bag, e)
	}
	_, again, e := s.Claim(ctx, saved, "hard-run", at)
	if e != nil {
		t.Fatal(e)
	}
	bag, e = inventory.ReadBag(again.State)
	if e != nil || bag.Items[0].Amount != 12 {
		t.Fatal("retry paid twice")
	}
	p, _, e = s.Freeze(ctx, again, "normal-run", 3, at)
	if e != nil || p.Hard || p.Cards["party_card"][0].Amount != 1 {
		t.Fatalf("normal pool changed: %+v %v", p, e)
	}
}
