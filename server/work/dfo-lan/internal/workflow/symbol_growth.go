package workflow

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

func (s *ItemService) ApplySymbolGrowth(ctx context.Context, role database.Character, req inventory.SymbolGrowthRequest, now time.Time) (database.Character, inventory.SymbolGrowthReceipt, bool, error) {
	var receipt inventory.SymbolGrowthReceipt
	if s == nil || s.Store == nil || s.Items == nil {
		return role, receipt, false, fmt.Errorf("星徽服务不可用")
	}
	identity, e := json.Marshal(req)
	if e != nil {
		return role, receipt, false, e
	}
	key := fmt.Sprintf("symbol-growth:%d:%x", req.Sequence, sha256.Sum256(identity))
	saved, _, applied, e := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, "symbol-growth-v1", func(current database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		updated, next, out, e := s.Items.PrepareSymbolGrowth(current.State, counts, req, now.Unix(), inventory.SymbolGrowthDraw)
		receipt = out
		return updated, next, e
	})
	if e != nil {
		return role, receipt, false, e
	}
	// The immutable transaction outcome carries the exact request, even after
	// later operations replace the account's latest receipt or advance a color.
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, receipt, false, e
	}
	materials, e := inventory.ReadAccountMaterials(raw)
	if e != nil {
		return role, receipt, false, e
	}
	if e = json.Unmarshal(materials.SymbolReceipt, &receipt); e != nil {
		return role, receipt, false, e
	}
	if receipt.Request != req {
		return role, receipt, false, fmt.Errorf("星徽回执冲突")
	}
	saved.WireID = role.WireID
	return saved, receipt, applied, nil
}
