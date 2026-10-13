package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type biddingHoldReceiptFailure struct{ *biddingEventFake }

func (s biddingHoldReceiptFailure) CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error) {
	if strings.HasPrefix(key, "bakal-bidding-bid:") {
		return nil, fmt.Errorf("simulated receipt read timeout after debit")
	}
	return s.biddingEventFake.CharacterEventReceipt(ctx, account, id, key)
}

func TestBakalCommittedHoldDoesNotDependOnSecondReceiptQuery(t *testing.T) {
	s, store, p, _ := bakalReservationFixture(t)
	s.Store = biddingHoldReceiptFailure{store}
	r, hold, e := s.ReserveBiddingLot(context.Background(), store.role, "owned-run", 0, 10000, p.ReadyAt)
	if e != nil || !hold.Reserved || bakalReservedGold(t, r) != 90000 {
		t.Fatal("committed bid falsely rejected after debit", e)
	}
}

func bakalReservationFixture(t *testing.T) (*BakalRewardService, *biddingEventFake, BakalBiddingPlan, time.Time) {
	t.Helper()
	s, store, at := biddingFixture(t)
	bag, e := inventory.ReadBag(store.role.State)
	if e != nil {
		t.Fatal(e)
	}
	bag.Gold = 100000
	store.role.State, e = inventory.SaveBag(store.role.State, bag)
	if e != nil {
		t.Fatal(e)
	}
	p, _, e := s.FreezeBidding(context.Background(), store.role, "owned-run", false, at)
	if e != nil {
		t.Fatal(e)
	}
	return s, store, p, at
}

func bakalReservedGold(t *testing.T, r database.Character) uint32 {
	t.Helper()
	b, e := inventory.ReadBag(r.State)
	if e != nil {
		t.Fatal(e)
	}
	return b.Gold
}

func TestBakalReservedBidCommitsOnceAndSaleDoesNotChargeAgain(t *testing.T) {
	s, store, p, at := bakalReservationFixture(t)
	ctx := context.Background()
	if _, _, e := s.ReserveBiddingLot(ctx, store.role, "owned-run", 0, 10000, at); e == nil {
		t.Fatal("bid before ready")
	}
	if _, _, e := s.ReserveBiddingLot(ctx, store.role, "owned-run", 0, 9999, p.ReadyAt); e == nil {
		t.Fatal("below owner minimum")
	}
	r, hold, e := s.ReserveBiddingLot(ctx, store.role, "owned-run", 0, 10000, p.ReadyAt)
	if e != nil || !hold.Reserved || bakalReservedGold(t, r) != 90000 {
		t.Fatalf("reservation %+v %v", hold, e)
	}
	writes := store.writes
	r, _, e = s.ReserveBiddingLot(ctx, r, "owned-run", 0, 10000, p.ReadyAt.Add(time.Second))
	if e != nil || store.writes != writes || bakalReservedGold(t, r) != 90000 {
		t.Fatal("replay charged twice", e)
	}
	if _, _, e = s.ReserveBiddingLot(ctx, r, "owned-run", 0, 20000, p.ReadyAt); e == nil {
		t.Fatal("receipt price conflict accepted")
	}
	r, sale, e := s.PurchaseBiddingLot(ctx, r, "owned-run", 0, 10000, p.ReadyAt.Add(8*time.Second))
	if e != nil || !sale.Sold || sale.Reserved || !sale.Closed || bakalReservedGold(t, r) != 90000 {
		t.Fatal("sale charged hold twice", e)
	}
	b, _ := inventory.ReadBag(r.State)
	if len(b.Items) != 1 || b.Items[0].Amount != 1 {
		t.Fatal("reserved item not paid")
	}
	r, e = s.ReleaseBiddingReservations(ctx, r, "")
	if e != nil || bakalReservedGold(t, r) != 90000 {
		t.Fatal("sale also refunded", e)
	}
	if _, _, e = s.ReserveBiddingLot(ctx, r, "owned-run", 0, 10000, p.ReadyAt); e == nil {
		t.Fatal("completed hold replay accepted")
	}
}

func TestBakalLostAuctionRefundsAndOldUnsoldLotCannotBeBought(t *testing.T) {
	s, store, p, _ := bakalReservationFixture(t)
	ctx := context.Background()
	r, _, e := s.ReserveBiddingLot(ctx, store.role, "owned-run", 0, 10000, p.ReadyAt)
	if e != nil {
		t.Fatal(e)
	}
	r, e = s.Recover(ctx, r, p.ReadyAt.Add(time.Minute))
	if e != nil || bakalReservedGold(t, r) != 100000 {
		t.Fatal("lost auction hold not recovered", e)
	}
	r, e = s.Recover(ctx, r, p.ReadyAt.Add(time.Hour))
	if e != nil || bakalReservedGold(t, r) != 100000 {
		t.Fatal("hold refunded twice", e)
	}
	if _, _, e = s.PurchaseBiddingLot(ctx, r, "owned-run", 0, 10000, p.ReadyAt); e == nil {
		t.Fatal("refunded lot sold")
	}
	r, lot, e := s.CloseUnsoldBiddingLot(ctx, r, "owned-run", 1)
	if e != nil || !lot.Closed {
		t.Fatal(e)
	}
	if _, _, e = s.ReserveBiddingLot(ctx, r, "owned-run", 1, 10000, p.ReadyAt); e == nil {
		t.Fatal("unsold closed lot accepted bid")
	}
	if _, _, e = s.PurchaseBiddingLot(ctx, r, "owned-run", 1, 10000, p.ReadyAt); e == nil {
		t.Fatal("unsold closed lot sold")
	}
}

func TestBakalReservedPayoutFailurePreservesRefundableHold(t *testing.T) {
	s, store, p, _ := bakalReservationFixture(t)
	ctx := context.Background()
	r, _, e := s.ReserveBiddingLot(ctx, store.role, "owned-run", 0, 10000, p.ReadyAt)
	if e != nil {
		t.Fatal(e)
	}
	top, plans, _ := readBakalBidding(r.State)
	plan := plans["owned-run"]
	plan.Lots[0].Products[0].Template = 999999999
	plans["owned-run"] = plan
	store.role.State, _ = saveBakalBidding(top, plans)
	before := string(store.role.State)
	if _, _, e = s.PurchaseBiddingLot(ctx, store.role, "owned-run", 0, 10000, p.ReadyAt); e == nil {
		t.Fatal("bad reserved payout accepted")
	}
	if string(store.role.State) != before || bakalReservedGold(t, store.role) != 90000 {
		t.Fatal("failed sale partially mutated wallet")
	}
	r, e = s.ReleaseBiddingReservations(ctx, store.role, "")
	if e != nil || bakalReservedGold(t, r) != 100000 {
		t.Fatal("failed payout hold lost", e)
	}
}
