package main

import (
	"encoding/binary"
	"fmt"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
)

// The normal and hard complete captures both advance result2 to stage4
// forty seconds after N588. The ordered [MOVIE END TIME] source values are
// 16/21/40/24; only the witnessed final-result slot is used here.
// This is a server deadline, not an acknowledgement of client movie playback.
type bakalRewardUI struct {
	run     string
	readyAt time.Time
	stage   byte
	packets []outboundPacket
}

func (w *worldSession) prepareBakalRewardUI(plan workflow.BakalRewardPlan, now time.Time) (*bakalRewardUI, error) {
	if w.bakal == nil || !w.bakal.NativeRuntime() {
		return nil, nil
	}
	if w.bakalRules == nil || len(w.bakalRules.MovieEndTimes) != 4 || w.bakalRules.MovieEndTimes[2] <= 0 {
		return nil, fmt.Errorf("missing source final-result movie deadline")
	}
	ui := &bakalRewardUI{run: w.bakalRun, readyAt: now.Add(time.Duration(w.bakalRules.MovieEndTimes[2]) * time.Second), stage: 3}
	appendState := func(stage byte) error {
		ui.packets = append(ui.packets, outboundPacket{"bakal_reward_state", 0, 574, []byte{stage, 0}})
		if w.bakalRecruitment != nil {
			r := *w.bakalRecruitment
			r.State = stage
			r.ActiveStartedAt = uint32(w.bakal.ActiveSince().Unix())
			body, err := protocol.RaidOwnedDetails(r)
			if err != nil {
				return err
			}
			binary.LittleEndian.PutUint32(body[4:], 2)
			ui.packets = append(ui.packets, outboundPacket{"bakal_reward_owned_state", 0, 578, body})
		}
		return nil
	}
	if err := appendState(4); err != nil {
		return nil, err
	}
	for _, card := range []struct {
		category string
		branch   byte
		index    int8
	}{{"squad_item", 0, 0}, {"party_card", 3, -1}} {
		var actors []protocol.BakalRewardActor
		if plan.Eligible {
			awards, ok := plan.Cards[card.category]
			if !ok {
				return nil, fmt.Errorf("frozen reward card %s unavailable", card.category)
			}
			actor := protocol.BakalRewardActor{Actor: w.role.WireID, Name: w.role.Name, CardIndex: card.index}
			for _, a := range awards {
				item, ok := w.loot.Catalog.Items[a.Template]
				if !ok || item.Kind != "stackable" || a.Amount > 65535 {
					return nil, fmt.Errorf("unbound reward material presentation %d/%d", a.Template, a.Amount)
				}
				actor.Materials = append(actor.Materials, protocol.BakalRewardMaterial{Template: a.Template, Amount: uint16(a.Amount)})
			}
			actors = append(actors, actor)
		}
		body, err := protocol.BakalRewardList115(card.branch, actors)
		if err != nil {
			return nil, err
		}
		ui.packets = append(ui.packets, outboundPacket{"bakal_frozen_" + card.category, 0, 587, body})
	}
	if err := appendState(6); err != nil {
		return nil, err
	}
	if err := appendState(7); err != nil {
		return nil, err
	}
	return ui, nil
}

func (w *worldSession) tickBakalRewardUI(now time.Time) []legion.BakalFrame {
	ui := w.bakalRewardUI
	if ui == nil || ui.run != w.bakalRun || w.bakal == nil || w.bakal.Stage() != legion.BakalOpeningEnded || now.Before(ui.readyAt) {
		return nil
	}
	frames := ui.advance(now, w.bakalRules)
	if len(frames) > 0 && w.bakalRecruitment != nil {
		w.bakalRecruitment.State = ui.stage
	}
	return frames
}

func (ui *bakalRewardUI) advance(now time.Time, rules *catalog.BakalRaidRules) []legion.BakalFrame {
	if now.Before(ui.readyAt) {
		return nil
	}
	if ui.stage == 3 {
		frames := make([]legion.BakalFrame, 0, len(ui.packets))
		for _, p := range ui.packets {
			frames = append(frames, legion.BakalFrame{Name: p.Name, ID: p.ID, Body: p.Payload})
		}
		ui.stage = 7
		ui.readyAt = now.Add(time.Duration(rules.Bidding.WaitSecs) * time.Second)
		return frames
	}
	// Stage8 opens the auction and requires a nonempty N2060 table. The
	// auction controller must publish its durable lot table before advancing.
	return nil
}
