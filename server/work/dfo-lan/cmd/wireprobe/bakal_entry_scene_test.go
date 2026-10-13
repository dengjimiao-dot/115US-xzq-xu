package main

import (
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
	"time"
)

func TestBakalFirstEntrySceneAndReentry(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	grid := [2]byte{1, 1}
	if !w.bakal.FirstBakalEntry() {
		t.Fatal("source first entry latch missing")
	}
	plan, err := w.bakalDungeonEntry(100003149, 0, &grid, now, false)
	if err != nil {
		t.Fatal(err)
	}
	s := w.activeDungeon
	scene, ok := w.bakalRules.EntryScenes[s.Room.Map]
	if !ok || s.EntrySceneMap != scene.InitialMap || s.Room.Map != scene.BattleMap {
		t.Fatal("entry scene lost logical battle room")
	}
	maps := 0
	mapAt, clearAt := -1, -1
	for i, p := range plan {
		if p.ID == 570 && len(p.Payload) == 9 && binary.LittleEndian.Uint32(p.Payload[1:5]) == w.bakalRules.Symbols["[IS BAKAL FIRST ENTER]"] && binary.LittleEndian.Uint32(p.Payload[5:9]) == 0 {
			if clearAt == -1 {
				clearAt = i
			}
		}
		if p.ID == 29 {
			maps++
			mapAt = i
			if len(p.Payload) < 36 || binary.LittleEndian.Uint32(p.Payload[32:]) != scene.InitialMap || p.Payload[0] != 1 || p.Payload[1] != 1 {
				t.Fatal("N29 did not use source intro map at central grid")
			}
		}
	}
	if clearAt <= mapAt || mapAt < 0 {
		t.Fatalf("first-enter symbol must follow intro N29: map%d clear%d", mapAt, clearAt)
	}
	if maps != 1 || w.bakal.FirstBakalEntry() {
		t.Fatal("first-entry latch was not consumed exactly once")
	}
	if _, _, err = c.bakalLoadingDone(nil); err != nil {
		t.Fatal(err)
	}
	report := make([]byte, 184)
	binary.LittleEndian.PutUint32(report[17:], s.Definition.ID)
	for _, id := range []uint32{scene.InitialMap, scene.BattleMap} {
		binary.LittleEndian.PutUint32(report[21:], id)
		if _, _, err := c.bakalBattleReport(report, now); err != nil {
			t.Fatalf("owned phase scene rejected: %v", err)
		}
	}
	binary.LittleEndian.PutUint32(report[21:], scene.BattleMap+1)
	if _, _, err := c.bakalBattleReport(report, now); err == nil {
		t.Fatal("foreign room report accepted")
	}
	if _, err := w.bakal.CampReturnAt(now, legion.BakalReturnVoluntary); err != nil {
		t.Fatal(err)
	}
	w.bakal.AcknowledgeCampReturn()
	w.activeDungeon = nil
	at := now.Add(time.Duration(w.bakalRules.RevivalTimes[0]+1) * time.Second)
	w.bakal.Tick(at)
	plan, err = w.bakalDungeonEntry(100003149, 0, &grid, at, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range plan {
		if p.ID == 29 && binary.LittleEndian.Uint32(p.Payload[32:]) != scene.BattleMap {
			t.Fatal("reentry replayed initial scene")
		}
	}
	if w.activeDungeon.EntrySceneMap != 0 {
		t.Fatal("reentry retained intro presentation")
	}
}
