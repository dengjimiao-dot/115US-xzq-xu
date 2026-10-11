package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"net"
	"strconv"
	"testing"
	"time"
)

func nonBakalFixture(t *testing.T, channel uint32) (*gameConnection, net.Conn, []byte) {
	t.Helper()
	server, peer := net.Pipe()
	t.Cleanup(func() { server.Close(); peer.Close() })
	peer.SetDeadline(time.Now().Add(10 * time.Second))
	log := func(map[string]any) {}
	rules := catalog.RaidEntrance{MemberMax: 12, StartMinimum: 1, PartyMax: 3, Waiting: catalog.TownArea{TownID: 152, AreaID: 2, Walkable: [][4]int32{{10, 20, 300, 400}}}, Recruiting: &catalog.TownArea{TownID: 152, AreaID: 1}}
	runtime := &gatewayRuntime{raidEntrances: map[uint32]catalog.RaidEntrance{channel: rules}}
	w := &worldSession{nonBakalEntrance: &rules, role: database.Character{ID: 1, WireID: 9, Profession: 16, Name: "owner", State: []byte(`{"level":115,"advancement":3,"awakening":3}`)}, level: 115, channelType: channel, state: database.WorldState{Position: database.WorldPosition{Town: 152, Area: 1, X: 50, Y: 60}}}
	c := &gameConnection{gatewayRuntime: runtime, worldState: w, channel: channel, channelCfg: channelrefresh.Config{ServerID: 1}, bootstrapped: true, event: log, output: newConnectionOutput(server, make([]byte, wire.SessionKeyBytes), "test", log)}
	kind, _ := protocol.RaidKindForChannel(channel)
	p := []byte{kind}
	p = binary.LittleEndian.AppendUint32(p, 4)
	p = append(p, []byte("raid")...)
	p = append(p, make([]byte, 10)...)
	return c, peer, p
}

func nonBakalExchange(t *testing.T, c *gameConnection, peer net.Conn, id uint16, p []byte, frames ...uint16) {
	t.Helper()
	done := make(chan dispatchAction, 1)
	go func() {
		done <- c.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: id}, plaintext: p, verified: true})
	}()
	for _, want := range frames {
		f := readDispatchServerFrame(t, peer)
		if got := binary.LittleEndian.Uint16(f[1:3]); got != want {
			t.Fatalf("frame got %d want %d", got, want)
		}
	}
	if got := <-done; got != dispatchHandled {
		t.Fatalf("dispatch=%d", got)
	}
}

func TestNonBakalCreateInfoLeaveLifecycle(t *testing.T) {
	for _, channel := range []uint32{67, 68, 76, 78, 93, 98, 107} {
		t.Run(strconv.Itoa(int(channel)), func(t *testing.T) {
			c, peer, p := nonBakalFixture(t, channel)
			origin := c.worldState.state.Position
			nonBakalExchange(t, c, peer, 656, p, 578, 656, 23, 24)
			team, ok := c.nonBakalOwnedTeam()
			if !ok || team.Recruitment.MemberMax != 12 || !c.worldState.privateArea(c.worldState.state.Position) {
				t.Fatal("ownership/source/private room missing")
			}
			nonBakalExchange(t, c, peer, 656, p, 656) // Duplicate creation must not replace ownership.
			again, _ := c.nonBakalOwnedTeam()
			if again.Recruitment.ID != team.Recruitment.ID {
				t.Fatal("duplicate replaced team")
			}
			info := binary.LittleEndian.AppendUint32([]byte{1}, team.Recruitment.ID)
			nonBakalExchange(t, c, peer, 1353, info, 1353, 1220)
			nonBakalExchange(t, c, peer, 657, []byte{2, 0, 0, 0, 0, 0, 0, 0}, 657, 23, 24, 577)
			if c.nonBakalOwner != 0 || c.worldState.nonBakalWaiting || c.worldState.state.Position != origin {
				t.Fatal("leave did not restore scene")
			}
		})
	}
}

