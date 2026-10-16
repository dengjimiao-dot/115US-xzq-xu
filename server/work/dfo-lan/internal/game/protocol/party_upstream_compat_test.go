package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSoloPartyUpstreamRaidProjectionUnchanged(t *testing.T) {
	for _, actor := range []uint16{1, 8, 4096, 65534} {
		want := make([]byte, 99)
		binary.LittleEndian.PutUint16(want, 1)
		binary.LittleEndian.PutUint16(want[4:], 1)
		want[15], want[61] = 1, 1
		binary.LittleEndian.PutUint16(want[63:], actor)
		got, err := SoloPartyInfo(actor)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("upstream roster changed for actor %d: %v", actor, err)
		}
	}
	for _, actor := range []uint16{0, 65535} {
		if _, err := SoloPartyInfo(actor); err == nil {
			t.Fatal("invalid upstream actor accepted")
		}
	}
}
