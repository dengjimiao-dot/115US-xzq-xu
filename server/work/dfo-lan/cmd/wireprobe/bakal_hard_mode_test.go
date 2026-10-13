package main

import (
	"bytes"
	"dfolan/internal/database"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"encoding/hex"
	"testing"
	"time"
)

func TestBakalHardCreateAndRetryKeepSourceMode(t *testing.T) {
	c, now := bakalNativeClient(t)
	w := c.worldState
	c.gatewayRuntime = &gatewayRuntime{}
	w.serverID = 1
	w.bakal = nil
	w.bakalRewards = &workflow.BakalRewardService{Rules: w.bakalRules}
	w.state.Position = database.WorldPosition{Town: 152, Area: 1}
	body, _ := hex.DecodeString("0802000000313100010007010000000000000000000000000000000000000000")
	if _, _, e := c.bakalCreateRaid(body, now); e != nil {
		t.Fatal(e)
	}
	if !w.bakal.HardMode() {
		t.Fatal("official hard create prepared normal phase")
	}
	start, _ := hex.DecodeString("88f25f0000000000feffffffff00010000000000000000000000000000000000")
	if _, _, e := c.bakalStartRaid(start, now); e != nil {
		t.Fatal(e)
	}
	w.bakal.Tick(now.Add(time.Duration(w.bakalRules.StartDelaySecs) * time.Second))
	want := legion.BakalOpenBuffCounts(w.bakalRules.HardPhase)
	if got := w.bakal.BuffInventorySnapshot().Body[:5]; !bytes.Equal(got, want[:]) {
		t.Fatalf("wrong hard inventory: %x want%x", got, want)
	}
	w.bakal.Tick(now.Add(time.Duration(w.bakalRules.PhaseTimeOverSecs+1) * time.Second))
	if w.bakal.Stage() != legion.BakalOpeningFailed {
		t.Fatal("source timeout did not fail")
	}
	if _, _, e := c.bakalStartRaid(start, now.Add(3*time.Hour)); e != nil {
		t.Fatal(e)
	}
	if !w.bakal.HardMode() {
		t.Fatal("failed hard run retried as normal")
	}
}
