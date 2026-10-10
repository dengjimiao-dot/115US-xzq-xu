package legion

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestBakalSettlementReportsOwnedDurationRatherThanGolden610(t *testing.T) {
	for _, seconds := range []uint32{45, 612, 1800} {
		o, now := runtimeNative(t, false)
		o.stage = BakalOpeningFinal
		o.settleAt = now.Add(time.Duration(seconds) * time.Second)
		o.SetRewardInventory([]byte{})
		o.SetRewardPublished(true)
		frames := o.Tick(o.settleAt.Add(time.Second))
		found := false
		for _, f := range frames {
			if f.ID == 588 {
				found = true
				if len(f.Body) != 10 || f.Body[0] != 2 || f.Body[1] != 0 || f.Body[8] != 1 || f.Body[9] != 1 {
					t.Fatal("native final-result variant/claimed flag missing")
				}
				if binary.LittleEndian.Uint32(f.Body[2:]) != seconds {
					t.Fatal("result reused sample duration or counted reward publication latency")
				}
			}
		}
		if !found {
			t.Fatal("no settlement result")
		}
	}
}

func TestBakalFinalResultWaitsForDurableInventoryAndPublishesCompletedState(t *testing.T) {
	for _, published := range []bool{false, true} {
		o, now := runtimeNative(t, false)
		o.stage = BakalOpeningFinal
		o.settleAt = now.Add(time.Minute)
		o.SetRewardPublished(published)
		for _, f := range o.Tick(o.settleAt) {
			if f.ID == 588 {
				t.Fatal("result bypassed durable inventory readiness")
			}
		}
		o.SetRewardInventory([]byte{})
		frames := o.Tick(o.settleAt)
		inventory, result, state := -1, -1, -1
		for i, f := range frames {
			switch f.ID {
			case 13:
				inventory = i
			case 588:
				result = i
				want := byte(0)
				if published {
					want = 1
				}
				if f.Body[0] != 2 || f.Body[8] != want || binary.LittleEndian.Uint16(f.Body[6:]) != 0 {
					t.Fatal("final result fabricated reward or unresolved metric")
				}
			case 574:
				if len(f.Body) == 2 && f.Body[0] == 3 && f.Body[1] == 0 {
					state = i
				}
			}
		}
		if inventory < 0 || result <= inventory || state <= result {
			t.Fatal("native inventory/result/completed-state ordering missing")
		}
		for _, f := range o.Tick(o.settleAt.Add(time.Second)) {
			if f.ID == 588 {
				t.Fatal("repeated tick replayed clear result")
			}
		}
	}
}
