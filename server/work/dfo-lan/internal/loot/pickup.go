package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

type Service struct {
	// Explicit candidate policy: only auto-opened training rewards may mail
	// ordinary bag overflow. Other reward paths retain their existing rules.
	BoostAutoOverflowMail bool
	BlackPurgatory        *BlackPurgatoryRewards
	BleedingMine          *BleedingMineRewards
	Currency              *OdysseyCurrency
	Catalog               catalog.LootCatalog
	DropCatalog           catalog.LootCatalog
	Rules                 Rules
	BagRules              inventory.BagRules
	Tables                Tables
	Equipment             *inventory.EquipmentCatalog
	CardPolicy            *CardRules
	// Boxes 保存已导出的袖珍罐奖励与进度规则。
	Boxes *BoxCatalog
	// ChapterDrop 是章节最终领主的章节盒掉落（手册 P3 子项 3）。默认整表
	// enabled=false，禁用行连掷骰种子都不消耗；由 profile 显式开启。
	ChapterDrop *OdysseyChapterDrop
	// Attunement 是「调律之边界」（深渊）副本的专属奖励表，直接取自源
	// rewardboostinfo CTP。只对声明了 [dungeon index] 的副本生效，其它副本
	// 连掷骰种子都不消耗。
	Attunement *AttunementRewards
	// RewardBoxes 解析奖励包装（源的 [booster]）开一层会出什么。奖励表发出来的
	// 是包装本身，玩家该拿到的是包装里的东西，所以展开发生在掉落时；见
	// OpenRewardBoxes。为 nil 时包装原样落地，启动期会拦下这个组合。
	RewardBoxes RewardBoxSource
	// InstantlyOpenBoxes 是「调律之边界」**专用**的包装源：只把源里带 `[instantly open]`
	// 的 booster 交给服务端代开，其余 booster 原样落地让玩家自己在客户端开
	// （源标记判据见 catalog.BoosterDefinition.InstantlyOpen 与 next176 §19.3）。
	//
	// 与 RewardBoxes 分开是**故意的**：这条规则先在调律之边界验证，其它玩法沿用旧行为；
	// 为 nil 时一律用 RewardBoxes。
	InstantlyOpenBoxes RewardBoxSource
	// Omen 是千海之空深渊的征兆系统累积账（见 omen.go）。为 nil 时通关不推进。
	Omen *OmenLedger
	// ExtraDrop 是活动 1093「额外掉落奖励」：在 ENDKEEPER OF ORDER 副本里按掉落装备的
	// 品质额外给 Mystical Energy（见 extradrop.go）。为 nil 时整段跳过。
	ExtraDrop *ExtraDropRewards
}
type PickupReceipt struct {
	Run         string
	Map, Object uint32
	Award       Award
	Destination uint16
	Source      string
	// Mailed/MailID：拾取那一刻背包放不下，这一件改走系统邮件附件（MailID 是留审计的
	// 邮件编号），此时 Destination 不是背包格。旧回执里没有这两个键，读回来是零值。
	Mailed bool  `json:",omitempty"`
	MailID int64 `json:",omitempty"`
}

func (s *Service) Bootstrap(role Role) ([]byte, error) {
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return nil, fmt.Errorf("inventory source mismatch")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	return protocol.InventoryRestore(b.Rows(), b.Expansion)
}

type PickupPlan struct {
	Drop     Drop
	Catalog  catalog.LootCatalog
	BagRules inventory.BagRules
}

// PlanAutoPickup 保留本地自动拾取的场景、物品归属及目录校验，不检查距离。
func (s *Service) PlanAutoPickup(role Role, session *Session, d *dungeon.Session, object uint32) (PickupPlan, error) {
	if session == nil || session.Catalog.Source.Checksum != s.Catalog.Source.Checksum || session.Rules.Model != s.Rules.Model {
		return PickupPlan{}, (fmt.Errorf("pickup without loot session"))
	}
	drop, e := session.Owned(d, role.AccountID, role.ID, role.WireID, object)
	if e != nil {
		return PickupPlan{}, (e)
	}
	if drop.BlackPurgatoryIndex != 0 {
		return PickupPlan{Drop: drop}, nil
	}
	awardCatalog, bagRules := s.Catalog, s.BagRules
	if d.Definition.Odyssey && session.Currency != nil {
		// 与 Death 里的同一判据：`session.Currency.Source` 是内层真哈希，
		// 运行期对象之间比 Checksum（比 SaveIdentity() 会恒不等，见 session.go 的注释）。
		if s.Currency == nil || session.Currency.Source != s.Catalog.Source.Checksum || session.Currency.Model != s.Currency.Model {
			return PickupPlan{}, fmt.Errorf("currency pickup policy mismatch")
		}
		awardCatalog, bagRules = session.Currency.StorageCatalog(s.Catalog), session.Currency.BagRules(s.BagRules)
	}
	return PickupPlan{Drop: drop, Catalog: awardCatalog, BagRules: bagRules}, nil
}

