package main

import (
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// Reuses R7's owned solo recruitment lifecycle and the existing US codecs.
// No combat, reward, or CN packet implementation is enabled here.
func (c *gameConnection) releaseNonBakalRaid() {
	if c.gatewayRuntime == nil || c.nonBakalOwner == 0 {
		return
	}
	t, ok := c.nonBakalTeams.get(c.nonBakalOwner, c.channel)
	if ok && t.Recruitment.ID == c.nonBakalRaidID {
		c.nonBakalTeams.leave(c.nonBakalOwner, c.nonBakalRaidID)
		if w := c.worldState; w != nil && w.role.ID == c.nonBakalOwner {
			w.state.Position = t.Origin
			w.nonBakalWaiting = false
		}
	}
	c.nonBakalOwner, c.nonBakalRaidID = 0, 0
}

func (c *gameConnection) nonBakalOwnedTeam() (raidTeam, bool) {
	if c.gatewayRuntime == nil || c.worldState == nil || c.nonBakalOwner == 0 || c.worldState.role.ID != c.nonBakalOwner {
		return raidTeam{}, false
	}
	t, ok := c.nonBakalTeams.get(c.nonBakalOwner, c.channel)
	return t, ok && t.Recruitment.ID == c.nonBakalRaidID && t.Recruitment.Leader == c.worldState.role.WireID
}

func (c *gameConnection) sendNonBakalArea() error {
	w := c.worldState
	area, err := w.userAreaPayload()
	if err != nil {
		return err
	}
	if err = c.output.send(0, 23, area); err != nil {
		return err
	}
	w.enterArea()
	if err = w.introducePeers(c.output.send); err != nil {
		return err
	}
	scene, err := w.areaPayload()
	if err != nil {
		return err
	}
	return c.output.send(0, 24, scene)
}

func (c *gameConnection) dispatchNonBakalRaid(q *clientRequest) dispatchAction {
	if q.frame.Type != 1 || c.gatewayRuntime == nil || c.worldState == nil {
		return dispatchNext
	}
	w := c.worldState
	kind, isRaid := protocol.RaidKindForChannel(w.channelType)
	if !isRaid || kind == 8 {
		return dispatchNext
	} // Both Bakal modes remain untouched.
	switch q.frame.ID {
	case 650, 656, 657, 658, 661, 1353, 2121:
	case 15, 16:
		if !w.nonBakalWaiting {
			return dispatchNext
		}
	default:
		return dispatchNext
	}
	if !q.verified || !c.bootstrapped {
		return dispatchHandled
	}
	if c.nonBakalOwner != 0 && c.nonBakalOwner != w.role.ID {
		c.releaseNonBakalRaid()
	}
	refuse := func(reason string) dispatchAction {
		if c.event != nil {
			c.event(map[string]any{"kind": "raid_entry_refused", "id": q.frame.ID, "character_id": w.role.ID, "channel_type": w.channelType, "reason": reason, "attempt": "2/3"})
		}
		if c.output.send(1, q.frame.ID, protocol.Refusal(19)) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	send := func(id uint16, body []byte) dispatchAction {
		if c.output.send(1, id, body) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	switch q.frame.ID {
	case 15, 16:
		return refuse("owned raid has no started phase; ordinary dungeon entry is unavailable")
	case 650:
		if _, err := protocol.DecodeRaidUpdateControl(q.plaintext); err != nil {
			return refuse(err.Error())
		}
		if c.output.send(0, 585, protocol.RaidEmptyEntryCostInfo()) != nil {
			return dispatchClose
		}
		return send(650, []byte{1})
	case 2121:
		if _, err := protocol.DecodeRaidUpdateControl(q.plaintext); err != nil {
			return refuse(err.Error())
		}
		return send(2121, []byte{1})
	case 656:
		req, err := protocol.DecodeRaidCreateRequest(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		if req.Kind != kind {
			return refuse("raid kind does not match channel")
		}
		rules, ok := c.raidEntrances[w.channelType]
		if !ok {
			return refuse("PVF waiting room is not bound")
		}
		if c.nonBakalOwner != 0 || w.activeDungeon != nil || w.selectingDungeon || w.role.ID <= 0 || w.nonBakalEntrance == nil {
			return refuse("raid creation requires an unowned isolated town session")
		}
		if uint32(w.level) < rules.Waiting.MinimumLevel || rules.Waiting.AreaID == 0 || rules.Waiting.AreaID > 65535 {
			return refuse("invalid waiting area or level requirement")
		}
		if room := rules.Recruiting; room != nil && (w.state.Position.Town != room.TownID || w.state.Position.Area != room.AreaID) {
			return refuse("raid creation requires its source recruiting room")
		}
		if w.role.WireID == 0 || w.role.WireID == 65535 || rules.MemberMax == 0 || rules.MemberMax > 255 {
			return refuse("invalid member identity or source limit")
		}
		var state character.State
		if err = json.Unmarshal(w.role.State, &state); err != nil {
			return refuse("invalid saved character state")
		}
		advance, err := state.WireAdvancement()
		if err != nil {
			return refuse(err.Error())
		}
		entry := protocol.RaidRecruitment{Create: req, Leader: w.role.WireID, LeaderProfession: w.role.Profession, LeaderAdvancement: advance, LeaderLevel: w.level, LeaderName: w.role.Name, MemberCount: 1, MemberArea: uint16(rules.Waiting.AreaID), MemberMax: rules.MemberMax}
		team, err := c.nonBakalTeams.create(w.role.ID, c.channel, c.channelCfg.ServerID, entry, w.state.Position)
		if err != nil {
			return refuse(err.Error())
		}
		c.nonBakalOwner, c.nonBakalRaidID = w.role.ID, team.Recruitment.ID
		fail := func(err error) dispatchAction {
			c.releaseNonBakalRaid()
			if c.event != nil {
				c.event(map[string]any{"kind": "raid_create_delivery_error", "error": err.Error()})
			}
			return dispatchClose
		}
		body, err := protocol.RaidOwnedDetails(team.Recruitment)
		if err != nil {
			return fail(err)
		}
		if err = c.output.send(0, 578, body); err != nil {
			return fail(err)
		}
		if err = c.output.send(1, 656, protocol.RaidCreateSuccess(team.Recruitment.ID)); err != nil {
			return fail(err)
		}
		x, y := rules.Waiting.Spawn()
		w.nonBakalWaiting = true
		w.state.Position = database.WorldPosition{Town: rules.Waiting.TownID, Area: rules.Waiting.AreaID, X: x, Y: y}
		if err = c.sendNonBakalArea(); err != nil {
			return fail(err)
		}
		if c.event != nil {
			c.event(map[string]any{"kind": "raid_created_waiting", "raid_id": team.Recruitment.ID, "channel_type": w.channelType, "source": rules.Path, "member_max": rules.MemberMax, "attempt": "2/3"})
		}
		return dispatchHandled
	case 1353:
		mode, id, err := protocol.DecodeRaidInfoRequest(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		t, ok := c.nonBakalOwnedTeam()
		if !ok || id != t.Recruitment.ID || (mode != 1 && mode != 2) {
			return refuse("raid info does not name owned team")
		}
		var body []byte
		if mode == 1 {
			body, err = protocol.RaidMemberInfoSuccess(t.Recruitment)
		} else {
			body, err = protocol.RaidFullInfoSuccess(t.Recruitment)
		}
		if err != nil {
			return refuse(err.Error())
		}
		if c.output.send(1, 1353, body) != nil || c.output.send(0, 1220, []byte{0}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	case 661:
		actor, pos, err := protocol.DecodeRaidAssignment(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		_, ok := c.nonBakalOwnedTeam()
		rules, bound := c.raidEntrances[w.channelType]
		if !ok || !bound {
			return refuse("no owned raid")
		}
		t, err := c.nonBakalTeams.assign(w.role.ID, c.channel, actor, pos, rules.MemberMax)
		if err != nil {
			return refuse(err.Error())
		}
		body, err := protocol.RaidAssignmentUpdate(t.Recruitment)
		if err != nil {
			return refuse(err.Error())
		}
		if c.output.send(0, 578, body) != nil {
			return dispatchClose
		}
		return send(661, protocol.RaidAssignmentSuccess())
	case 657:
		if _, err := protocol.DecodeRaidLeaveRequest(q.plaintext); err != nil {
			return refuse(err.Error())
		}
		if _, ok := c.nonBakalOwnedTeam(); !ok {
			return refuse("no owned raid")
		}
		if c.output.send(1, 657, protocol.RaidLeaveSuccess()) != nil {
			return dispatchClose
		}
		c.releaseNonBakalRaid()
		if err := c.sendNonBakalArea(); err != nil {
			return dispatchClose
		}
		body, err := protocol.RaidRecruitmentList(nil)
		if err != nil {
			return dispatchClose
		}
		if c.output.send(0, 577, body) != nil {
			return dispatchClose
		}
		return dispatchHandled
	case 658:
		return refuse(fmt.Sprintf("raid phase execution is not yet connected for channel %d", w.channelType))
	}
	return dispatchNext
}
