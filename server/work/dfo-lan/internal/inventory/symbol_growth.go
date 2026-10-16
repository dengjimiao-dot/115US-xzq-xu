package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

const (
	SymbolGenerateLights uint16 = 1 + iota
	SymbolResonate
	SymbolDisjoint
	SymbolNormalUpgrade
	SymbolSpecialUpgrade
	SymbolGradeTicket
	SymbolChangeEmblem
	SymbolGrowthPurchase
)

type SymbolGrowthRequest struct {
	Source                     string `json:"source"`
	Sequence                   uint64 `json:"sequence"`
	Operation                  uint16 `json:"operation"`
	Slot                       uint16 `json:"slot"`
	Template, Argument, Amount uint32
}
type SymbolGrowthVisual struct {
	Operation, Outcome uint16
	Item, Amount       uint32
}
type SymbolGrowthState struct {
	Visual     *SymbolGrowthVisual `json:"visual,omitempty"`
	Points     map[uint32]uint32   `json:"points,omitempty"`
	Lights     []uint32            `json:"lights,omitempty"`
	LastResult string              `json:"last_result,omitempty"`
}
type SymbolGrowthReceipt struct {
	Request SymbolGrowthRequest
	Result  string
}

func ReadSymbolGrowth(state json.RawMessage) (SymbolGrowthState, error) {
	var fields map[string]json.RawMessage
	var out SymbolGrowthState
	if e := json.Unmarshal(state, &fields); e != nil {
		return out, e
	}
	if raw := fields["symbol_growth"]; len(raw) > 0 {
		if e := json.Unmarshal(raw, &out); e != nil {
			return out, e
		}
	}
	if out.Points == nil {
		out.Points = map[uint32]uint32{}
	}
	for grade, points := range out.Points {
		if points > 100 || grade == 0 {
			return out, fmt.Errorf("invalid saved breakthrough points")
		}
	}
	if len(out.Lights) != 0 && len(out.Lights) != 10 {
		return out, fmt.Errorf("invalid saved resonance light count")
	}
	for _, color := range out.Lights {
		if color > 4 {
			return out, fmt.Errorf("invalid saved resonance light color")
		}
	}
	return out, nil
}
func symbolItemNumbers(c catalog.LootCatalog, id uint32, name string) ([]uint32, error) {
	script, e := c.ItemScript(id)
	if e != nil {
		return nil, e
	}
	var out []uint32
	active := false
	for _, t := range script.Cells {
		if t.Type == 3 {
			if active {
				break
			}
			active = t.Text == name
			continue
		}
		if active {
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid item %d field %s", id, name)
			}
			out = append(out, uint32(t.Value))
		}
	}
	return out, nil
}
func symbolCommonItem(c catalog.LootCatalog, id uint32) (bool, error) {
	script, e := c.ItemScript(id)
	if e != nil {
		return false, e
	}
	for _, t := range script.Cells {
		if t.Type == 3 && t.Text == "[stack in account common item inventory]" {
			return true, nil
		}
	}
	return false, nil
}
func symbolItemScalar(c catalog.LootCatalog, id uint32, name string) (uint32, error) {
	v, e := symbolItemNumbers(c, id, name)
	if e != nil || len(v) != 1 {
		return 0, fmt.Errorf("item %d %s missing", id, name)
	}
	return v[0], nil
}
func symbolSpend(b *Bag, m *AccountMaterials, c catalog.LootCatalog, id, n uint32, now int64) error {
	if n == 0 {
		return nil
	}
	if id == 0 {
		if b.Gold < n {
			return fmt.Errorf("金币不足")
		}
		b.Gold -= n
		return nil
	}
	common, e := symbolCommonItem(c, id)
	if e != nil {
		return e
	}
	if common {
		if m.CommonItems[id] < n {
			return fmt.Errorf("账号材料不足")
		}
		m.CommonItems[id] -= n
		return nil
	}
	if _, _, ok := AccountMaterialTarget(id); ok {
		next, _, e := m.Spend(id, n)
		if e != nil {
			return e
		}
		*m = next
		return nil
	}
	var available uint64
	for _, i := range b.Items {
		if i.Template == id && !protocol.StoredItemExpired(i.ExpireTime, now) {
			available += uint64(i.Amount)
		}
	}
	if available < uint64(n) {
		return fmt.Errorf("材料不足")
	}
	for j := 0; j < len(b.Items) && n > 0; {
		i := &b.Items[j]
		if i.Template != id || protocol.StoredItemExpired(i.ExpireTime, now) {
			j++
			continue
		}
		used := min(n, i.Amount)
		i.Amount -= used
		n -= used
		if i.Amount == 0 {
			b.Items = append(b.Items[:j], b.Items[j+1:]...)
		} else {
			j++
		}
	}
	return nil
}
func symbolAward(b *Bag, m *AccountMaterials, c catalog.LootCatalog, r BagRules, id, n, expiry uint32) error {
	common, e := symbolCommonItem(c, id)
	if e != nil {
		return e
	}
	if common {
		if uint64(m.CommonItems[id])+uint64(n) > math.MaxUint32 {
			return fmt.Errorf("账号材料溢出")
		}
		m.CommonItems[id] += n
		return nil
	}
	// New rewards never inherit an old expiring stack's lifetime.
	for _, i := range b.Items {
		if i.Template == id && i.ExpireTime != expiry {
			return fmt.Errorf("不同期限的同类物品请先整理")
		}
	}
	next, _, e := b.Add(c, r, id, n, expiry)
	if e != nil {
		return e
	}
	*b = next
	return nil
}
func symbolTake(b *Bag, slot uint16, id, n uint32, now int64) (BagItem, error) {
	for j, i := range b.Items {
		if i.Slot != slot {
			continue
		}
		if i.Template != id || i.Amount < n || n == 0 || protocol.StoredItemExpired(i.ExpireTime, now) {
			return BagItem{}, fmt.Errorf("徽章已变化或数量不足")
		}
		b.Items[j].Amount -= n
		if b.Items[j].Amount == 0 {
			b.Items = append(b.Items[:j], b.Items[j+1:]...)
		}
		return i, nil
	}
	return BagItem{}, fmt.Errorf("徽章不在指定槽位")
}
func symbolNext(r *SymbolEmblemRules, typ uint32, p SymbolProgress) (SymbolProgress, bool) {
	levels := []SymbolProgress{}
	for k, t := range r.Tiers {
		if k.Type == typ {
			for l := range t.Levels {
				if k.Grade > p.Grade || k.Grade == p.Grade && l > p.Level {
					levels = append(levels, SymbolProgress{k.Grade, l})
				}
			}
		}
	}
	sort.Slice(levels, func(i, j int) bool {
		return levels[i].Grade < levels[j].Grade || levels[i].Grade == levels[j].Grade && levels[i].Level < levels[j].Level
	})
	if len(levels) == 0 {
		return p, false
	}
	return levels[0], true
}
func symbolRateUnit(r *EmblemGrowthRules) (uint32, error) {
	var unit uint64
	for _, v := range r.Normal {
		var n uint64
		for _, p := range v.Results {
			n += uint64(p.Weight)
		}
		if unit != 0 && unit != n {
			return 0, fmt.Errorf("徽章概率单位不一致")
		}
		unit = n
	}
	if unit == 0 || unit > math.MaxUint32 {
		return 0, fmt.Errorf("概率单位缺失")
	}
	return uint32(unit), nil
}
func symbolDraw(draw func(uint32) (uint32, error), limit uint32) (uint32, error) {
	v, e := draw(limit)
	if e != nil {
		return 0, e
	}
	if limit == 0 || v >= limit {
		return 0, fmt.Errorf("invalid growth entropy")
	}
	return v, nil
}

