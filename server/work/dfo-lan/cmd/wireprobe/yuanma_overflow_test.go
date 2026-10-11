package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func yuanmaOverflowWorld(t *testing.T, step byte, bag inventory.Bag) (*worldSession, *database.TestFixture) {
	t.Helper()
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })
	require.NoError(t, f.MigrateAccountMaterials(ctx))
	require.NoError(t, f.MigrateMailbox(ctx))
	account, err := f.DevelopmentAccount(ctx, "yuanma-overflow")
	require.NoError(t, err)
	prof, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	require.NoError(t, err)
	eq, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", prof.Source.Checksum)
	require.NoError(t, err)
	ls := &loot.Service{BoostAutoOverflowMail: true, Equipment: eq, BagRules: inventory.BagRules{Source: prof.Source.Checksum, EquipmentSlots: [2]uint16{9, 9}, Slots: map[string][2]uint16{"[material]": {121, 121}, "[booster]": {65, 65}}, MissingStackLimit: 10000}, Catalog: catalog.LootCatalog{Source: prof.Source, Items: map[uint32]catalog.LootItem{
		590015966: {ID: 590015966, Kind: "stackable", StackableType: "[material]", StackLimit: 10000}, 590015964: {ID: 590015964, Kind: "stackable", StackableType: "[booster]", StackLimit: 1000}, 3037: {ID: 3037, Kind: "stackable", StackableType: "[material]", StackLimit: 10000},
	}}}
	ls.RewardBoxes = boosterBoxSource{catalog: autoBoxCatalog()}
	c := &boostup.Catalog{Town: 222}
	for i := byte(1); i <= 11; i++ {
		c.Steps = append(c.Steps, boostup.Step{Number: i, Area: i - 1, Guide: "normal", Mission: "none"})
	}
	c.Steps[8].AutoOpen = true
	c.Steps[8].Mission = "disjoint"
	c.Steps[8].Rewards = []boostup.Reward{{Item: 9, Count: 1}}
	c.Steps[10].AutoOpen = true
	c.Steps[10].Rewards = []boostup.Reward{{Item: 11, Count: 1}}
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future_character":42}`), bag)
	require.NoError(t, err)
	raw, err = boostup.WriteState(raw, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: step, Phase: 1}})
	require.NoError(t, err)
	role, err := f.CreateCharacter(ctx, database.Character{AccountID: account, Name: "OverflowHero", Profession: 0, Request: []byte{0}, ConfigVersion: savecontract.Identity(), State: raw}, 100)
	require.NoError(t, err)
	return &worldSession{account: account, role: role, loot: ls, store: f.Storage(), boostup: c, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: uint32(step - 1)}}}, f
}

func yuanmaRequest(step byte) []byte {
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p, 662)
	binary.LittleEndian.PutUint32(p[4:], uint32(step))
	return p
}

func TestYuanmaOverflowMailCommitReplayAndReopen(t *testing.T) {
	w, f := yuanmaOverflowWorld(t, 9, inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}}})
	ctx := context.Background()
	notices := 0
	w.notifyBoostMail = func(id int64) { require.Equal(t, w.role.ID, id); notices++ }
	plan, err := w.boostStepRequest(ctx, yuanmaRequest(9), 680)
	require.NoError(t, err)
	require.EqualValues(t, 13, plan[0].ID)
	require.EqualValues(t, 680, plan[len(plan)-1].ID)
	require.Equal(t, 1, notices)
	messages, err := f.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Len(t, messages[0].Assets, 1)
	var item inventory.MailItem
	require.NoError(t, json.Unmarshal(messages[0].Assets[0].Item, &item))
	require.EqualValues(t, 20002, item.Equipment.Template)
	require.EqualValues(t, inventory.GrantExpireTime, item.Equipment.Period)
	for i := 0; i < 3; i++ {
		_, err = w.boostStepRequest(ctx, yuanmaRequest(9), 680)
		require.NoError(t, err)
	}
	messages, err = f.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	count, err := f.EventCount(ctx, w.role.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	reopened, err := f.Reopen(ctx)
	require.NoError(t, err)
	defer reopened.Close()
	roles, err := reopened.Characters(ctx, w.account)
	require.NoError(t, err)
	require.Len(t, roles, 1)
	svc := &workflow.LootService{Store: reopened, Loot: w.loot}
	_, _, applied, err := svc.BoostStepRequest(ctx, roles[0], w.boostup, 9, true)
	require.NoError(t, err)
	require.False(t, applied)
	messages, err = reopened.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
}

func TestYuanmaOverflowFullMailboxRollsBackAndCanRetry(t *testing.T) {
	w, f := yuanmaOverflowWorld(t, 9, inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}}})
	ctx := context.Background()
	for i := 0; i < 255; i++ {
		_, _, err := f.CommitSystemMail(ctx, w.account, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("fill:%d", i), "fixture", "Fill", "Fill", nil)
		require.NoError(t, err)
	}
	before, _, err := f.CharacterSnapshot(ctx, w.role.ID)
	require.NoError(t, err)
	events, err := f.EventCount(ctx, w.role.ID)
	require.NoError(t, err)
	_, err = w.boostStepRequest(ctx, yuanmaRequest(9), 680)
	require.ErrorIs(t, err, database.ErrMailFull)
	after, _, err := f.CharacterSnapshot(ctx, w.role.ID)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	newEvents, err := f.EventCount(ctx, w.role.ID)
	require.NoError(t, err)
	require.Equal(t, events, newEvents)
	// Free the bag instead of the mailbox: the original failed claim remains usable.
	bag, err := inventory.ReadBag(before)
	require.NoError(t, err)
	bag.Equipment = nil
	state, err := inventory.SaveBag(before, bag)
	require.NoError(t, err)
	require.NoError(t, f.SeedCharacterState(ctx, w.role.ID, state))
	w.role.State = state
	_, err = w.boostStepRequest(ctx, yuanmaRequest(9), 680)
	require.NoError(t, err)
}

