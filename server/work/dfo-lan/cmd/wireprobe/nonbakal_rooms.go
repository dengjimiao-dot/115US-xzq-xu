package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
)

func (w *worldSession) loadNonBakalEntry(ctx context.Context, role int64, level byte, odyssey bool, spawn database.WorldPosition, worldType uint32) (database.WorldState, error) {
	if w.nonBakalEntrance == nil {
		return w.service.Enter(ctx, w.account, role, level, odyssey, spawn, worldType)
	}
	w.nonBakalWaiting = false
	if room := w.nonBakalEntrance.Recruiting; room != nil {
		x, y := room.Spawn()
		spawn = database.WorldPosition{Town: room.TownID, Area: room.AreaID, X: x, Y: y}
	}
	if err := w.service.ValidatePosition(level, odyssey, spawn); err != nil {
		return database.WorldState{}, err
	}
	version := w.service.Catalog.Source.SaveIdentity()
	saved, err := w.service.Store.LoadWorld(ctx, w.account, role, worldType, spawn, version)
	if err != nil {
		return saved, err
	}
	if saved.ConfigVersion != version {
		return saved, fmt.Errorf("raid room source identity mismatch")
	}
	saved.Position = spawn
	return saved, nil
}

func (w *worldSession) authorizeNonBakalArea(r protocol.AreaChangeRequest) error {
	if w.nonBakalEntrance == nil {
		return nil
	}
	waiting := w.nonBakalEntrance.Waiting
	toWaiting := r.Town == waiting.TownID && r.Area == waiting.AreaID
	if w.nonBakalWaiting && !toWaiting {
		return fmt.Errorf("leave owned raid before leaving waiting room")
	}
	if w.nonBakalEntrance.Recruiting != nil && toWaiting && !w.nonBakalWaiting {
		return fmt.Errorf("waiting room requires an owned raid")
	}
	return nil
}