// Pure preparation. All three documents are committed by one SQLite transaction.
func (s *ItemService) PrepareSymbolGrowth(state, counts json.RawMessage, req SymbolGrowthRequest, now int64, draw func(uint32) (uint32, error)) (json.RawMessage, json.RawMessage, SymbolGrowthReceipt, error) {
	var receipt SymbolGrowthReceipt
	failure := func(e error) (json.RawMessage, json.RawMessage, SymbolGrowthReceipt, error) {
		return nil, nil, receipt, e
	}
	l := s.SymbolSockets
	if l == nil || l.Rules == nil || l.Growth == nil || l.Source != s.Catalog.Source.Checksum || req.Source != l.Source || l.Growth.Source != l.Source || s.BagRules.Source != l.Source {
		return failure(fmt.Errorf("星徽养成源不匹配"))
	}
	slots, e := ReadSymbolSockets(state, l)
	if e != nil {
		return failure(e)
	}
	if slots.Sequence != req.Sequence || slots.Sequence == math.MaxUint64 {
		return failure(fmt.Errorf("星徽操作已过期"))
	}
	growth, e := ReadSymbolGrowth(state)
	if e != nil {
		return failure(e)
	}
	bag, e := ReadBag(state)
	if e != nil {
		return failure(e)
	}
	bag.Items = append([]BagItem(nil), bag.Items...)
	materials, e := ReadAccountMaterials(counts)
	if e != nil {
		return failure(e)
	}
	shared := map[uint32]uint32{}
	for id, n := range materials.CommonItems {
		shared[id] = n
	}
	materials.CommonItems = shared
	// Sweep only source-marked common materials participating in this system.
	allowed := map[uint32]bool{l.Rules.CostItem: true, l.Rules.Tuning.FailRewardItem: true}
	for _, d := range l.Growth.Disjoint {
		allowed[d.Template] = true
	}
	for j := 0; j < len(bag.Items); {
		i := bag.Items[j]
		if !allowed[i.Template] || protocol.StoredItemExpired(i.ExpireTime, now) {
			j++
			continue
		}
		common, e := symbolCommonItem(s.Catalog, i.Template)
		if e != nil {
			return failure(e)
		}
		if !common {
			j++
			continue
		}
		if i.ExpireTime != 0 {
			return failure(fmt.Errorf("限时材料不能进入账号共用栏"))
		}
		if uint64(materials.CommonItems[i.Template])+uint64(i.Amount) > math.MaxUint32 {
			return failure(fmt.Errorf("账号材料溢出"))
		}
		materials.CommonItems[i.Template] += i.Amount
		bag.Items = append(bag.Items[:j], bag.Items[j+1:]...)
	}
	unit, e := symbolRateUnit(l.Growth)
	if e != nil {
		return failure(e)
	}
	result := ""
	visual := SymbolGrowthVisual{Operation: req.Operation, Outcome: 2}
	switch req.Operation {
	case SymbolGenerateLights:
		if len(growth.Lights) != 0 {
			return failure(fmt.Errorf("请先完成当前共鸣"))
		}
		var colors []uint32
		var weights []uint32
		var total uint64
		for typ := uint32(0); typ < 4; typ++ {
			name := symbolColorName(typ)
			p := slots.Progress[name]
			if _, ok := symbolNext(l.Rules, typ, p); !ok {
				continue
			}
			v, ok := l.Rules.Steps[SymbolStepKey{p.Grade, p.Level}]
			if !ok || v.ButtonWeight == 0 {
				return failure(fmt.Errorf("共鸣权重缺失"))
			}
			colors = append(colors, typ)
			weights = append(weights, v.ButtonWeight)
			total += uint64(v.ButtonWeight)
		}
		if total == 0 || total > math.MaxUint32 {
			return failure(fmt.Errorf("所有星徽已满级"))
		}
		for i := 0; i < 10; i++ {
			v, e := symbolDraw(draw, unit)
			if e != nil {
				return failure(e)
			}
			if v < l.Rules.Tuning.MultiColorAppearanceRate {
				growth.Lights = append(growth.Lights, 4)
				continue
			}
			v, e = symbolDraw(draw, uint32(total))
			if e != nil {
				return failure(e)
			}
			for j, w := range weights {
				if v < w {
					growth.Lights = append(growth.Lights, colors[j])
					break
				}
				v -= w
			}
		}
		result = "共鸣之光已生成，请选择颜色"
	case SymbolResonate:
		if len(growth.Lights) != 10 || req.Argument >= uint32(len(growth.Lights)) {
			return failure(fmt.Errorf("共鸣之光已失效"))
		}
		selected := growth.Lights[req.Argument]
		if selected > 4 {
			return failure(fmt.Errorf("无效共鸣颜色"))
		}
		var duplicates [5]uint32
		for _, color := range growth.Lights {
			if color > 4 {
				return failure(fmt.Errorf("无效共鸣之光"))
			}
			duplicates[color]++
		}
		target := selected
		if selected == 4 {
			target = math.MaxUint32
			for typ := uint32(0); typ < 4; typ++ {
				a := slots.Progress[symbolColorName(typ)]
				if _, ok := symbolNext(l.Rules, typ, a); !ok {
					continue
				}
				if target == math.MaxUint32 {
					target = typ
					continue
				}
				b := slots.Progress[symbolColorName(target)]
				if a.Grade > b.Grade || a.Grade == b.Grade && a.Level > b.Level {
					target = typ
				}
			}
			if target == math.MaxUint32 {
				return failure(fmt.Errorf("所有星徽已满级"))
			}
		}
		p := slots.Progress[symbolColorName(target)]
		step, ok := l.Rules.Steps[SymbolStepKey{p.Grade, p.Level}]
		if !ok {
			if selected != 4 {
				return failure(fmt.Errorf("星徽已满级"))
			}
			step.SuccessRate = 0
		}
		cost := step.Cost
		if selected == 4 {
			cost = l.Rules.Tuning.MultiColorCost
		}
		if e = symbolSpend(&bag, &materials, s.Catalog, l.Rules.CostItem, cost, now); e != nil {
			return failure(e)
		}
		rate := symbolResonanceSuccessRate(step, l.Rules.Tuning, selected, duplicates[selected], unit)
		v, e := symbolDraw(draw, unit)
		if e != nil {
			return failure(e)
		}
		if uint64(v) >= rate {
			if e = symbolAward(&bag, &materials, s.Catalog, s.BagRules, l.Rules.Tuning.FailRewardItem, l.Rules.Tuning.FailRewardCount, 0); e != nil {
				return failure(e)
			}
			visual.Outcome = 1
			visual.Item = l.Rules.Tuning.FailRewardItem
			visual.Amount = l.Rules.Tuning.FailRewardCount
			result = "共鸣失败，获得散落的五曜星痕"
		} else {
			advanced := uint32(0)
			for typ := uint32(0); typ < 4; typ++ {
				if selected != 4 && typ != selected {
					continue
				}
				name := symbolColorName(typ)
				p := slots.Progress[name]
				limit := duplicates[typ] + duplicates[4]
				if limit == 0 {
					limit = 1
				}
				n := uint32(0)
				for n < limit {
					if n > 0 {
						combo, ok := l.Rules.Tuning.ComboRates[n]
						if !ok {
							break
						}
						v, e := symbolDraw(draw, unit)
						if e != nil {
							return failure(e)
						}
						if v >= combo {
							break
						}
					}
					next, ok := symbolNext(l.Rules, typ, p)
					if !ok {
						break
					}
					p = next
					n++
				}
				slots.Progress[name] = p
				advanced += n
			}
			if advanced > 1 {
				visual.Outcome = 3
			}
			visual.Amount = advanced
			result = fmt.Sprintf("共鸣成功，共提升%d阶段", advanced)
		}
		growth.Lights = nil
	case SymbolDisjoint:
		if req.Amount == 0 || req.Amount > 1024 {
			return failure(fmt.Errorf("请选择1至1024个徽章"))
		}
		definition, ok := s.Catalog.Items[req.Template]
		if !ok || definition.StackableType != "[avatar emblem]" {
			return failure(fmt.Errorf("只能分解徽章"))
		}
		rarity, e := symbolItemScalar(s.Catalog, req.Template, "[rarity]")
		if e != nil {
			return failure(e)
		}
		rule, ok := l.Growth.Disjoint[rarity]
		if !ok {
			return failure(fmt.Errorf("此品质无法分解"))
		}
		if _, e = symbolTake(&bag, req.Slot, req.Template, req.Amount, now); e != nil {
			return failure(e)
		}
		var award uint64
		for i := uint32(0); i < req.Amount; i++ {
			v, e := symbolDraw(draw, rule.Max-rule.Min+1)
			if e != nil {
				return failure(e)
			}
			award += uint64(rule.Min + v)
		}
		if award > math.MaxUint32 {
			return failure(fmt.Errorf("分解产出溢出"))
		}
		if e = symbolAward(&bag, &materials, s.Catalog, s.BagRules, rule.Template, uint32(award), 0); e != nil {
			return failure(e)
		}
		visual.Item = rule.Template
		visual.Amount = uint32(award)
		result = fmt.Sprintf("分解完成，获得%d个材料", award)
	case SymbolNormalUpgrade, SymbolSpecialUpgrade:
		groupIndex, ok := l.Growth.ByItem[req.Template]
		if !ok {
			return failure(fmt.Errorf("此徽章不在突破源表"))
		}
		grade, e := symbolItemScalar(s.Catalog, req.Template, "[grade]")
		if e != nil {
			return failure(e)
		}
		rules := l.Growth.Normal
		quantity := uint32(1)
		if req.Operation == SymbolSpecialUpgrade {
			rules = l.Growth.Special
			quantity = 2
		}
		rule, ok := rules[grade]
		if !ok {
			return failure(fmt.Errorf("此品质无法突破"))
		}
		// A request names a source stack; special breakthrough requires two identical items.
		item, e := symbolTake(&bag, req.Slot, req.Template, 1, now)
		if e != nil {
			return failure(e)
		}
		if quantity == 2 {
			found := false
			for _, other := range bag.Items {
				if other.Template == req.Template && other.ExpireTime == item.ExpireTime && other.Amount > 0 {
					if _, e = symbolTake(&bag, other.Slot, req.Template, 1, now); e != nil {
						return failure(e)
					}
					found = true
					break
				}
			}
			if !found {
				return failure(fmt.Errorf("必定突破需要两个相同徽章"))
			}
		}
		fee := rule.Material
		if req.Argument != 0 {
			if req.Argument != 1 || req.Operation != SymbolSpecialUpgrade || rule.Replace.Template == 0 {
				return failure(fmt.Errorf("无替代材料"))
			}
			fee = rule.Replace
		}
		if e = symbolSpend(&bag, &materials, s.Catalog, fee.Template, fee.Amount, now); e != nil {
			return failure(e)
		}
		target := grade
		guarantee := l.Growth.Guarantee[grade]
		// Source point ranges are percentages; reaching a full gauge guarantees the next grade.
		if req.Operation == SymbolNormalUpgrade && growth.Points[grade] >= 100 {
			target = guarantee.NextGrade
		} else {
			var total uint32
			for _, p := range rule.Results {
				total += p.Weight
			}
			v, e := symbolDraw(draw, total)
			if e != nil {
				return failure(e)
			}
			for _, p := range rule.Results {
				if v < p.Weight {
					target = p.Grade
					break
				}
				v -= p.Weight
			}
		}
		if target == grade {
			if req.Operation == SymbolNormalUpgrade {
				if guarantee.MinPoint == 0 || guarantee.MaxPoint < guarantee.MinPoint {
					return failure(fmt.Errorf("突破保底源缺失"))
				}
				v, e := symbolDraw(draw, guarantee.MaxPoint-guarantee.MinPoint+1)
				if e != nil {
					return failure(e)
				}
				growth.Points[grade] = min(uint32(100), growth.Points[grade]+guarantee.MinPoint+v)
			}
			target = req.Template
			visual.Outcome = 4
			result = "突破未升品，保底点数已增加"
		} else {
			pool := l.Growth.Groups[groupIndex].Grades[target]
			if len(pool) == 0 {
				return failure(fmt.Errorf("突破目标源缺失"))
			}
			v, e := symbolDraw(draw, uint32(len(pool)))
			if e != nil {
				return failure(e)
			}
			target = pool[v]
			growth.Points[grade] = 0
			result = "徽章突破成功"
		}
		visual.Item = target
		visual.Amount = 1
		if e = symbolAward(&bag, &materials, s.Catalog, s.BagRules, target, 1, item.ExpireTime); e != nil {
			return failure(e)
		}
	case SymbolGradeTicket:
		action, e := symbolGradeTicket(s.Catalog, req.Template)
		if e != nil {
			return failure(e)
		}
		name := symbolColorName(action.Color)
		p, ok := slots.Progress[name]
		if !ok || p.Grade >= action.Grade {
			return failure(fmt.Errorf("星徽已达到此券的上限"))
		}
		if req.Argument == 0 {
			if e = symbolPayOffer(s, &bag, &materials, req.Template, now); e != nil {
				return failure(e)
			}
		} else if req.Argument == 1 {
			if _, e = symbolTake(&bag, req.Slot, req.Template, 1, now); e != nil {
				return failure(e)
			}
		} else {
			return failure(fmt.Errorf("无效提升券来源"))
		}
		if action.Kind == "[symbolemblem grade]" {
			p = SymbolProgress{action.Grade, 1}
		} else {
			var ok bool
			p, ok = symbolNext(l.Rules, action.Color, p)
			if !ok {
				return failure(fmt.Errorf("星徽已满级"))
			}
		}
		if _, e = l.Rules.FixedAt(action.Color, p.Grade, p.Level); e != nil {
			return failure(e)
		}
		slots.Progress[name] = p
		growth.Lights = nil
		result = "星徽提升券使用成功"
	case SymbolGrowthPurchase:
		if req.Amount != 1 {
			return failure(fmt.Errorf("一次兑换一份"))
		}
		if e = symbolPayOffer(s, &bag, &materials, req.Template, now); e != nil {
			return failure(e)
		}
		if e = symbolAward(&bag, &materials, s.Catalog, s.BagRules, req.Template, 1, 0); e != nil {
			return failure(e)
		}
		result = "兑换成功"
	case SymbolChangeEmblem:
		rule, e := symbolChangeRule(s.Catalog, req.Argument)
		if e != nil {
			return failure(e)
		}
		if rule.Inputs[req.Template] != 1 {
			return failure(fmt.Errorf("变更券不适用于此徽章"))
		}
		item, e := symbolTake(&bag, req.Slot, req.Template, 1, now)
		if e != nil {
			return failure(e)
		}
		physical := false
		for _, i := range bag.Items {
			if i.Template == req.Argument && i.Amount > 0 && !protocol.StoredItemExpired(i.ExpireTime, now) {
				physical = true
				break
			}
		}
		if physical {
			if e = symbolSpend(&bag, &materials, s.Catalog, req.Argument, 1, now); e != nil {
				return failure(e)
			}
		} else {
			if e = symbolPayOffer(s, &bag, &materials, req.Argument, now); e != nil {
				return failure(e)
			}
		}
		for _, fee := range rule.Materials {
			if e = symbolSpend(&bag, &materials, s.Catalog, fee.Template, fee.Amount, now); e != nil {
				return failure(e)
			}
		}
		v, e := symbolDraw(draw, rule.Total)
		if e != nil {
			return failure(e)
		}
		target := uint32(0)
		for _, row := range rule.Results {
			if v < row.Weight {
				target = row.Template
				break
			}
			v -= row.Weight
		}
		if target < 2 {
			return failure(fmt.Errorf("变更源结果缺失"))
		}
		before, e := symbolItemScalar(s.Catalog, req.Template, "[grade]")
		if e != nil {
			return failure(e)
		}
		after, e := symbolItemScalar(s.Catalog, target, "[grade]")
		if e != nil || before != after {
			return failure(fmt.Errorf("变更源品质不一致"))
		}
		visual.Item = target
		visual.Amount = 1
		if e = symbolAward(&bag, &materials, s.Catalog, s.BagRules, target, 1, item.ExpireTime); e != nil {
			return failure(e)
		}
		result = "徽章同品质变更完成"
	default:
		return failure(fmt.Errorf("未支持的星徽操作"))
	}
	slots.Sequence++
	growth.LastResult = result
	growth.Visual = &visual
	updated, e := SaveBag(state, bag)
	if e != nil {
		return failure(e)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(updated, &fields); e != nil {
		return failure(e)
	}
	fields["symbol_emblem_sockets"], e = json.Marshal(slots)
	if e != nil {
		return failure(e)
	}
	fields["symbol_growth"], e = json.Marshal(growth)
	if e != nil {
		return failure(e)
	}
	updated, e = json.Marshal(fields)
	if e != nil {
		return failure(e)
	}
	receipt = SymbolGrowthReceipt{req, result}
	materials.SymbolReceipt, e = json.Marshal(receipt)
	if e != nil {
		return failure(e)
	}
	nextCounts, e := materials.Save()
	if e != nil {
		return failure(e)
	}
	return updated, nextCounts, receipt, nil
}
func symbolColorName(typ uint32) string {
	for name, t := range SymbolTypes {
		if t == typ {
			return name
		}
	}
	return ""
}

// Production RNG is shared with the existing cryptographic emblem executor.
func SymbolGrowthDraw(limit uint32) (uint32, error) { return emblemCompoundDraw(limit) }

// The CN client tooltip shows three red lights as base + 3%, including
// the clicked light (COLG thread 9594335, native screenshot 3).
// Multicolor counting remains the previous compatibility behavior until its
// consumer is verified; the single-color evidence does not establish it.
func symbolResonanceSuccessRate(step SymbolResonanceStep, tuning SymbolTuningRules, color, lightCount, unit uint32) uint64 {
	bonusCount := lightCount
	if color == 4 && bonusCount > 0 {
		bonusCount--
	}
	return min(uint64(unit), uint64(step.SuccessRate)+uint64(bonusCount)*uint64(tuning.DuplicateButtonBonusRate))
}
