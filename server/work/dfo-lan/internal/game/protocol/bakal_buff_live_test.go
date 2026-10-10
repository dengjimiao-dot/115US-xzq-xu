package protocol

import (
	"encoding/hex"
	"testing"
)

func TestBakalCommanderOfficialAlignedBodies(t *testing.T) {
	vectors := []string{
		"0000803f00000000000000000000000000000000000000000000000000000000",
		"0000803f00000000000000000001000000000000000000000000000000000000",
		"0000803f00000000000000000002000000000000000000000000000000000000",
		"0000803f00000000000000000003000000000000000000000000000000000000",
		"00000000000000005ea6eb460104000000000000000000000000000000000000",
	}
	for want, s := range vectors {
		p, e := hex.DecodeString(s)
		if e != nil {
			t.Fatal(e)
		}
		got, e := DecodeBakalCommanderBuff(p)
		if e != nil || got != want {
			t.Fatalf("slot%d: got%d err%v", want, got, e)
		}
		p[31] = 1
		if _, e = DecodeBakalCommanderBuff(p); e == nil {
			t.Fatal("nonzero cipher padding accepted")
		}
	}
}
