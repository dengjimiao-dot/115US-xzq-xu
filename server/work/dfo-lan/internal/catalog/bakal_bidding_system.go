package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strconv"
)

// Current native1425C4B50 loads this shared raid bidding UI contract.
const BakalBiddingSystemSource = "contents/2021/ozma_raid/etc/biddingsystem.cos"

func loadBakalBiddingSystem(a *pvf.Archive, r *BakalRaidRules) error {
	cs, e := readBakalCOSTokens(a, BakalBiddingSystemSource)
	if e != nil {
		return e
	}
	for _, f := range []struct {
		key    string
		target *int
	}{
		{"[bidding item max]", &r.Bidding.ItemsMax},
	} {
		v, e := bakalRowInt(cs, f.key)
		if e != nil || v <= 0 {
			return fmt.Errorf("invalid bidding source %s", f.key)
		}
		*f.target = v
	}
	for _, f := range []struct {
		key    string
		target *uint32
	}{
		{"[bidding start min price]", &r.Bidding.StartMinPrice},
		{"[bidding gold default]", &r.Bidding.GoldStep},
	} {
		v, e := bakalRowInt(cs, f.key)
		if e != nil || v <= 0 {
			return fmt.Errorf("invalid bidding source %s", f.key)
		}
		*f.target = uint32(v)
	}
	values := sectionCells(cs, "[bidding add gold percent]")
	if len(values) != 1 {
		return fmt.Errorf("missing bidding price multiplier")
	}
	v := float64(values[0].Number)
	if values[0].Type == 2 && values[0].Text != "" {
		var err error
		v, err = strconv.ParseFloat(values[0].Text, 64)
		if err != nil {
			return err
		}
	}
	if values[0].Type == 0 {
		v = float64(values[0].Value)
	}
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 1 {
		return fmt.Errorf("invalid bidding price multiplier")
	}
	r.Bidding.GoldMultiplier = v
	// The client and domain read different source tables for these clocks.
	// Fail on disagreement instead of silently introducing a second duration.
	for _, f := range []struct {
		key  string
		want int
	}{
		{"[bidding start delay time]", r.Bidding.StartDelaySecs},
		{"[bidding break time]", r.Bidding.BreakSecs},
		{"[bidding time]", r.Bidding.TimeSecs},
	} {
		got, e := bakalRowInt(cs, f.key)
		if e != nil || got != f.want {
			return fmt.Errorf("bidding clock source mismatch: %s", f.key)
		}
	}
	return nil
}
