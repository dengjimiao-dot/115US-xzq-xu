package raid

import "fmt"

func legacyStateValid(state string) bool {
	switch state {
	case "open", "hide", "close", "clear":
		return true
	}
	return false
}

// Validate snapshots before their changes can dispatch further source events.
// An action error rejects the entire Apply, including timers already consumed.
func (p *LegacyPhase) validateView(v LegacyView) error {
	for id, state := range v.Dungeons {
		if _, ok := p.source.Dungeons[id]; !ok || !legacyStateValid(state) {
			return fmt.Errorf("invalid dungeon snapshot %d", id)
		}
	}
	parties := map[byte]bool{}
	for id, occupants := range v.Occupants {
		d, ok := p.source.Dungeons[id]
		if !ok || len(occupants) > int(d.PartyMax) {
			return fmt.Errorf("invalid dungeon occupancy %d", id)
		}
		for _, party := range occupants {
			if parties[party] {
				return fmt.Errorf("party occupies multiple raid slots: %d", party)
			}
			parties[party] = true
		}
	}
	for key, timer := range v.Timers {
		if timer.Due < v.Now {
			return fmt.Errorf("raid timer precedes current time")
		}
		if key.Kind == -1 {
			if _, ok := p.source.Dungeons[key.Key]; !ok || !legacyStateValid(timer.State) {
				return fmt.Errorf("invalid reserved dungeon timer")
			}
		} else if timer.State != "" {
			return fmt.Errorf("event timer contains dungeon state")
		}
	}
	return nil
}
