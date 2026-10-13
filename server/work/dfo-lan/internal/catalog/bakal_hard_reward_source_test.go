package catalog

import "testing"

func TestBakalHardRewardSourceIsSeparate(t *testing.T) {
	r, e := ImportBakalRaid(OpenNativeArchive(t))
	if e != nil {
		t.Fatal(e)
	}
	defer r.DungeonCatalog.CloseMapSource()
	if len(r.Rewards) != 10 || len(r.HardRewards) != 15 {
		t.Fatalf("normal%d hard%d", len(r.Rewards), len(r.HardRewards))
	}
	normal := map[uint32]bool{}
	for _, x := range r.Rewards {
		normal[x.Template] = true
	}
	for _, x := range r.HardRewards {
		if x.Difficulty != 1 || normal[x.Template] {
			t.Fatal("hard source mixed with normal")
		}
	}
}

func TestBakalBiddingSystemMatchesNativeSource(t *testing.T) {
	r, e := ImportBakalRaid(OpenNativeArchive(t))
	if e != nil {
		t.Fatal(e)
	}
	defer r.DungeonCatalog.CloseMapSource()
	b := r.Bidding
	if b.ItemsMax != 5 || b.StartMinPrice != 100 || b.GoldStep != 10000 || b.GoldMultiplier != 1.05 {
		t.Fatalf("bidding source changed: %+v", b)
	}
}