func TestYuanmaOverflowPartialStackAndAccountMaterialsCommitTogether(t *testing.T) {
	w, f := yuanmaOverflowWorld(t, 11, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 121, Template: 590015966, Amount: 9500, ExpireTime: inventory.GrantExpireTime}}})
	ctx := context.Background()
	_, err := w.boostStepRequest(ctx, yuanmaRequest(11), 680)
	require.NoError(t, err)
	bag, err := inventory.ReadBag(w.role.State)
	require.NoError(t, err)
	require.EqualValues(t, 10000, bag.Items[0].Amount)
	msgs, err := f.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	var item inventory.MailItem
	require.NoError(t, json.Unmarshal(msgs[0].Assets[0].Item, &item))
	require.EqualValues(t, 500, item.Stack.Amount)
	counts, err := f.AccountMaterials(ctx, w.account)
	require.NoError(t, err)
	materials, err := inventory.ReadAccountMaterials(counts)
	require.NoError(t, err)
	require.EqualValues(t, 1000, materials.Count(3037))
	st, err := boostup.ReadState(w.role.State)
	require.NoError(t, err)
	require.True(t, st.Training.Finished)
}

func TestYuanmaOverflowConcurrentClaimAwardsOnce(t *testing.T) {
	w, f := yuanmaOverflowWorld(t, 9, inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}}})
	s := &workflow.LootService{Store: w.store, Loot: w.loot}
	ctx := context.Background()
	var group sync.WaitGroup
	results := make(chan error, 4)
	for i := 0; i < 4; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _, _, err := s.BoostStepRequest(ctx, w.role, w.boostup, 9, true)
			results <- err
		}()
	}
	group.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	msgs, err := f.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
}

func TestYuanmaOverflowBadSourceRollsBack(t *testing.T) {
	w, f := yuanmaOverflowWorld(t, 9, inventory.Bag{Version: "ordinary-bag-v1", Equipment: []inventory.BagEquipment{{Slot: 9, Template: 20002}}})
	w.loot.Equipment = nil
	_, err := w.boostStepRequest(context.Background(), yuanmaRequest(9), 680)
	require.Error(t, err)
	require.False(t, errors.Is(err, database.ErrMailFull))
	msgs, err := f.Mailbox(context.Background(), w.account, w.role.ID)
	require.NoError(t, err)
	require.Empty(t, msgs)
}

func TestYuanmaOverflowLateAccountMaterialFailureLeavesNoMail(t *testing.T) {
	w, f := yuanmaOverflowWorld(t, 11, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 121, Template: 590015966, Amount: 10000, ExpireTime: inventory.GrantExpireTime}}})
	ctx := context.Background()
	_, _, _, err := f.CommitAccountMaterialEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, "fill-shared", "fixture", func(cur database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		m, err := inventory.ReadAccountMaterials(counts)
		if err != nil {
			return nil, nil, err
		}
		m, _, err = m.Add(3037, ^uint32(0))
		if err != nil {
			return nil, nil, err
		}
		next, err := m.Save()
		return cur.State, next, err
	})
	require.NoError(t, err)
	before, _, err := f.CharacterSnapshot(ctx, w.role.ID)
	require.NoError(t, err)
	_, err = w.boostStepRequest(ctx, yuanmaRequest(11), 680)
	require.Error(t, err)
	after, _, err := f.CharacterSnapshot(ctx, w.role.ID)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	msgs, err := f.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Empty(t, msgs)
}

func TestYuanmaOverflowSecondMailFailureRollsBackFirst(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1"}
	for i := uint16(9); i <= 20; i++ {
		bag.Equipment = append(bag.Equipment, inventory.BagEquipment{Slot: i, Template: 20002})
	}
	w, f := yuanmaOverflowWorld(t, 9, bag)
	ctx := context.Background()
	w.loot.BagRules.EquipmentSlots = [2]uint16{9, 20}
	boxes := autoBoxCatalog()
	definition := boxes.Definitions[9]
	definition.Pools[0].Candidates[0].Count = 12
	boxes.Definitions[9] = definition
	w.loot.RewardBoxes = boosterBoxSource{catalog: boxes}
	for i := 0; i < 254; i++ {
		_, _, err := f.CommitSystemMail(ctx, w.account, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("fill:%d", i), "fixture", "Fill", "Fill", nil)
		require.NoError(t, err)
	}
	_, err := w.boostStepRequest(ctx, yuanmaRequest(9), 680)
	require.ErrorIs(t, err, database.ErrMailFull)
	msgs, err := f.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 254)
	for _, msg := range msgs {
		require.Empty(t, msg.Assets)
	}
	state, _, err := f.CharacterSnapshot(ctx, w.role.ID)
	require.NoError(t, err)
	st, err := boostup.ReadState(state)
	require.NoError(t, err)
	require.False(t, st.Training.Claimed[9])
}
