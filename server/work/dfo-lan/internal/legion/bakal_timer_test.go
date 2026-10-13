package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestBakalTimerOfficialStartAndCancellation(t *testing.T) {
	tests := []struct {
		id, sub, expiry, seconds uint32
		hex                      string
	}{
		{9, 1, 1791558627, 10, "0900000001000000e303c96a0a000000"},
		{28, 56, 1791559920, 180, "1c00000038000000f008c96ab4000000"},
		{28, 24, 0, 0, "1c000000180000000000000000000000"},
	}
	for _, v := range tests {
		p, e := BakalTimerFrame(v.id, v.sub, time.Unix(int64(v.expiry), 0), v.seconds)
		if e != nil || hex.EncodeToString(p) != v.hex {
			t.Fatalf("timer %d/%d: %x %v", v.id, v.sub, p, e)
		}
	}
	if _, e := BakalTimerFrame(1, 1, time.Unix(1<<32, 0), 1); e == nil {
		t.Fatal("overflow accepted")
	}
}
func TestBakalSourceTimerPublishesChangedDuration(t *testing.T) {
	o, now := runtimeNative(t, false)
	o.eventFrames()
	timer := o.script.InitTimers[0]
	duration := int32(timer.Secs + 7)
	ins := catalog.BakalScriptInstruction{Op: "[SET TIMER]", Args: []pvf.Token{{Type: 0, Value: int32(timer.ID)}, {Type: 0, Value: int32(timer.Sub)}, {Type: 0, Value: duration}}}
	signals := []bakalSignal{}
	if e := o.executeEvent(ins, now, &signals); e != nil {
		t.Fatal(e)
	}
	frames := o.eventFrames()
	if len(frames) != 1 || frames[0].ID != 583 {
		t.Fatal("source timer not published")
	}
	p := frames[0].Body
	if binary.LittleEndian.Uint32(p[12:]) != uint32(duration) || binary.LittleEndian.Uint32(p[8:]) != uint32(now.Unix()+int64(duration)) {
		t.Fatal("notification did not follow source duration")
	}
	if !o.runtime.timers[[2]uint32{uint32(timer.ID), uint32(timer.Sub)}].Equal(now.Add(time.Duration(duration) * time.Second)) {
		t.Fatal("wire and server timer diverged")
	}
}
