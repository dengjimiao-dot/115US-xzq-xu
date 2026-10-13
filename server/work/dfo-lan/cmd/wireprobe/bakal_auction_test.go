package main

import (
	"encoding/binary"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
)

func bakalAuctionClient(t *testing.T) (*gameConnection, time.Time) {
	t.Helper()
	c, now := bakalNativeClient(t)
	w := c.worldState
	w.characters = &character.Service{}
	for _, d := range w.bakalRules.Dungeons {
		if d.Type != "" {
			loc, err := legion.BakalLocationOfDungeon(w.bakalRules, d.Index)
			if err != nil {
				t.Fatal(err)
			}
			w.bakal.EnterDungeon(d.Index, loc, now)
			w.bakal.LoadingDone(d.Index)
			if d.Type != "bakal" {
				w.bakal.DefeatMonster(d.Index, loc, now)
			}
		}
	}
	id := uint32(w.bakalRules.NormalPhase.SettlementDungeon)
	w.bakal.EnterDungeon(id, 24, now)
	w.bakal.LoadingDone(id)
	w.bakal.DefeatMonster(id, 24, now)
	lc, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = lc.SupplementStackables("../../internal/catalog/testdata/item-flow.json"); err != nil {
		t.Fatal(err)
	}
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", lc.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	w.loot = &loot.Service{Catalog: lc, BagRules: bagRules, Equipment: gear, RewardBoxes: bakalStackableBoxes{}}
	rules := *w.bakalRules
	rules.Rewards = []catalog.BakalRewardEntry{{Category: "party_card", Template: 10418036, Amount: 1, Weight: 1}, {Category: "squad_item", Template: 10418036, Amount: 1, Weight: 1}}
	rules.Bidding.Items = []catalog.BakalRewardEntry{{Template: 10418036, Amount: 1, Weight: 1}}
	rules.Bidding.Hards = []catalog.BakalBiddingHard{{Hard: 0, Rates: []catalog.BakalBiddingRate{{Grade: 2, Weight: 1}}}}
	rules.Bidding.WeeklyCounts = []catalog.BakalBiddingHard{{Hard: 0, Rates: []catalog.BakalBiddingRate{{Grade: 0, Weight: 1}}}}
	w.bakalRules = &rules
	w.role.State, err = inventory.SaveBag(w.role.State, inventory.Bag{Version: "ordinary-bag-v1", Gold: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	store := &bakalLedgerFake{role: w.role}
	w.bakalRewards = &workflow.BakalRewardService{Store: store, Loot: w.loot, Rules: &rules, Content: rules.Source}
	w.bakalRecruitment = &protocol.RaidRecruitment{ID: 1, Leader: w.role.WireID, LeaderName: w.role.Name, MemberCount: 1, MemberMax: 12, Create: protocol.RaidCreateRequest{Kind: 8, Title: "test"}}
	due := now.Add(time.Duration(rules.NormalPhase.SettlementTimer.Secs) * time.Second)
	if err = w.bakalSettleSplice(due); err != nil {
		t.Fatal(err)
	}
	w.bakal.Tick(due)
	if w.bakal.Stage() != legion.BakalOpeningEnded || w.bakalAuction == nil {
		t.Fatalf("auction preparation %v %v", w.bakal.Stage(), w.bakalAuctionError)
	}
	f := w.tickBakalRewardUI(w.bakalRewardUI.readyAt)
	if len(f) != 10 || f[0].ID != 2061 || f[1].ID != 2060 || binary.LittleEndian.Uint32(f[1].Body) != 2 {
		t.Fatalf("missing owned lot cache %+v", f)
	}
	openAt := w.bakalRewardUI.readyAt
	if p, err := w.tickBakalAuction(openAt); err != nil || len(p) != 2 || p[0].Payload[0] != 8 {
		t.Fatal("auction opening", err, p)
	}
	startAt := w.bakalAuction.deadline
	p, err := w.tickBakalAuction(startAt)
	if err != nil || len(p) != 3 || p[2].ID != 2059 || binary.LittleEndian.Uint32(p[2].Payload[8:]) != 10000 {
		t.Fatal("owner start price", err, p)
	}
	return c, startAt
}

func TestBakalAuctionBidWinAndUnsoldNativeSequence(t *testing.T) {
	c, at := bakalAuctionClient(t)
	w := c.worldState
	bid := binary.LittleEndian.AppendUint32([]byte{1}, 10000)
	h, p, err := c.bakalBid(bid, at.Add(time.Second), "first-frame")
	if err != nil || !h || len(p) != 2 || p[0].Kind != 1 || p[0].ID != 1902 || p[0].Payload[len(p[0].Payload)-5] != 0 {
		t.Fatal("native bid acknowledgement", err, p)
	}
	b, _ := inventory.ReadBag(w.role.State)
	if b.Gold != 990000 || len(b.Items) != 1 || b.Items[0].Amount != 2 {
		t.Fatal("bid did not hold only gold", b)
	}
	deadline := w.bakalAuction.deadline
	if _, _, err = c.bakalBid(bid, at.Add(2*time.Second), "first-frame"); err != nil {
		t.Fatal(err)
	}
	if w.bakalAuction.deadline != deadline {
		t.Fatal("replay extended countdown")
	}
	if p, err = w.tickBakalAuction(deadline.Add(-time.Nanosecond)); err != nil || len(p) != 0 {
		t.Fatal("sale too early")
	}
	p, err = w.tickBakalAuction(deadline)
	if err != nil || p[0].ID != 1902 || p[0].Kind != 1 || p[0].Payload[len(p[0].Payload)-5] != 1 || p[1].ID != 13 || p[2].Payload[0] != 10 {
		t.Fatal("winner/inventory/break order", err, p)
	}
	b, _ = inventory.ReadBag(w.role.State)
	if b.Gold != 990000 || b.Items[0].Amount != 3 {
		t.Fatal("sale charged twice or missed award", b)
	}
	p, err = w.tickBakalAuction(w.bakalAuction.deadline)
	if err != nil || p[len(p)-1].ID != 2059 {
		t.Fatal("next source lot", err, p)
	}
	_, p, err = c.bakalBid(bid, w.bakalAuction.deadline.Add(-time.Second), "first-frame")
	if err != nil || binary.LittleEndian.Uint16(p[0].Payload[1:]) != 18 {
		t.Fatal("old frame bid on next lot", err, p)
	}
	p, err = w.tickBakalAuction(w.bakalAuction.deadline)
	if err != nil || p[0].Payload[len(p[0].Payload)-5] != 2 || p[1].Payload[0] != 11 {
		t.Fatal("unsold/final sequence", err, p)
	}
	if p, err = w.tickBakalAuction(w.bakalAuction.deadline.Add(time.Hour)); err != nil || len(p) != 0 {
		t.Fatal("terminal auction replayed")
	}
	b, _ = inventory.ReadBag(w.role.State)
	if b.Gold != 990000 || b.Items[0].Amount != 3 {
		t.Fatal("unsold lot charged or granted")
	}
}

func TestBakalAuctionLeaveRefundsUnfinishedBid(t *testing.T) {
	c, at := bakalAuctionClient(t)
	w := c.worldState
	bid := binary.LittleEndian.AppendUint32([]byte{1}, 10000)
	if _, _, err := c.bakalBid(bid, at, "bid"); err != nil {
		t.Fatal(err)
	}
	request := binary.LittleEndian.AppendUint16(nil, w.role.WireID)
	request = append(request, make([]byte, 6)...)
	_, p, err := c.bakalLeaveRaid(request, at.Add(time.Second))
	if err != nil || len(p) != 3 || p[0].ID != 13 || p[1].ID != 657 || p[2].ID != 578 || w.bakalAuction != nil || w.bakal != nil {
		t.Fatal("leave/refund order", err, p)
	}
	b, _ := inventory.ReadBag(w.role.State)
	if b.Gold != 1000000 || b.Items[0].Amount != 2 {
		t.Fatal("unfinished bid did not refund", b)
	}
}

func TestBakalLiveActionZeroBidsAndPaysOnce(t *testing.T) {
	c, at := bakalAuctionClient(t)
	w := c.worldState
	// Reproduce the actual confirmed large-bid vector and wallet amount.
	const amount uint32 = 310034448
	b, _ := inventory.ReadBag(w.role.State)
	b.Gold = amount
	w.role.State, _ = inventory.SaveBag(w.role.State, b)
	w.bakalRewards.Store.(*bakalLedgerFake).role = w.role
	bid := binary.LittleEndian.AppendUint32([]byte{0}, amount)
	_, p, e := c.bakalBid(bid, at, "live-zero")
	if e != nil || p[0].Payload[0] != 1 {
		t.Fatal("live zero bid refused", e, p)
	}
	if _, _, e = c.bakalBid(bid, at.Add(time.Second), "live-zero"); e != nil {
		t.Fatal(e)
	}
	if _, e = w.tickBakalAuction(w.bakalAuction.deadline); e != nil {
		t.Fatal(e)
	}
	b, _ = inventory.ReadBag(w.role.State)
	if b.Gold != 0 || b.Items[0].Amount != 3 {
		t.Fatal("zero bid charged/granted twice", b)
	}
}

func TestBakalSubpartyLeavePreservesRaidAndRaidLeaveClearsClient(t *testing.T) {
	c, at := bakalAuctionClient(t)
	w := c.worldState
	w.soloPartyReady = true
	run, team := w.bakalRun, w.bakalRecruitment.ID
	_, p, e := c.bakalRewardClaimOrLeave(make([]byte, 8), at)
	if e != nil || w.bakal == nil || w.bakalRecruitment == nil || w.bakalRun != run || w.soloPartyReady || p[len(p)-1].ID != 9 {
		t.Fatal("subparty leave destroyed raid", e, p)
	}
	request := binary.LittleEndian.AppendUint16(nil, w.role.WireID)
	request = append(request, make([]byte, 6)...)
	_, p, e = c.bakalLeaveRaid(request, at)
	if e != nil || len(p) != 2 || p[0].ID != 657 || p[0].Kind != 1 || p[0].Payload[0] != 1 || p[1].ID != 578 || binary.LittleEndian.Uint32(p[1].Payload) != team || binary.LittleEndian.Uint32(p[1].Payload[4:]) != 1 || w.bakalRecruitment != nil {
		t.Fatal("raid cache not removed", e, p)
	}
	_, p, e = c.bakalLeaveRaid(request, at)
	if e != nil || len(p) != 1 || binary.LittleEndian.Uint16(p[0].Payload[1:]) != 19 {
		t.Fatal("duplicate leave not answered", e, p)
	}
	_, p, e = c.bakalRewardClaimOrLeave(make([]byte, 8), at)
	if e != nil || len(p) != 1 || binary.LittleEndian.Uint16(p[0].Payload[1:]) != 18 {
		t.Fatal("post-raid party leave pending", e, p)
	}
}
