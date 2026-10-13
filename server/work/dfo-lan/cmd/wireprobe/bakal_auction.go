package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
)

type bakalAuction struct {
	run      string
	plan     workflow.BakalBiddingPlan
	lot      int
	price    uint32
	deadline time.Time
	requests map[string]int
}

func (w *worldSession) prepareBakalAuction(ctx context.Context, clear workflow.BakalRewardPlan, now time.Time) error {
	if !clear.Eligible || w.bakalRewardUI == nil || w.bakalAuction != nil {
		return nil
	}
	if w.bakalRecruitment == nil || w.bakalRecruitment.MemberCount != 1 {
		return fmt.Errorf("bidding requires owned solo formation")
	}
	p, saved, err := w.bakalRewards.FreezeBidding(ctx, w.role, w.bakalRun, clear.Hard, now)
	w.role = saved
	if err != nil {
		return err
	}
	if len(p.Lots) == 0 {
		return nil
	}
	if len(p.Lots) > w.bakalRules.Bidding.ItemsMax || len(p.Lots) > 255 {
		return fmt.Errorf("source auction lot count outside native limit")
	}
	var rows []protocol.BakalBiddingItem
	for _, lot := range p.Lots {
		item, ok := w.loot.Catalog.Items[lot.Award.Template]
		if !ok || item.Kind != "stackable" {
			return fmt.Errorf("unbound native bidding preview %d", lot.Award.Template)
		}
		rows = append(rows, protocol.BakalBiddingItem{Template: lot.Award.Template, Amount: lot.Award.Amount})
	}
	participants, err := protocol.BakalBiddingParticipants([]protocol.BakalBiddingParticipant{{Actor: w.role.WireID, Eligible: true}})
	if err != nil {
		return err
	}
	items, err := protocol.BakalBiddingItems(rows)
	if err != nil {
		return err
	}
	w.bakalRewardUI.packets = append([]outboundPacket{{"bakal_bidding_participants", 0, 2061, participants}, {"bakal_bidding_lots", 0, 2060, items}}, w.bakalRewardUI.packets...)
	w.bakalAuction = &bakalAuction{run: w.bakalRun, plan: p, lot: -1, requests: map[string]int{}}
	return nil
}

func (w *worldSession) bakalAuctionState(stage byte) ([]outboundPacket, error) {
	if w.bakalRecruitment == nil || w.bakalRewardUI == nil {
		return nil, fmt.Errorf("auction owned state unavailable")
	}
	r := *w.bakalRecruitment
	r.State = stage
	r.ActiveStartedAt = uint32(w.bakal.ActiveSince().Unix())
	body, err := protocol.RaidOwnedDetails(r)
	if err != nil {
		return nil, err
	}
	binary.LittleEndian.PutUint32(body[4:], 2)
	w.bakalRecruitment.State = stage
	w.bakalRewardUI.stage = stage
	return []outboundPacket{{"bakal_auction_state", 0, 574, []byte{stage, 0}}, {"bakal_auction_owned_state", 0, 578, body}}, nil
}

func (w *worldSession) startBakalAuctionLot(now time.Time) ([]outboundPacket, error) {
	a := w.bakalAuction
	next := a.lot + 1
	if next >= len(a.plan.Lots) {
		return nil, fmt.Errorf("source auction lot exhausted")
	}
	r := a.plan.Lots[next]
	body, err := protocol.BakalBiddingStart(r.Award.Template, r.Award.Amount, workflow.BakalAuctionStartPrice)
	if err != nil {
		return nil, err
	}
	packets, err := w.bakalAuctionState(9)
	if err != nil {
		return nil, err
	}
	a.lot, a.price = next, 0
	a.deadline = now.Add(time.Duration(w.bakalRules.Bidding.TimeSecs) * time.Second)
	return append(packets, outboundPacket{"bakal_bidding_start", 0, 2059, body}), nil
}

