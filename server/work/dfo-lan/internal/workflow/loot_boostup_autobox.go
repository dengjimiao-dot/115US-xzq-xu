package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"dfolan/internal/loot"
	"dfolan/internal/mail"
	"encoding/json"
	"fmt"
)

// boostAutoStep 编排「毕业一步自动开盒」的账户材料事务（§7.2 E13）；
// 展开与发放纯计算在 loot.PrepareBoostAutoStep。
//
// 事务复用既有的 CommitAccountMaterialEvent：它已经把角色行与同账号的
// account_material_storage 行一起 FOR UPDATE，事件键 boostup-reward:<step>
// 就是幂等闸门，重放不会二次发放，也不会让两个角色的材料入账互相覆盖。
// 默认满包回滚；显式邮件策略下，溢出邮件在本事务内插入，保留同一个事件键。
func (s *LootService) boostAutoStep(ctx context.Context, role database.Character, c *boostup.Catalog, step byte) (database.Character, loot.BoostStepReceipt, bool, error) {
	var receipt loot.BoostStepReceipt
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, receipt, false, fmt.Errorf("auto step storage missing")
	}
	key := fmt.Sprintf("boostup-reward:%d", step)
	prepare := func(tx *database.Tx, cur database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		state, materials, out, err := s.Loot.PrepareBoostAutoStep(LootRole(cur), c, step, counts)
		if err != nil {
			return nil, nil, err
		}
		if out.Step != step || !out.Claim || !out.AutoOpened || len(out.Rewards) == 0 || len(out.Granted) == 0 {
			return nil, nil, fmt.Errorf("auto step landing receipt mismatch")
		}
		var assets []database.MailAsset
		for _, item := range out.MailItems {
			if _, err := item.Row(); err != nil {
				return nil, nil, err
			}
			body, err := json.Marshal(item)
			if err != nil {
				return nil, nil, err
			}
			assets = append(assets, database.MailAsset{Item: body})
		}
		for len(assets) > 0 {
			if tx == nil {
				return nil, nil, fmt.Errorf("auto overflow mail requires the claim transaction")
			}
			count := min(len(assets), mail.MaxAttachments)
			id, err := tx.SendSystemMail(ctx, "Starter Boost", "Inventory overflow rewards", assets[:count])
			if err != nil {
				return nil, nil, err
			}
			out.MailIDs = append(out.MailIDs, id)
			assets = assets[count:]
		}
		out.MailItems = nil
		receipt = out
		return state, materials, nil
	}
	var next database.Character
	var applied bool
	var e error
	if s.Loot.BoostAutoOverflowMail {
		next, _, applied, e = s.Store.CommitAccountMaterialEventTx(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-step-v1", prepare)
	} else {
		next, _, applied, e = s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-step-v1", func(cur database.Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			return prepare(nil, cur, counts)
		})
	}
	if e != nil {
		return role, receipt, false, e
	}
	if applied {
		if receipt.Step != step || !receipt.Claim || !receipt.AutoOpened || len(receipt.Rewards) == 0 || len(receipt.Granted) == 0 {
			return role, receipt, false, fmt.Errorf("auto step landing receipt mismatch")
		}
		next.WireID = role.WireID
		return next, receipt, true, nil
	}
	// 重放：这一步早就提交过了。发放明细不从历史回执里猜（旧树的回执形状与
	// 本树不同），只如实报告「已领」，由调用方按整包 inventory bootstrap 回给
	// 客户端，账户材料以存储里的实际内容为准。
	next.WireID = role.WireID
	return next, loot.BoostStepReceipt{Step: step, Claim: true, AutoOpened: true}, false, nil
}
