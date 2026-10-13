package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
)

func TestBakalPostCharacterInfoNativeNotification(t *testing.T) {
	c := bakalPostCharacterTestClient()
	c.selectedCharacterID = c.worldState.role.ID
	p := c.bakalPostCharacterInfo()
	if len(p) != 1 || p[0].Kind != 0 || p[0].ID != 1220 || len(p[0].Payload) != 1 || p[0].Payload[0] != 0 {
		t.Fatalf("NOTI registry146D58EFC must route to timestamp handler146D5A3C0: %+v", p)
	}
	// Member/full-info refreshes must not restart the native3000ms gate.
	for _, mode := range []byte{1, 2} {
		q := binary.LittleEndian.AppendUint32([]byte{mode}, c.worldState.bakalRecruitment.ID)
		_, packets, err := c.bakalOwnedInfo(q)
		if err != nil || len(packets) != 1 || packets[0].Kind != 1 || packets[0].ID != 1353 {
			t.Fatalf("mode%d info query sent unrelated readiness notification: %+v %v", mode, packets, err)
		}
	}
}

func TestBakalPostCharacterInfoSelectionBoundary(t *testing.T) {
	c := bakalPostCharacterTestClient()
	c.selectedCharacterID = 0
	if len(c.bakalPostCharacterInfo()) != 0 {
		t.Fatal("unselected character initialized raid UI")
	}
	c.selectedCharacterID = c.worldState.role.ID + 1
	if len(c.bakalPostCharacterInfo()) != 0 {
		t.Fatal("stale role initialized raid UI")
	}
	c.selectedCharacterID = c.worldState.role.ID
	c.worldState.channelType = 81
	if len(c.bakalPostCharacterInfo()) != 0 {
		t.Fatal("unrelated channel received raid initialization")
	}
	c.worldState = nil
	if len(c.bakalPostCharacterInfo()) != 0 {
		t.Fatal("missing world received raid initialization")
	}
}

func bakalPostCharacterTestClient() *gameConnection {
	return &gameConnection{worldState: &worldSession{
		channelType:      legion.BakalChannelType,
		role:             database.Character{ID: 2},
		bakal:            &legion.BakalOpening{},
		bakalRecruitment: &protocol.RaidRecruitment{ID: 0x01520001, Leader: 1, LeaderName: "test", MemberCount: 1, MemberMax: 12, Create: protocol.RaidCreateRequest{Kind: 8, Title: "test"}},
	}}
}