func (s *Service) PlanPickup(role Role, session *Session, d *dungeon.Session, r protocol.PickupRequest) (PickupPlan, error) {
	plan, err := s.PlanAutoPickup(role, session, d, r.Object)
	if err != nil || plan.Drop.BlackPurgatoryIndex != 0 {
		return plan, err
	}
	distance := func(a, b uint16) int {
		n := int(a) - int(b)
		if n < 0 {
			return -n
		}
		return n
	}
	if distance(r.ActorX, r.DropX) > int(s.Rules.MaximumPickupX) || distance(r.ActorY, r.DropY) > int(s.Rules.MaximumPickupY) {
		return PickupPlan{}, (fmt.Errorf("pickup request coordinates are too far apart"))
	}
	return plan, nil
}
func (s *Service) PreparePickup(current Role, plan PickupPlan) (json.RawMessage, json.RawMessage, error) {
	var result PickupReceipt
	drop, awardCatalog, bagRules := plan.Drop, plan.Catalog, plan.BagRules
	b, e := inventory.ReadBag(current.State)
	if e != nil {
		return nil, nil, e
	}
	var slot uint16
	if drop.Award.Template != 0 && awardCatalog.Items[drop.Award.Template].Kind != "stackable" {
		// Gear goes to the bag's equipment range with its own durability,
		// not into a stackable category slot.
		var slots []uint16
		b, slots, e = b.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, drop.Award.Template, drop.Award.Amount)
		if e != nil {
			return nil, nil, e
		}
		slot = slots[0]
	} else if b, slot, e = b.Add(awardCatalog, bagRules, drop.Award.Template, drop.Award.Amount); e != nil {
		return nil, nil, e
	}
	updated, e := inventory.SaveBag(current.State, b)
	if e != nil {
		return nil, nil, e
	}
	result = PickupReceipt{Run: drop.Run, Map: drop.Map, Object: drop.Object, Award: drop.Award, Destination: slot, Source: s.Catalog.Source.SaveIdentity()}
	receipt, e := json.Marshal(result)
	return updated, receipt, e
}

// PreparePickupToMail 是背包放不下时的拾取出口：用与 PreparePickup 同一套判据决定这一件
// 是装备还是堆叠物，并按同一条实例规则（装备的耐久与训练封存）造出邮件附件。
// 背包一格都不动 —— 落地发生在邮件表里，所以这里不返回新的角色状态。
//
// 堆叠物按**主目录**（s.Catalog）复核，不看 plan.Catalog：领取邮件那一步用的就是
// `w.loot.Catalog`，而奥德赛货币只存在于会话的存储目录里，照 plan.Catalog 造附件会得到
// 一封领不出来的信。主目录认不出就报错，让这次拾取照旧整笔拒绝、物品留在地上。
//
// 金币（模板 0）永不进来：它有独立的计数、不存在容量问题。
func (s *Service) PreparePickupToMail(current Role, plan PickupPlan) (inventory.MailItem, error) {
	drop, awardCatalog := plan.Drop, plan.Catalog
	if drop.Award.Template < 2 || drop.Award.Amount == 0 {
		return inventory.MailItem{}, fmt.Errorf("pickup award cannot be mailed")
	}
	b, e := inventory.ReadBag(current.State)
	if e != nil {
		return inventory.MailItem{}, e
	}
	if awardCatalog.Items[drop.Award.Template].Kind != "stackable" {
		return b.MailEquipment(s.Equipment, drop.Award.Template)
	}
	if main, ok := s.Catalog.Items[drop.Award.Template]; !ok || main.Kind != "stackable" {
		return inventory.MailItem{}, fmt.Errorf("pickup award cannot be claimed from mail: %d", drop.Award.Template)
	}
	item := inventory.MailItem{Stack: &inventory.BagItem{Template: drop.Award.Template, Amount: drop.Award.Amount, ExpireTime: inventory.GrantExpireTime}}
	if _, e = item.Row(); e != nil {
		return inventory.MailItem{}, e
	}
	return item, nil
}
