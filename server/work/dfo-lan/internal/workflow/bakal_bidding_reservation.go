package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/inventory"
)

// One owned member participates in these raids. Its first accepted bid is
// held until the timer closes; this is not a multi-party auction controller.
func (s *BakalRewardService) ReserveBiddingLot(ctx context.Context, role database.Character, run string, lot int, price uint32, now time.Time) (database.Character, BakalBiddingLot, error) {
	if s == nil || s.Store == nil || s.Rules == nil || lot < 0 || price < BakalAuctionStartPrice || price < s.Rules.Bidding.StartMinPrice || price < s.Rules.Bidding.GoldStep {
		return role, BakalBiddingLot{}, fmt.Errorf("invalid source bidding reservation")
	}
	saved, row, err := s.commitBiddingReservation(ctx, role, run, lot, func(current database.Character) (json.RawMessage, BakalBiddingLot, error) {
		top, plans, e := readBakalBidding(current.State)
		if e != nil {
			return nil, BakalBiddingLot{}, e
		}
		plan, ok := plans[run]
		if !ok || plan.Source != current.ConfigVersion || plan.Content != s.Content || lot >= len(plan.Lots) || now.Before(plan.ReadyAt) {
			return nil, BakalBiddingLot{}, fmt.Errorf("bidding reservation owner/deadline mismatch")
		}
		r := plan.Lots[lot]
		if r.Sold || r.Closed || r.Reserved || r.Released {
			return nil, r, fmt.Errorf("bidding lot already reserved or closed")
		}
		bag, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, r, e
		}
		if bag.Gold < price {
			return nil, r, fmt.Errorf("insufficient bidding gold")
		}
		bag.Gold -= price
		state, e := inventory.SaveBag(current.State, bag)
		if e != nil {
			return nil, r, e
		}
		top, _, e = readBakalBidding(state)
		if e != nil {
			return nil, r, e
		}
		r.Reserved, r.Price = true, price
		plan.Lots[lot] = r
		plans[run] = plan
		state, e = saveBakalBidding(top, plans)
		return state, r, e
	})
	if err == nil && (!row.Reserved || row.Price != price) {
		err = fmt.Errorf("bidding reservation receipt conflict")
	}
	if err == nil {
		_, plans, e := readBakalBidding(saved.State)
		if e != nil {
			err = e
		} else {
			p, ok := plans[run]
			if !ok || lot >= len(p.Lots) || !p.Lots[lot].Reserved || p.Lots[lot].Sold || p.Lots[lot].Closed || p.Lots[lot].Released {
				err = fmt.Errorf("bidding reservation already finished")
			}
		}
	}
	return saved, row, err
}

// The hold's current row is the authoritative acknowledgement. Reading the
// receipt in a second query after a successful debit could fail after the
// deadline and falsely reject an already committed bid.
func (s *BakalRewardService) commitBiddingReservation(ctx context.Context, role database.Character, run string, lot int, apply func(database.Character) (json.RawMessage, BakalBiddingLot, error)) (database.Character, BakalBiddingLot, error) {
	saved, _, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, fmt.Sprintf("bakal-bidding-bid:%s:%d", run, lot), "bakal-bidding-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		next, r, e := apply(current)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(r)
		return next, receipt, e
	})
	if err != nil {
		return role, BakalBiddingLot{}, err
	}
	saved.WireID = role.WireID
	_, plans, err := readBakalBidding(saved.State)
	if err != nil {
		return saved, BakalBiddingLot{}, err
	}
	p, ok := plans[run]
	if !ok || p.Source != saved.ConfigVersion || p.Content != s.Content || lot >= len(p.Lots) {
		return saved, BakalBiddingLot{}, fmt.Errorf("committed bidding hold missing")
	}
	return saved, p.Lots[lot], nil
}

func (s *BakalRewardService) CloseUnsoldBiddingLot(ctx context.Context, role database.Character, run string, lot int) (database.Character, BakalBiddingLot, error) {
	if s == nil || s.Store == nil || lot < 0 {
		return role, BakalBiddingLot{}, fmt.Errorf("invalid unsold bidding lot")
	}
	return commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-bidding-unsold:%s:%d", run, lot), "bakal-bidding-v1", func(current database.Character) (json.RawMessage, BakalBiddingLot, error) {
		top, plans, e := readBakalBidding(current.State)
		if e != nil {
			return nil, BakalBiddingLot{}, e
		}
		p, ok := plans[run]
		if !ok || p.Source != current.ConfigVersion || p.Content != s.Content || lot >= len(p.Lots) {
			return nil, BakalBiddingLot{}, fmt.Errorf("unsold bidding owner mismatch")
		}
		r := p.Lots[lot]
		if r.Reserved || r.Sold {
			return nil, r, fmt.Errorf("reserved lot cannot finish unsold")
		}
		r.Closed = true
		p.Lots[lot] = r
		plans[run] = p
		state, e := saveBakalBidding(top, plans)
		return state, r, e
	})
}

// Only unfinished holds are refunded. A completed sale has already consumed
// its reservation and cannot also be refunded. Used on leaving and re-entry;
// old saves have no Reserved fields and remain unchanged.
func (s *BakalRewardService) ReleaseBiddingReservations(ctx context.Context, role database.Character, onlyRun string) (database.Character, error) {
	if s == nil || s.Store == nil {
		return role, fmt.Errorf("bidding store missing")
	}
	_, plans, err := readBakalBidding(role.State)
	if err != nil {
		return role, err
	}
	var runs []string
	for run := range plans {
		if onlyRun == "" || onlyRun == run {
			runs = append(runs, run)
		}
	}
	sort.Strings(runs)
	var errs []error
	for _, run := range runs {
		for lot, r := range plans[run].Lots {
			if !r.Reserved || r.Sold || r.Released {
				continue
			}
			saved, _, e := commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-bidding-release:%s:%d", run, lot), "bakal-bidding-v1", func(current database.Character) (json.RawMessage, BakalBiddingLot, error) {
				top, currentPlans, e := readBakalBidding(current.State)
				if e != nil {
					return nil, BakalBiddingLot{}, e
				}
				p, ok := currentPlans[run]
				if !ok || lot >= len(p.Lots) {
					return nil, BakalBiddingLot{}, fmt.Errorf("reservation missing")
				}
				r := p.Lots[lot]
				if !r.Reserved || r.Sold || r.Released {
					return nil, r, fmt.Errorf("reservation no longer refundable")
				}
				bag, e := inventory.ReadBag(current.State)
				if e != nil {
					return nil, r, e
				}
				if uint64(bag.Gold)+uint64(r.Price) > math.MaxUint32 {
					return nil, r, fmt.Errorf("bidding refund gold overflow")
				}
				bag.Gold += r.Price
				state, e := inventory.SaveBag(current.State, bag)
				if e != nil {
					return nil, r, e
				}
				top, _, e = readBakalBidding(state)
				if e != nil {
					return nil, r, e
				}
				r.Reserved, r.Released, r.Closed = false, true, true
				p.Lots[lot] = r
				currentPlans[run] = p
				state, e = saveBakalBidding(top, currentPlans)
				return state, r, e
			})
			role = saved
			if e != nil {
				errs = append(errs, e)
			}
		}
	}
	return role, errors.Join(errs...)
}
