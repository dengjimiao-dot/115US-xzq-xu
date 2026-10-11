package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sort"
)

type SymbolGrowthOption struct {
	Request SymbolGrowthRequest
	Label   string
	UI      SymbolGrowthUIQuote
}

// Display projection of the same parsed rules used by the executor. These
// fields supply original CN controls; they never authorize or execute an action.
type SymbolGrowthUIQuote struct {
	MaterialTemplate, MaterialAmount, Points, SuccessRate, RateUnit uint32
	Results                                                         [][2]uint32
}
type SymbolGrowthUI struct {
	MaterialTemplate, FailRewardTemplate uint32
	MaterialBalance, FailRewardBalance   uint64
	Lights                               []uint32
}
type SymbolGrowthDisplay struct {
	Materials, Result, Progress, Lights string
	Options                             []SymbolGrowthOption
	Visual                              *SymbolGrowthVisual
	UI                                  *SymbolGrowthUI
}

func (s *ItemService) SymbolGrowthDisplay(state, counts json.RawMessage, now int64) (SymbolGrowthDisplay, error) {
	var out SymbolGrowthDisplay
	l := s.SymbolSockets
	if l == nil || l.Growth == nil || l.Rules == nil {
		return out, nil
	}
	slots, e := ReadSymbolSockets(state, l)
	if e != nil {
		return out, e
	}
	growth, e := ReadSymbolGrowth(state)
	if e != nil {
		return out, e
	}
	bag, e := ReadBag(state)
	if e != nil {
		return out, e
	}
	m, e := ReadAccountMaterials(counts)
	if e != nil {
		return out, e
	}
	r := l.Growth
	name := func(id uint32) string {
		if n := r.Names[id]; n != "" {
			return n
		}
		return "道具"
	}
	balance := func(id uint32) uint64 {
		n := uint64(m.CommonItems[id])
		for _, i := range bag.Items {
			if i.Template == id && !protocol.StoredItemExpired(i.ExpireTime, now) {
				n += uint64(i.Amount)
			}
		}
		return n
	}
	out.Materials = fmt.Sprintf("%s：%d　星痕：%d", name(l.Rules.CostItem), balance(l.Rules.CostItem), balance(l.Rules.Tuning.FailRewardItem))
	out.UI = &SymbolGrowthUI{MaterialTemplate: l.Rules.CostItem, FailRewardTemplate: l.Rules.Tuning.FailRewardItem, MaterialBalance: balance(l.Rules.CostItem), FailRewardBalance: balance(l.Rules.Tuning.FailRewardItem), Lights: append([]uint32(nil), growth.Lights...)}
	out.Result = growth.LastResult
	out.Visual = growth.Visual
	colors := []string{"红", "黄", "绿", "蓝", "彩"}
	for typ := uint32(0); typ < 4; typ++ {
		p := slots.Progress[symbolColorName(typ)]
		out.Progress += fmt.Sprintf("%s：%s%d  ", colors[typ], []string{"稀有", "神器", "传说", "史诗"}[p.Grade], p.Level)
	}
	add := func(op, slot uint16, id, arg, n uint32, label string) *SymbolGrowthOption {
		out.Options = append(out.Options, SymbolGrowthOption{Request: SymbolGrowthRequest{Source: l.Source, Sequence: slots.Sequence, Operation: op, Slot: slot, Template: id, Argument: arg, Amount: n}, Label: label})
		return &out.Options[len(out.Options)-1]
	}
	if len(growth.Lights) == 0 {
		full := true
		for typ := uint32(0); typ < 4; typ++ {
			if _, ok := symbolNext(l.Rules, typ, slots.Progress[symbolColorName(typ)]); ok {
				full = false
			}
		}
		if !full {
			add(SymbolGenerateLights, 0, 0, 0, 0, "生成共鸣之光")
		}
	} else {
		unit, e := symbolRateUnit(r)
		if e != nil {
			return out, e
		}
		var duplicates [5]uint32
		for _, c := range growth.Lights {
			if c > 4 {
				return out, fmt.Errorf("invalid saved resonance lights")
			}
			duplicates[c]++
			out.Lights += colors[c] + " "
		}
		for color, n := range duplicates {
			if n == 0 {
				continue
			}
			first := uint32(0)
			for i, c := range growth.Lights {
				if c == uint32(color) {
					first = uint32(i)
					break
				}
			}
			target := uint32(color)
			if color == 4 {
				target = 0
				found := false
				for typ := uint32(0); typ < 4; typ++ {
					p := slots.Progress[symbolColorName(typ)]
					if _, ok := symbolNext(l.Rules, typ, p); !ok {
						continue
					}
					previous := slots.Progress[symbolColorName(target)]
					if !found || p.Grade > previous.Grade || p.Grade == previous.Grade && p.Level > previous.Level {
						target = typ
						found = true
					}
				}
			}
			p := slots.Progress[symbolColorName(target)]
			step := l.Rules.Steps[SymbolStepKey{p.Grade, p.Level}]
			cost := step.Cost
			if color == 4 {
				cost = l.Rules.Tuning.MultiColorCost
			}
			rate := symbolResonanceSuccessRate(step, l.Rules.Tuning, uint32(color), n, unit)
			o := add(SymbolResonate, 0, 0, first, 0, fmt.Sprintf("%s色共鸣　成功率%.2f%%　%d星光", colors[color], float64(rate)*100/float64(unit), cost))
			o.UI = SymbolGrowthUIQuote{MaterialTemplate: l.Rules.CostItem, MaterialAmount: cost, SuccessRate: uint32(rate), RateUnit: unit}
		}
	}
	sorted := append([]BagItem(nil), bag.Items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slot < sorted[j].Slot })
	for _, i := range sorted {
		if protocol.StoredItemExpired(i.ExpireTime, now) || i.Amount == 0 {
			continue
		}
		definition, ok := s.Catalog.Items[i.Template]
		if !ok {
			continue
		}
		if definition.StackableType == "[avatar emblem]" {
			rarity, e := symbolItemScalar(s.Catalog, i.Template, "[rarity]")
			if e != nil {
				return out, e
			}
			if d, ok := r.Disjoint[rarity]; ok {
				add(SymbolDisjoint, i.Slot, i.Template, 0, 1, fmt.Sprintf("分解%s　获得%d～%d%s", name(i.Template), d.Min, d.Max, name(d.Template)))
			}
			grade, e := symbolItemScalar(s.Catalog, i.Template, "[grade]")
			if e != nil {
				return out, e
			}
			if _, ok := r.ByItem[i.Template]; !ok {
				continue
			}
			if u, ok := r.Normal[grade]; ok {
				o := add(SymbolNormalUpgrade, i.Slot, i.Template, 0, 0, fmt.Sprintf("普通突破%s　%d星光　保底%d%%", name(i.Template), u.Material.Amount, growth.Points[grade]))
				o.UI = SymbolGrowthUIQuote{MaterialTemplate: u.Material.Template, MaterialAmount: u.Material.Amount, Points: growth.Points[grade]}
				for _, result := range u.Results {
					o.UI.Results = append(o.UI.Results, [2]uint32{result.Grade, result.Weight})
					o.UI.RateUnit += result.Weight
				}
			}
			if u, ok := r.Special[grade]; ok {
				o := add(SymbolSpecialUpgrade, i.Slot, i.Template, 0, 0, fmt.Sprintf("必定突破%s　两个相同徽章＋%d星光", name(i.Template), u.Material.Amount))
				o.UI = SymbolGrowthUIQuote{MaterialTemplate: u.Material.Template, MaterialAmount: u.Material.Amount}
				for _, result := range u.Results {
					o.UI.Results = append(o.UI.Results, [2]uint32{result.Grade, result.Weight})
					o.UI.RateUnit += result.Weight
				}
				if u.Replace.Template > 1 {
					o := add(SymbolSpecialUpgrade, i.Slot, i.Template, 1, 0, fmt.Sprintf("必定突破%s　两个徽章＋%s", name(i.Template), name(u.Replace.Template)))
					o.UI = SymbolGrowthUIQuote{MaterialTemplate: u.Replace.Template, MaterialAmount: u.Replace.Amount}
					for _, result := range u.Results {
						o.UI.Results = append(o.UI.Results, [2]uint32{result.Grade, result.Weight})
						o.UI.RateUnit += result.Weight
					}
				}
			}
		}
		if action, e := symbolGradeTicket(s.Catalog, i.Template); e == nil {
			if p := slots.Progress[symbolColorName(action.Color)]; p.Grade < action.Grade {
				o := add(SymbolGradeTicket, i.Slot, i.Template, 1, 0, "使用"+name(i.Template))
				o.UI = SymbolGrowthUIQuote{MaterialTemplate: i.Template, MaterialAmount: 1}
			}
		}
	}
	if r.Shop != nil {
		for _, offer := range r.Shop.Shops["1"].Offers {
			id := offer.Template
			cost, e := symbolItemNumbers(s.Catalog, id, "[need material]")
			if e != nil {
				return out, e
			}
			fee := ""
			for j := 0; j+1 < len(cost); j += 2 {
				fee += fmt.Sprintf("%d%s ", cost[j+1], name(cost[j]))
			}
			if len(cost) == 0 {
				if p, e := symbolItemScalar(s.Catalog, id, "[price]"); e == nil {
					fee = fmt.Sprintf("%d金币", p)
				} else {
					continue
				}
			}
			var quote SymbolGrowthUIQuote
			if len(cost) >= 2 {
				quote.MaterialTemplate = cost[0]
				quote.MaterialAmount = cost[1]
			}
			if action, e := symbolGradeTicket(s.Catalog, id); e == nil {
				if p := slots.Progress[symbolColorName(action.Color)]; p.Grade < action.Grade {
					o := add(SymbolGradeTicket, 0, id, 0, 0, "兑换并使用"+name(id)+"　"+fee)
					o.UI = quote
				}
			} else {
				o := add(SymbolGrowthPurchase, 0, id, 0, 1, "兑换"+name(id)+"　"+fee)
				o.UI = quote
			}
		}
	}
	// Present source-defined conversion coupons, held physically or in Daphne's shop.
	coupons := map[uint32]bool{}
	for _, i := range sorted {
		coupons[i.Template] = true
	}
	if r.Shop != nil {
		for _, o := range r.Shop.Shops["1"].Offers {
			coupons[o.Template] = true
		}
	}
	ids := []uint32{}
	for id := range coupons {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		rule, e := symbolChangeRule(s.Catalog, id)
		if e != nil {
			continue
		}
		fee := ""
		held := false
		for _, item := range sorted {
			if item.Template == id && item.Amount > 0 && !protocol.StoredItemExpired(item.ExpireTime, now) {
				held = true
			}
		}
		if held {
			fee = "消耗1张变更券"
		} else {
			cost, e := symbolItemNumbers(s.Catalog, id, "[need material]")
			if e != nil {
				return out, e
			}
			for j := 0; j+1 < len(cost); j += 2 {
				fee += fmt.Sprintf("%d%s ", cost[j+1], name(cost[j]))
			}
		}
		for _, material := range rule.Materials {
			fee += fmt.Sprintf("＋%d%s", material.Amount, name(material.Template))
		}
		for _, i := range sorted {
			if rule.Inputs[i.Template] == 1 && i.Amount > 0 && !protocol.StoredItemExpired(i.ExpireTime, now) {
				add(SymbolChangeEmblem, i.Slot, i.Template, id, 0, "变更"+name(i.Template)+"（"+name(id)+"） "+fee)
			}
		}
	}
	if len(out.Options) > 1024 {
		return out, fmt.Errorf("too many growth options")
	}
	return out, nil
}