func (w *worldSession) tickBakalAuction(now time.Time) ([]outboundPacket, error) {
	a, ui := w.bakalAuction, w.bakalRewardUI
	if a == nil || ui == nil || a.run != w.bakalRun || w.bakal == nil || w.bakal.Stage() != legion.BakalOpeningEnded {
		return nil, nil
	}
	switch ui.stage {
	case 7:
		if now.Before(ui.readyAt) {
			return nil, nil
		}
		packets, err := w.bakalAuctionState(8)
		if err == nil {
			a.deadline = now.Add(time.Duration(w.bakalRules.Bidding.StartDelaySecs) * time.Second)
		}
		return packets, err
	case 8, 10:
		if now.Before(a.deadline) {
			return nil, nil
		}
		return w.startBakalAuctionLot(now)
	case 9:
		if now.Before(a.deadline) {
			return nil, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		var r workflow.BakalBiddingLot
		if a.price == 0 {
			w.role, r, err = w.bakalRewards.CloseUnsoldBiddingLot(ctx, w.role, a.run, a.lot)
		} else {
			w.role, r, err = w.bakalRewards.PurchaseBiddingLot(ctx, w.role, a.run, a.lot, a.price, now)
		}
		if err != nil {
			return nil, err
		}
		a.plan.Lots[a.lot] = r
		actor, name, outcome, price := uint16(0), "", byte(2), workflow.BakalAuctionStartPrice
		if a.price != 0 {
			actor, name, outcome, price = w.role.WireID, w.role.Name, 1, a.price
		}
		body, err := protocol.BakalBiddingResult(actor, name, byte(a.lot), r.Award.Template, r.Award.Amount, price, outcome, uint32(w.bakalRules.Bidding.TimeSecs)*1000)
		if err != nil {
			return nil, err
		}
		packets := []outboundPacket{{"bakal_bidding_lot_result", 1, 1902, body}}
		if outcome == 1 {
			bag, err := w.loot.Bootstrap(workflow.LootRole(w.role))
			if err != nil {
				return nil, err
			}
			packets = append(packets, outboundPacket{"bakal_bidding_award_inventory", 0, 13, bag})
		}
		stage := byte(10)
		if a.lot+1 == len(a.plan.Lots) {
			stage = 11
		}
		states, err := w.bakalAuctionState(stage)
		if err != nil {
			return nil, err
		}
		a.deadline = now.Add(time.Duration(w.bakalRules.Bidding.BreakSecs) * time.Second)
		return append(packets, states...), nil
	}
	return nil, nil
}

func (client *gameConnection) bakalBid(p []byte, now time.Time, key string) (bool, []outboundPacket, error) {
	w := client.worldState
	refuse := func(code uint16, reason string) (bool, []outboundPacket, error) {
		client.event(map[string]any{"kind": "bakal_bid_refused", "raid": w.bakalRun, "reason": reason})
		return true, []outboundPacket{{"bakal_bid_refused", 1, 1902, protocol.BakalBiddingFailure(code)}}, nil
	}
	price, err := protocol.DecodeBakalBid(p)
	if err != nil {
		return refuse(22, err.Error())
	}
	a, ui := w.bakalAuction, w.bakalRewardUI
	if a == nil || ui == nil || a.run != w.bakalRun || ui.stage != 9 || a.lot < 0 || now.Compare(a.deadline) >= 0 || w.bakal == nil || w.bakal.Stage() != legion.BakalOpeningEnded {
		return refuse(19, "no owned auction in progress")
	}
	previous, seen := a.requests[key]
	if key != "" && seen && previous != a.lot {
		return refuse(18, "stale bid frame belongs to previous lot")
	}
	if a.price != 0 && (a.price != price || key != "" && !seen) {
		return refuse(18, "owned member already bid")
	}
	if price < workflow.BakalAuctionStartPrice || price < w.bakalRules.Bidding.StartMinPrice || price < w.bakalRules.Bidding.GoldStep {
		return refuse(22, "bid below owner/source minimum")
	}
	if a.price == 0 {
		bag, err := inventory.ReadBag(w.role.State)
		if err != nil {
			return true, nil, err
		}
		if bag.Gold < price {
			return refuse(10, "insufficient bidding gold")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		saved, r, err := w.bakalRewards.ReserveBiddingLot(ctx, w.role, a.run, a.lot, price, now)
		w.role = saved
		if err != nil {
			return refuse(22, err.Error())
		}
		a.plan.Lots[a.lot] = r
		a.price = price
		a.deadline = now.Add(time.Duration(w.bakalRules.Bidding.TimeSecs) * time.Second)
	}
	if key != "" {
		a.requests[key] = a.lot
	}
	r := a.plan.Lots[a.lot]
	remaining := uint32(max(0, a.deadline.Sub(now)/time.Millisecond))
	body, err := protocol.BakalBiddingResult(w.role.WireID, w.role.Name, byte(a.lot), r.Award.Template, r.Award.Amount, a.price, 0, remaining)
	if err != nil {
		return true, nil, err
	}
	bag, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return true, nil, err
	}
	client.event(map[string]any{"kind": "bakal_bid_reserved", "raid": a.run, "lot": a.lot, "price": a.price})
	return true, []outboundPacket{{"bakal_bid_accepted", 1, 1902, body}, {"bakal_bid_reserved_inventory", 0, 13, bag}}, nil
}

func (w *worldSession) releaseBakalAuction() ([]outboundPacket, error) {
	if w.bakalAuction == nil {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, err := w.bakalRewards.ReleaseBiddingReservations(ctx, w.role, w.bakalAuction.run)
	w.role = saved
	if err != nil {
		return nil, err
	}
	body, err := w.loot.Bootstrap(workflow.LootRole(saved))
	if err != nil {
		return nil, err
	}
	w.bakalAuction = nil
	return []outboundPacket{{"bakal_bid_released_inventory", 0, 13, body}}, nil
}
