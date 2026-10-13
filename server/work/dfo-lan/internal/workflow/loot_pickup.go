package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"errors"
	"fmt"
)

func (s *LootService) Pickup(ctx context.Context, role database.Character, session *loot.Session, d *dungeon.Session, r protocol.PickupRequest) (database.Character, loot.PickupReceipt, bool, error) {
	plan, e := s.Loot.PlanPickup(LootRole(role), session, d, r)
	if e != nil {
		return role, loot.PickupReceipt{}, false, e
	}
	return s.pickupPlan(ctx, role, plan)
}

// AutoPickup 与手动拾取共用持久化事务、幂等回执及失败保留地面物品的语义。
func (s *LootService) AutoPickup(ctx context.Context, role database.Character, session *loot.Session, d *dungeon.Session, object uint32) (database.Character, loot.PickupReceipt, bool, error) {
	plan, err := s.Loot.PlanAutoPickup(LootRole(role), session, d, object)
	if err != nil {
		return role, loot.PickupReceipt{}, false, err
	}
	return s.pickupPlan(ctx, role, plan)
}

func (s *LootService) pickupPlan(ctx context.Context, role database.Character, plan loot.PickupPlan) (database.Character, loot.PickupReceipt, bool, error) {
	var result loot.PickupReceipt
	fail := func(e error) (database.Character, loot.PickupReceipt, bool, error) { return role, result, false, e }
	drop := plan.Drop
	if drop.BlackPurgatoryIndex != 0 {
		saved, receipt, applied, err := s.pickBlackPurgatoryBoss(ctx, role, drop.Run, drop.BlackPurgatoryIndex, drop.Award)
		if err != nil {
			return fail(err)
		}
		return saved, loot.PickupReceipt{Run: drop.Run, Map: drop.Map, Object: drop.Object, Award: receipt.Award, Destination: receipt.Destination, Source: s.Loot.Catalog.Source.SaveIdentity()}, applied, nil
	}
	key := fmt.Sprintf("pickup:%s:%d", drop.Run, drop.Object)
	saved, applied, e := s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, s.Loot.Rules.Model,
		func(tx *database.Tx, current database.Character) (json.RawMessage, json.RawMessage, error) {
			state, receipt, e := s.Loot.PreparePickup(LootRole(current), plan)
			if e == nil {
				return state, receipt, nil
			}
			if !isPickupCapacityError(e) {
				return nil, nil, e
			}
			return s.pickupToMail(ctx, tx, current, plan, e)
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &result); e != nil {
		return fail(e)
	}
	if result.Source != s.Loot.Catalog.Source.SaveIdentity() || result.Run != drop.Run || result.Object != drop.Object || result.Award != drop.Award {
		return fail(fmt.Errorf("pickup receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}

// 满包拾取的邮件署名与正文。业主定的是「背包满时拾取转邮箱」这条行为，当前 PVF 里
// 只有 `equipmenttransformsystem.cos` 的 `[full inventory mail title/contents]`（装备变换
// 专用文案），掉落拾取在源里没有对应文案键，所以这两行是本服的调服策略文本，不新增
// JSON 内容表；源日后给出拾取专用的文案键时就改读源。
const (
	pickupMailSender = "Field Loot"
	pickupMailBody   = "Your Inventory was full, so the item you picked up has been sent by mail."
)

// isPickupCapacityError 只把「放不下」认作满包。超过堆叠上限、目录里认不出的模板、
// 状态解不开等失败仍整笔拒绝 —— 不能把「源不认识这件物品」洗成一封寄出去的信。
func isPickupCapacityError(e error) bool {
	return errors.Is(e, inventory.ErrBagFull) || errors.Is(e, inventory.ErrEquipmentBagFull) || errors.Is(e, inventory.ErrPetContainerFull)
}

// pickupToMail 是满包拾取的落点：角色状态一格未动（原样返回 current.State），这一件在同
// 一事务里落成系统邮件附件，回执仍是一条 PickupReceipt（Mailed=true、Destination 不是背包格）。
// 因此重放走的还是原来那条按回执比对的幂等路径：事件键已存在就不会第二次入账、也不会第二封。
func (s *LootService) pickupToMail(ctx context.Context, tx *database.Tx, current database.Character, plan loot.PickupPlan, capacity error) (json.RawMessage, json.RawMessage, error) {
	drop := plan.Drop
	item, e := s.Loot.PreparePickupToMail(LootRole(current), plan)
	if e != nil {
		return nil, nil, fmt.Errorf("拾取转邮寄造附件失败（%v）: %w", capacity, e)
	}
	asset, e := database.SystemMailAssetFor(item)
	if e != nil {
		return nil, nil, e
	}
	mailID, e := tx.InsertSystemMail(ctx, pickupMailSender, pickupMailBody, []database.MailAsset{asset})
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(loot.PickupReceipt{Run: drop.Run, Map: drop.Map, Object: drop.Object, Award: drop.Award, Source: s.Loot.Catalog.Source.SaveIdentity(), Mailed: true, MailID: mailID})
	if e != nil {
		return nil, nil, e
	}
	return current.State, receipt, nil
}
