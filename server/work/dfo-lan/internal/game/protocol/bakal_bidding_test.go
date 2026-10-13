package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestBakalBiddingOfficialHardVectors(t *testing.T) {
	b, e := BakalBiddingParticipants([]BakalBiddingParticipant{{Actor: 41626, Eligible: true}})
	w, _ := hex.DecodeString("010000009aa201")
	if e != nil || !bytes.Equal(b, w) {
		t.Fatalf("participants %x %v", b, e)
	}
	lots := []BakalBiddingItem{{Template: 10341704, Amount: 1, Flags: [3]byte{0, 0, 1}}, {Template: 10354120, Amount: 1}, {Template: 10354120, Amount: 1}}
	b, e = BakalBiddingItems(lots)
	w, _ = hex.DecodeString("0300000048cd9d00010000000000010000c8fd9d00010000000000000000c8fd9d00010000000000000000")
	if e != nil || !bytes.Equal(b, w) {
		t.Fatalf("lots %x %v", b, e)
	}
	b, e = BakalBiddingStart(10341704, 1, 1000000)
	w, _ = hex.DecodeString("48cd9d000100000040420f00")
	if e != nil || !bytes.Equal(b, w) {
		t.Fatalf("start %x %v", b, e)
	}
	for _, tc := range []struct {
		lot  byte
		item uint32
		hex  string
	}{{0, 10341704, "010000000000000048cd9d000100000040420f0002401f0000"}, {1, 10354120, "0100000000000001c8fd9d000100000040420f0002401f0000"}, {2, 10354120, "0100000000000002c8fd9d000100000040420f0002401f0000"}} {
		b, e = BakalBiddingResult(0, "", tc.lot, tc.item, 1, 1000000, 2, 8000)
		w, _ = hex.DecodeString(tc.hex)
		if e != nil || !bytes.Equal(b, w) {
			t.Fatalf("terminal %x %v", b, e)
		}
	}
	bid, _ := hex.DecodeString("0164270000000000")
	if n, e := DecodeBakalBid(bid); e != nil || n != 10084 {
		t.Fatalf("native bid %d %v", n, e)
	}
	bid[7] = 1
	if _, e := DecodeBakalBid(bid); e == nil {
		t.Fatal("nonzero padding accepted")
	}
	live, _ := hex.DecodeString("0010c07a120000000000000000000000")
	if n, e := DecodeBakalBid(live); e != nil || n != 310034448 {
		t.Fatalf("confirmed live bid rejected: %d %v", n, e)
	}
	live[0] = 2
	if _, e := DecodeBakalBid(live); e == nil {
		t.Fatal("unknown action accepted")
	}
	live[0] = 0
	for i := 1; i < 5; i++ {
		live[i] = 0
	}
	if _, e := DecodeBakalBid(live); e == nil {
		t.Fatal("zero-value action misinterpreted as a bid")
	}
}
