package protocol

import (
	"encoding/hex"
	"testing"
)

func TestBakalDifficultyOfficialCreateVectors(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		hard bool
	}{
		{"0802000000313100010007010000000000000000000000000000000000000000", true},
		{"08010000003100000007000000000000", false},
	} {
		b, _ := hex.DecodeString(tc.raw)
		r, e := DecodeRaidCreateRequest(b)
		if e != nil {
			t.Fatal(e)
		}
		h, e := BakalHardMode(r)
		if e != nil || h != tc.hard {
			t.Fatalf("%+v %v", r, e)
		}
	}
	for _, flags := range [][2]byte{{1, 0}, {0, 1}, {2, 2}} {
		r := RaidCreateRequest{}
		r.Options[1], r.Options[4] = flags[0], flags[1]
		if _, e := BakalHardMode(r); e == nil {
			t.Fatal("inconsistent flags accepted")
		}
	}
}
