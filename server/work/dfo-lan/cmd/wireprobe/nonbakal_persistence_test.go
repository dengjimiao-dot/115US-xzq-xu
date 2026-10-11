package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/world"
	"encoding/binary"
	"testing"
)

type raidRoomWorldStore struct {
	saved   database.WorldState
	loads   int
	typ     uint32
	initial database.WorldPosition
}

func (s *raidRoomWorldStore) LoadWorld(_ context.Context, _, _ int64, typ uint32, initial database.WorldPosition, version string) (database.WorldState, error) {
	s.loads++
	s.typ, s.initial = typ, initial
	state := s.saved
	if state.ConfigVersion == "" {
		state.ConfigVersion = version
	}
	return state, nil
}

func raidRoomWorld(entry catalog.RaidEntrance, store *raidRoomWorldStore) *worldSession {
	areas := map[string]catalog.WorldArea{}
	for _, room := range []catalog.TownArea{*entry.Recruiting, entry.Waiting} {
		areas[catalog.AreaKey(room.TownID, room.AreaID)] = catalog.WorldArea{Town: room.TownID, Area: room.AreaID, MinimumLevel: room.MinimumLevel, Walkable: room.Walkable}
	}
	return &worldSession{channelType: 112, channelWorldIsolated: true, nonBakalEntrance: &entry, account: 1,
		service: &world.Service{Store: store, Catalog: catalog.WorldCatalog{Source: entry.Recruiting.Source, Areas: areas}}}
}

func raidRoomFixture() catalog.RaidEntrance {
	return catalog.RaidEntrance{MemberMax: 12, StartMinimum: 1, PartyMax: 3,
		Recruiting: &catalog.TownArea{Source: pvf.ArchiveSnapshot{Checksum: "room-fixture"}, TownID: 701, AreaID: 7, MinimumLevel: 115, Walkable: [][4]int32{{100, 100, 400, 200}}},
		Waiting:    catalog.TownArea{TownID: 701, AreaID: 9, MinimumLevel: 115, Walkable: [][4]int32{{200, 100, 400, 200}}}}
}

func raidRoomRole() database.Character {
	return database.Character{ID: 1, WireID: 9, Profession: 16, Name: "owner", State: []byte(`{"level":115,"advancement":3,"awakening":3}`)}
}

func roomAreaRequest(from, to database.WorldPosition) []byte {
	p := binary.LittleEndian.AppendUint32(nil, to.Town)
	p = binary.LittleEndian.AppendUint32(p, to.Area)
	p = binary.LittleEndian.AppendUint16(p, to.X)
	p = binary.LittleEndian.AppendUint16(p, to.Y)
	p = append(p, 0)
	p = binary.LittleEndian.AppendUint32(p, from.Town)
	p = binary.LittleEndian.AppendUint16(p, uint16(from.Area))
	return append(p, 0, 0)
}

func TestRaidPreparationRestoresSourceInsteadOfOrphanSavedRoom(t *testing.T) {
	entry := raidRoomFixture()
	for _, old := range []database.WorldPosition{
		{Town: entry.Waiting.TownID, Area: entry.Waiting.AreaID, X: 400, Y: 200},
		{Town: 9999, Area: 9999, X: 65535, Return: &database.WorldReturn{Town: 3, Area: 4}},
	} {
		store := &raidRoomWorldStore{saved: database.WorldState{Position: old, Revision: 53}}
		w := raidRoomWorld(entry, store)
		w.nonBakalWaiting = true
		if err := w.enter(raidRoomRole(), database.WorldPosition{Town: 3, Area: 4}); err != nil {
			t.Fatal(err)
		}
		x, y := entry.Recruiting.Spawn()
		want := database.WorldPosition{Town: entry.Recruiting.TownID, Area: entry.Recruiting.AreaID, X: x, Y: y}
		if w.state.Position != want || w.nonBakalWaiting || w.state.Revision != 53 || store.typ != 112 || store.initial != want || store.saved.Position != old {
			t.Fatalf("fresh preparation polluted saved state: position=%+v saved=%+v", w.state, store.saved)
		}
		// A real CMD35 goes through validation and the persistence branch. The
		// SQL store is deliberately nil: writing it would panic this regression.
		body := binary.LittleEndian.AppendUint16(nil, x+1)
		body = binary.LittleEndian.AppendUint16(body, y)
		body = append(body, 0, 0, 0)
		if err := w.handle(35, body, func(byte, uint16, []byte) error { return nil }, func(map[string]any) {}); err != nil {
			t.Fatal(err)
		}
		if w.state.Position.X != x+1 || w.state.Revision != 53 || store.loads != 1 || store.saved.Position != old {
			t.Fatal("room movement persisted or reverted the session position")
		}
	}
	store := &raidRoomWorldStore{saved: database.WorldState{ConfigVersion: "incompatible-save"}}
	w := raidRoomWorld(entry, store)
	if err := w.enter(raidRoomRole(), database.WorldPosition{}); err == nil {
		t.Fatal("transient room entry bypassed save identity validation")
	}
}