func TestNonBakalPartialCreateDeliveryRollsBack(t *testing.T) {
	for _, frames := range []int{1, 2, 3} {
		t.Run(strconv.Itoa(frames), func(t *testing.T) {
			c, peer, p := nonBakalFixture(t, 67)
			origin := c.worldState.state.Position
			done := make(chan dispatchAction, 1)
			go func() {
				done <- c.dispatchNonBakalRaid(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: p, verified: true})
			}()
			for i := 0; i < frames; i++ {
				readDispatchServerFrame(t, peer)
			}
			peer.Close()
			if <-done != dispatchClose {
				t.Fatal("partial delivery not closed")
			}
			if _, ok := c.nonBakalTeams.get(1, 67); ok || c.nonBakalOwner != 0 || c.worldState.nonBakalWaiting || c.worldState.state.Position != origin {
				t.Fatal("partial delivery survived rollback")
			}
		})
	}
}

func TestNonBakalChangedRoleCleanupPreservesNewPosition(t *testing.T) {
	c, peer, p := nonBakalFixture(t, 67)
	nonBakalExchange(t, c, peer, 656, p, 578, 656, 23, 24)
	c.worldState.role.ID = 2
	c.worldState.state.Position = database.WorldPosition{Town: 99, Area: 3}
	c.releaseNonBakalRaid()
	if _, ok := c.nonBakalTeams.get(1, 67); ok {
		t.Fatal("old role remains allocated")
	}
	if c.worldState.state.Position.Town != 99 {
		t.Fatal("old cleanup changed new role position")
	}
}

func TestNonBakalCreateBrokenDeliveryRollsBack(t *testing.T) {
	c, peer, p := nonBakalFixture(t, 67)
	origin := c.worldState.state.Position
	peer.Close()
	if c.dispatchNonBakalRaid(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: p, verified: true}) != dispatchClose {
		t.Fatal("broken delivery not closed")
	}
	if _, ok := c.nonBakalTeams.get(1, 67); ok || c.nonBakalOwner != 0 || c.worldState.nonBakalWaiting || c.worldState.state.Position != origin {
		t.Fatal("partial create retained")
	}
}

func TestNonBakalAreaAndStaleOwnerBoundaries(t *testing.T) {
	c, peer, p := nonBakalFixture(t, 67)
	w := c.worldState
	if w.authorizeNonBakalArea(protocol.AreaChangeRequest{Town: 152, Area: 2}) == nil {
		t.Fatal("unowned waiting entry")
	}
	nonBakalExchange(t, c, peer, 656, p, 578, 656, 23, 24)
	if w.authorizeNonBakalArea(protocol.AreaChangeRequest{Town: 152, Area: 1}) == nil {
		t.Fatal("owned waiting escape")
	}
	oldID := c.nonBakalRaidID
	c.releaseNonBakalRaid()
	next, err := c.nonBakalTeams.create(1, 67, 1, protocol.RaidRecruitment{Leader: 9}, database.WorldPosition{})
	if err != nil {
		t.Fatal(err)
	}
	c.nonBakalOwner = 1
	c.nonBakalRaidID = oldID
	c.releaseNonBakalRaid()
	if got, ok := c.nonBakalTeams.get(1, 67); !ok || got.Recruitment.ID != next.Recruitment.ID {
		t.Fatal("old cleanup removed new team")
	}
}

func TestNonBakalDoesNotCaptureBakalOrUnverified(t *testing.T) {
	c, _, p := nonBakalFixture(t, 82)
	if c.dispatchNonBakalRaid(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: p, verified: true}) != dispatchNext {
		t.Fatal("Bakal captured")
	}
	c.worldState.channelType = 67
	if c.dispatchNonBakalRaid(&clientRequest{frame: wire.Frame{Type: 1, ID: 656}, plaintext: p}) != dispatchHandled || c.nonBakalOwner != 0 {
		t.Fatal("unverified create accepted")
	}
}

func TestNonBakalUnhandledPartyRequestIsLoggedFromDispatcher(t *testing.T) {
	c, _, events := newDispatchTestClient()
	c.bootstrapped = false
	c.channel = 7
	c.channelTypes = map[uint32]uint32{7: 67}
	c.worldState.channelType = 67
	c.channelDirectory = &catalog.ChannelDirectory{ByType: map[uint32]catalog.ChannelAttributes{67: {Type: 67, IsRaid: true}}}
	if got := c.dispatch(&clientRequest{frame: wire.Frame{Type: 1, ID: 12}, plaintext: []byte{0, 0}, verified: true}); got != dispatchNext {
		t.Fatal(got)
	}
	found := false
	for _, e := range *events {
		if e["kind"] == "raid_unhandled_request" && e["command"] == "CREATE_PARTY_IN_RAID" {
			found = true
		}
	}
	if !found {
		t.Fatal("unhandled party request still has no dispatch log")
	}
}
