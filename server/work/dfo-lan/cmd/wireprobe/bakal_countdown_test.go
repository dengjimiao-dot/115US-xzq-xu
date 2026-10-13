package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestBakalCountdownDescriptorAndAdmission(t *testing.T) {
	for _, delay := range []int{3, 6} {
		c, now := bakalNativeClient(t)
		w := c.worldState
		w.bakalRules.StartDelaySecs = delay
		fresh, err := legion.PrepareBakalOpening(w.bakalRules, false)
		if err != nil {
			t.Fatal(err)
		}
		w.bakal = fresh
		w.bakalName = w.role.Name
		w.bakalRecruitment = &protocol.RaidRecruitment{ID: 0x01520001, Create: protocol.RaidCreateRequest{Kind: 8, Title: "countdown"}, Leader: w.role.WireID, LeaderName: w.role.Name, MemberCount: 1, MemberMax: uint32(w.bakalRules.RaidMemberMax), MemberPosition: 1}
		start, _ := hex.DecodeString("88f25f0000000000feffffffff00010000000000000000000000000000000000")
		_, packets, err := c.bakalStartRaid(start, now)
		if err != nil {
			t.Fatal(err)
		}
		descriptor, countdown := -1, -1
		for i, p := range packets {
			if p.ID == 578 && binary.LittleEndian.Uint32(p.Payload[4:]) == 2 {
				descriptor = i
				if p.Payload[17+len(w.bakalRecruitment.Create.Title)] != 1 {
					t.Fatal("countdown received idle raid descriptor")
				}
			}
			if p.ID == 581 {
				countdown = i
				if p.Kind != 0 || len(p.Payload) != 0 {
					t.Fatal("native N581 must be an empty notification")
				}
			}
		}
		if descriptor < 0 || countdown <= descriptor {
			t.Fatalf("owned descriptor must precede countdown: %d %d", descriptor, countdown)
		}
		id := uint32(w.bakalRules.NormalPhase.EnterBakalDungeon)
		deadline := now.Add(time.Duration(delay) * time.Second)
		for _, elapsed := range []time.Duration{0, time.Duration(delay)*time.Second - time.Nanosecond} {
			at := now.Add(elapsed)
			w.bakal.Tick(at)
			if w.bakal.ValidateDungeonEntry(id, at) == nil {
				t.Fatal("countdown bypassed domain admission")
			}
			if plan, err := w.bakalDungeonEntry(id, 0, nil, at, false); err == nil || len(plan) != 0 || w.activeDungeon != nil {
				t.Fatal("preparing request constructed a raid room")
			}
		}
		generic := make([]byte, 32)
		binary.LittleEndian.PutUint32(generic, id)
		if pending, plan, err := w.selectDungeon(generic); err == nil || pending != nil || len(plan) != 0 {
			t.Fatal("generic C16 bypassed native raid admission")
		}
		w.bakal.Tick(deadline)
		if err := w.bakal.ValidateDungeonEntry(id, deadline); err != nil {
			t.Fatalf("source deadline did not open raid: %v", err)
		}
	}
}
