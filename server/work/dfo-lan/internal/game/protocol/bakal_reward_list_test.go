package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestBakalRewardListNativeHardVectors(t *testing.T) {
	var rows []struct {
		Branch byte                  `json:"branch"`
		Actor  uint16                `json:"actor"`
		Name   string                `json:"name"`
		Card   int8                  `json:"card_index"`
		Items  []BakalRewardMaterial `json:"materials"`
		Hex    string                `json:"logical_hex"`
	}
	b, e := os.ReadFile("testdata/bakal_hard_rewards_20261013.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &rows); e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		want, _ := hex.DecodeString(r.Hex)
		got, e := BakalRewardList115(r.Branch, []BakalRewardActor{{Actor: r.Actor, Name: r.Name, CardIndex: r.Card, Materials: r.Items}})
		if e != nil || !bytes.Equal(got, want) {
			t.Fatalf("branch%d: %x %v", r.Branch, got, e)
		}
	}
	if _, e := BakalRewardList115(0, []BakalRewardActor{{Actor: 1, Name: "actor", CardIndex: -2}}); e == nil {
		t.Fatal("invalid card accepted")
	}
	if _, e := BakalRewardList115(0, []BakalRewardActor{{Actor: 1, Name: "actor", CardIndex: 0, Materials: []BakalRewardMaterial{{Template: 1, Amount: 0}}}}); e == nil {
		t.Fatal("zero reward accepted")
	}
}
