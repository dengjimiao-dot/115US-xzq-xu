package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSymbolGrowthSQLiteReplayAccountSharingAndReload(t *testing.T) {
	ctx := context.Background()
	store, e := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "growth.sqlite")})
	require.NoError(t, e)
	t.Cleanup(store.Close)
	require.NoError(t, store.Migrate(ctx))
	require.NoError(t, store.MigrateCharacterEvents(ctx))
	require.NoError(t, store.MigrateAccountMaterials(ctx))
	source := strings.Repeat("a", 64)
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}, Items: map[uint32]catalog.LootItem{20001: {ID: 20001, Kind: "stackable", StackableType: "[avatar emblem]", StackLimit: 100, Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[rarity]"}, {Type: 0, Value: 2}}}}, 30001: {ID: 30001, Kind: "stackable", StackableType: "[material]", StackLimit: 100, Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[stack in account common item inventory]"}}}}}}
	rules := &inventory.SymbolEmblemRules{SourceChecksum: source, CostItem: 30001, Tiers: map[inventory.SymbolTierKey]inventory.SymbolTier{}, Fixed: map[uint32]inventory.SymbolOptions{1: {}}}
	for typ := uint32(0); typ < 5; typ++ {
		rules.Tiers[inventory.SymbolTierKey{Type: typ, Grade: 0}] = inventory.SymbolTier{Levels: map[uint32]inventory.SymbolLevel{1: {OptionIndex: 1}}}
	}
	layout := &inventory.SymbolSocketLayout{Source: source, Sockets: map[string]inventory.SymbolSocketGroup{"red_symbol": {Count: 4, Mask: 1}}, Rules: rules, Growth: &inventory.EmblemGrowthRules{Source: source, Normal: map[uint32]inventory.EmblemUpgradeRule{2: {Results: []inventory.EmblemGradeWeight{{Grade: 2, Weight: 10000}}}}, Disjoint: map[uint32]inventory.EmblemDisjointRule{2: {Template: 30001, Min: 3, Max: 3}}}}
	service := &ItemService{Store: store, Items: &inventory.ItemService{Catalog: c, BagRules: inventory.BagRules{Source: source, MissingStackLimit: 100, Slots: map[string][2]uint16{"[avatar emblem]": {289, 290}}}, SymbolSockets: layout}}
	account, e := store.DevelopmentAccount(ctx, "symbol-growth-fixture")
	require.NoError(t, e)
	initial := json.RawMessage(`{"level":115,"future_role":{"keep":true},"inventory":{"version":"ordinary-bag-v1","gold":777,"future_inventory":{"keep":true},"items":[{"slot":289,"Template":20001,"Amount":2}]}}`)
	create := func(name string) database.Character {
		role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, ConfigVersion: c.Source.SaveIdentity(), State: initial, Request: []byte{}}, 24)
		require.NoError(t, e)
		return role
	}
	role, other := create("growth-a"), create("growth-b")
	req := inventory.SymbolGrowthRequest{Source: source, Operation: inventory.SymbolDisjoint, Slot: 289, Template: 20001, Amount: 1}
	saved, receipt, applied, e := service.ApplySymbolGrowth(ctx, role, req, time.Unix(100, 0))
	require.NoError(t, e)
	require.True(t, applied)
	require.Equal(t, req, receipt.Request)
	replay, receipt, applied, e := service.ApplySymbolGrowth(ctx, saved, req, time.Unix(101, 0))
	require.NoError(t, e)
	require.False(t, applied)
	require.JSONEq(t, string(saved.State), string(replay.State))
	require.Equal(t, req, receipt.Request)
	_, _, applied, e = service.ApplySymbolGrowth(ctx, other, req, time.Unix(102, 0))
	require.NoError(t, e)
	require.True(t, applied)
	counts, e := store.AccountMaterials(ctx, account)
	require.NoError(t, e)
	m, e := inventory.ReadAccountMaterials(counts)
	require.NoError(t, e)
	require.Equal(t, uint32(6), m.CommonItems[30001])
	// Simulate an old writer whose AccountMaterials struct has no CN fields.
	_, _, e = store.CommitAccountMaterialSweep(ctx, account, role.ID, c.Source.SaveIdentity(), func(current database.Character, _ json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		return current.State, json.RawMessage(`{"version":"account-materials-v1","counts":{"363":2}}`), nil
	})
	require.NoError(t, e)
	counts, e = store.AccountMaterials(ctx, account)
	require.NoError(t, e)
	m, e = inventory.ReadAccountMaterials(counts)
	require.NoError(t, e)
	require.Equal(t, uint32(6), m.CommonItems[30001])
	require.Equal(t, uint32(2), m.Counts[363])
	fresh, e := store.AdminCharacter(ctx, role.ID)
	require.NoError(t, e)
	slots, e := inventory.ReadSymbolSockets(fresh.State, layout)
	require.NoError(t, e)
	require.Equal(t, uint64(1), slots.Sequence)
	// A distinct stale request cannot reuse a committed sequence.
	stale := req
	stale.Amount = 2
	_, _, _, e = service.ApplySymbolGrowth(ctx, fresh, stale, time.Unix(103, 0))
	require.Error(t, e)
	after, e := store.AccountMaterials(ctx, account)
	require.NoError(t, e)
	require.JSONEq(t, string(counts), string(after))
	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fresh.State, &fields))
	require.JSONEq(t, `{"keep":true}`, string(fields["future_role"]))
	var bagFields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fields["inventory"], &bagFields))
	require.JSONEq(t, `{"keep":true}`, string(bagFields["future_inventory"]))
}
