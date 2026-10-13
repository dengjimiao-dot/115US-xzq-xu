package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"errors"
	"testing"
)

const capacityChecksum = "0123456789abcdef0123456789abcdef"

func capacityBagRules() BagRules {
	return BagRules{
		Model:             "capacity-test-v1",
		Source:            capacityChecksum,
		Slots:             map[string][2]uint16{"[etc]": {5, 5}},
		EquipmentSlots:    [2]uint16{20, 20},
		MissingStackLimit: 1000,
	}
}

func capacityCatalog() catalog.LootCatalog {
	return catalog.LootCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: capacityChecksum},
		Items: map[uint32]catalog.LootItem{
			100: {ID: 100, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000},
			200: {ID: 200, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000},
		},
	}
}

func capacityGearCatalog() *EquipmentCatalog {
	return &EquipmentCatalog{index: map[uint32]EquipmentDefinition{
		300: {ID: 300, Fields: map[string][]pvf.Token{
			"[rarity]":          {{Type: 0, Value: 6}},
			"[equipment type]":  {{Type: 3, Text: "[weapon]"}},
			"[durability]":      {{Type: 0, Value: 88}},
			"[minimum level]":   {{Type: 0, Value: 1}},
			"[attack speed]":    {{Type: 0, Value: 0}},
			"[physical attack]": {{Type: 0, Value: 0}},
		}},
	}}
}

// 满包必须能被调用方用 errors.Is 认出来（拾取据此把这一件转邮箱），
// 而「同模板可合并」「超过堆叠上限」两类不能被判成满包。
func TestAddCapacitySentinel(t *testing.T) {
	rules, items := capacityBagRules(), capacityCatalog()
	if _, _, err := (Bag{Items: []BagItem{{Slot: 5, Template: 200, Amount: 1}}}).Add(items, rules, 100, 1); !errors.Is(err, ErrBagFull) {
		t.Fatalf("满包没有回容量哨兵：%v", err)
	}
	if _, _, err := (Bag{Items: []BagItem{{Slot: 5, Template: 100, Amount: 1}}}).Add(items, rules, 100, 1); err != nil {
		t.Fatalf("同模板合并被误判成满包：%v", err)
	}
	if _, _, err := (Bag{}).Add(items, rules, 100, 5000); errors.Is(err, ErrBagFull) {
		t.Fatal("超过堆叠上限被当成满包")
	}
}

// 装备区放不下走另一个哨兵；MailEquipment 造的实例与直接入包那一件同规则
// （耐久都取 Reward），否则玩家从邮箱领到的会是一件不同的装备。
func TestEquipmentCapacitySentinelAndMailInstance(t *testing.T) {
	rules, gear := capacityBagRules(), capacityGearCatalog()
	full := Bag{Equipment: []BagEquipment{{Slot: 20, Template: 300, Durability: 1}}}
	if _, _, err := full.AddEquipment(gear, rules.EquipmentSlots, 300, 1); !errors.Is(err, ErrEquipmentBagFull) {
		t.Fatalf("装备区已满没有回容量哨兵：%v", err)
	}
	granted, slots, err := (Bag{}).AddEquipment(gear, rules.EquipmentSlots, 300, 1)
	if err != nil {
		t.Fatal(err)
	}
	mail, err := (Bag{}).MailEquipment(gear, 300)
	if err != nil {
		t.Fatal(err)
	}
	if mail.Equipment == nil || mail.Equipment.Template != 300 {
		t.Fatalf("邮件附件不是这件装备：%+v", mail)
	}
	if mail.Equipment.Durability != granted.Equipment[0].Durability || granted.Equipment[0].Slot != slots[0] {
		t.Fatalf("邮寄实例与入包实例不一致：mail=%+v bag=%+v", mail.Equipment, granted.Equipment[0])
	}
	if _, err := (Bag{}).MailEquipment(nil, 300); err == nil {
		t.Fatal("没有装备目录时不该造出邮件附件")
	}
}
