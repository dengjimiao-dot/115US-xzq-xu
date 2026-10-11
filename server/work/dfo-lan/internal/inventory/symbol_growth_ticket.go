package inventory

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
)

type SymbolTicketAction struct {
	Kind         string
	Color, Grade uint32
}

func symbolGradeTicket(c catalog.LootCatalog, id uint32) (SymbolTicketAction, error) {
	var out SymbolTicketAction
	script, e := c.ItemScript(id)
	if e != nil {
		return out, e
	}
	active := false
	var args []uint32
	for _, t := range script.Cells {
		if t.Type == 3 {
			if active {
				break
			}
			active = t.Text == "[action type]"
			continue
		}
		if !active {
			continue
		}
		if out.Kind == "" {
			if t.Type != 6 || (t.Text != "[symbolemblem grade]" && t.Text != "[symbolemblem level]") {
				return out, fmt.Errorf("不是星徽提升券")
			}
			out.Kind = t.Text
		} else {
			if t.Type != 0 || t.Value < 0 {
				return out, fmt.Errorf("星徽提升券参数错误")
			}
			args = append(args, uint32(t.Value))
		}
	}
	if len(args) != 2 || args[0] > 3 || args[1] == 0 {
		return out, fmt.Errorf("星徽提升券源缺失")
	}
	out.Color, out.Grade = args[0], args[1]
	return out, nil
}
func symbolPayOffer(s *ItemService, b *Bag, m *AccountMaterials, id uint32, now int64) error {
	l := s.SymbolSockets
	if l.Growth.Shop == nil {
		return fmt.Errorf("达芙妮商店源缺失")
	}
	found := false
	for _, offer := range l.Growth.Shop.Shops["1"].Offers {
		if offer.Template == id {
			if offer.LimitCount != 0 {
				return fmt.Errorf("限购兑换请使用原生商店")
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("不在达芙妮源商店中")
	}
	fees, e := symbolItemNumbers(s.Catalog, id, "[need material]")
	if e != nil {
		return e
	}
	if len(fees)%2 != 0 {
		return fmt.Errorf("兑换材料源错误")
	}
	if len(fees) == 0 {
		price, e := symbolItemScalar(s.Catalog, id, "[price]")
		if e != nil || price == 0 {
			return fmt.Errorf("兑换价格缺失")
		}
		return symbolSpend(b, m, s.Catalog, 0, price, now)
	}
	for j := 0; j < len(fees); j += 2 {
		if fees[j] < 2 || fees[j+1] == 0 {
			return fmt.Errorf("兑换材料源错误")
		}
		if e = symbolSpend(b, m, s.Catalog, fees[j], fees[j+1], now); e != nil {
			return e
		}
	}
	return nil
}

type SymbolChangeResult struct{ Template, Amount, Weight uint32 }
type SymbolChangeRule struct {
	Inputs    map[uint32]uint32
	Materials []EmblemMaterial
	Results   []SymbolChangeResult
	Total     uint32
}

func symbolChangeRule(c catalog.LootCatalog, id uint32) (SymbolChangeRule, error) {
	out := SymbolChangeRule{Inputs: map[uint32]uint32{}}
	inputs, e := symbolItemNumbers(c, id, "[A condition item]")
	if e != nil || len(inputs) == 0 || len(inputs)%2 != 0 {
		return out, fmt.Errorf("不是徽章变更券")
	}
	for i := 0; i < len(inputs); i += 2 {
		if inputs[i] < 2 || inputs[i+1] != 1 {
			return out, fmt.Errorf("变更券条件错误")
		}
		definition, ok := c.Items[inputs[i]]
		if !ok || definition.StackableType != "[avatar emblem]" {
			return out, fmt.Errorf("变更券非徽章条件")
		}
		out.Inputs[inputs[i]] = inputs[i+1]
	}
	fees, e := symbolItemNumbers(c, id, "[B condition item]")
	if e != nil || len(fees)%2 != 0 {
		return out, fmt.Errorf("变更券费用错误")
	}
	for i := 0; i < len(fees); i += 2 {
		if fees[i] < 2 || fees[i+1] == 0 {
			return out, fmt.Errorf("变更券费用错误")
		}
		out.Materials = append(out.Materials, EmblemMaterial{fees[i], fees[i+1]})
	}
	results, e := symbolItemNumbers(c, id, "[result item]")
	if e != nil || len(results) == 0 || len(results)%3 != 0 {
		return out, fmt.Errorf("变更券结果错误")
	}
	var total uint64
	for i := 0; i < len(results); i += 3 {
		if results[i] < 2 || results[i+1] != 1 || results[i+2] == 0 {
			return out, fmt.Errorf("变更券结果错误")
		}
		definition, ok := c.Items[results[i]]
		if !ok || definition.StackableType != "[avatar emblem]" {
			return out, fmt.Errorf("变更券非徽章结果")
		}
		out.Results = append(out.Results, SymbolChangeResult{results[i], results[i+1], results[i+2]})
		total += uint64(results[i+2])
	}
	if total > math.MaxUint32 {
		return out, fmt.Errorf("变更概率溢出")
	}
	out.Total = uint32(total)
	return out, nil
}
