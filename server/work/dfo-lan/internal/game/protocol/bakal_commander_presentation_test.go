package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestBakalCommanderPresentationOfficialVectors(t *testing.T) {
	// Complete20261009 s9: S1352,1438,2270,2276,2537. The native reader
	// consumes18 bytes; do not replay encrypted padding from these frames.
	for slot, raw := range []string{
		"020100000000000000000000000000000000",
		"020101000000000000000000000000000000",
		"020102000000000000000000000000000000",
		"020103000000000000000000000000000000",
		"020104000000000000000000000000000000",
	} {
		want, _ := hex.DecodeString(raw)
		got, err := BakalCommanderPresentation(byte(slot), 1)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("slot%d: %x %v", slot, got, err)
		}
	}
	for _, tc := range [][2]byte{{5, 1}, {255, 1}, {0, 0}, {0, 4}, {0, 255}} {
		if _, err := BakalCommanderPresentation(tc[0], tc[1]); err == nil {
			t.Fatalf("invalid slot/team accepted: %v", tc)
		}
	}
}
