package main

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestBakalEvidenceCommandsRetainedAfterSampleLimit(t *testing.T) {
	for _, id := range []uint16{656, 657, 2089, 2069, 2070, 2073, 2074, 2261, 1134} {
		seen := map[uint16]int{id: BodySampleLimit}
		if !observedGameRequest(id) || !retainRequestBody(id, seen) || seen[id] != BodySampleLimit {
			t.Fatalf("Bakal command %d lost evidence after sampling cap", id)
		}
	}
	if retainRequestBody(2127, map[uint16]int{2127: BodySampleLimit}) {
		t.Fatal("unrelated telemetry lost its sampling cap")
	}
}

// Official 20261008 s20 C2170 asks for (2,1), N5213 enters (1,1).
// The arena comes from the current source SpecificXY, not opaque portal words.
func TestBakalFirstPhaseOfficialPortalEntersArena(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	p, _ := hex.DecodeString("0300000000000000ffffffffff4dedf505000000000200000001000000bb000000210100000500000005000000000000")
	if handled, _, err := c.enterBakalPortal(2062, p, at); err != nil || !handled {
		t.Fatalf("portal: handled=%v err=%v", handled, err)
	}
	grid := bakalNativeBossGrid(t, w, "bakal")
	if [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} != grid {
		t.Fatal("portal did not place player in the source Bakal arena")
	}
	_, plan, err := c.bakalLoadingDone(nil)
	if err != nil {
		t.Fatal(err)
	}
	next := w.activeDungeon
	creates := 0
	for _, packet := range plan {
		if packet.ID == 2194 {
			creates++
			if packet.Payload[1] != next.Room.X || packet.Payload[2] != next.Room.Y || binary.LittleEndian.Uint32(packet.Payload[7:]) != w.bakalRules.Monsters["bakal"].Template {
				t.Fatal("boss create does not belong to loaded player room")
			}
		}
	}
	if creates != 1 || len(next.LivingMonsters()) != 1 {
		t.Fatalf("arena creates=%d living=%d", creates, len(next.LivingMonsters()))
	}
	if extra, err := w.bakalLoadedBoss(); err != nil || len(extra) != 0 {
		t.Fatalf("boss duplicated after load: %v", err)
	}
}
