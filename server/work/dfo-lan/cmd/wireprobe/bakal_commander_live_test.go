package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestBakalCommanderLiveBodyTargetsPublishedPartyAndAcknowledges(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := bakalNativeBossGrid(t, w, "basilisk")
	if _, e := w.bakalDungeonEntry(100003157, 0, &grid, now, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	w.bakalRecruitment = &protocol.RaidRecruitment{MemberPosition: 1}
	// Official s9 C686/N1860/A1863: a32B request, slot1, target party1.
	body, _ := hex.DecodeString("0000803f00000000000000000001000000000000000000000000000000000000")
	before := w.bakal.BuffInventorySnapshot().Body[1]
	_, plan, e := c.handleBakalRequest(2072, body, now)
	if e != nil {
		t.Fatal(e)
	}
	stock, presentation, effect, ack := -1, -1, -1, -1
	for i, p := range plan {
		if p.ID == 2288 {
			stock = i
			if p.Payload[1] != before-1 || binary.LittleEndian.Uint32(p.Payload[9:]) != uint32(legion.BakalOwnedPartyID) {
				t.Fatal("incorrect charge/activation party")
			}
		}
		if p.ID == 2289 {
			presentation = i
			if p.Kind != 0 || len(p.Payload) != 18 || p.Payload[0] != 2 || p.Payload[1] != legion.BakalOwnedPartyID || binary.LittleEndian.Uint32(p.Payload[2:]) != 1 {
				t.Fatal("NPC cut-in did not target the successful source commander")
			}
		}
		if p.ID == 2285 {
			effect = i
		}
		if p.ID == 2072 && p.Kind == 1 {
			ack = i
			if len(p.Payload) != 1 || p.Payload[0] != 1 {
				t.Fatal("incorrect success ACK")
			}
		}
	}
	if stock < 0 || presentation <= stock || effect <= presentation || ack <= effect {
		t.Fatal("missing native inventory/presentation/effect/ACK sequence")
	}
	if _, _, e = c.handleBakalRequest(2072, body, now); e == nil {
		t.Fatal("cooldown duplicate accepted")
	}
	if w.bakal.BuffInventorySnapshot().Body[1] != before-1 {
		t.Fatal("duplicate spent stock")
	}
}

func TestBakalCommanderPresentationAllSourceSlots(t *testing.T) {
	for slot := byte(0); slot < 5; slot++ {
		c, now := bakalNativeClient(t)
		w := c.worldState
		grid := bakalNativeBossGrid(t, w, "basilisk")
		if _, err := w.bakalDungeonEntry(100003157, 0, &grid, now, false); err != nil {
			t.Fatal(err)
		}
		if _, _, err := c.bakalLoadingDone(nil); err != nil {
			t.Fatal(err)
		}
		w.bakalRecruitment = &protocol.RaidRecruitment{MemberPosition: 1}
		body := make([]byte, 32)
		binary.LittleEndian.PutUint32(body[13:], uint32(slot))
		_, plan, err := c.handleBakalRequest(2072, body, now)
		if err != nil {
			t.Fatalf("slot%d: %v", slot, err)
		}
		count := 0
		for _, p := range plan {
			if p.ID == 2289 {
				count++
				if len(p.Payload) != 18 || p.Payload[1] != 1 || binary.LittleEndian.Uint32(p.Payload[2:]) != uint32(slot) {
					t.Fatalf("wrong NPC for slot%d", slot)
				}
			}
		}
		if count != 1 {
			t.Fatalf("slot%d sent%d presentations", slot, count)
		}
		if _, plan, err := c.handleBakalRequest(2072, body, now); err == nil || len(plan) != 0 {
			t.Fatal("rejected repeat replayed presentation")
		}
	}
}
