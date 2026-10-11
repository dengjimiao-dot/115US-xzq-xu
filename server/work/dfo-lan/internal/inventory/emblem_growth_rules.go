package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"strings"
)

// Runtime content is imported from the selected PVF, never from the audit JSON.
type EmblemGrowthRules struct {
	Source          string
	Tables          map[string]string
	Normal, Special map[uint32]EmblemUpgradeRule
	Guarantee       map[uint32]EmblemGuaranteeRule
	Disjoint        map[uint32]EmblemDisjointRule
	Groups          []EmblemGrowthGroup
	ByItem          map[uint32]int
	Shop            *catalog.ItemShops
	Names           map[uint32]string
}
type EmblemMaterial struct{ Template, Amount uint32 }
type EmblemGradeWeight struct{ Grade, Weight uint32 }
type EmblemUpgradeRule struct {
	Material, Replace EmblemMaterial
	Results           []EmblemGradeWeight
}
type EmblemGuaranteeRule struct{ NextGrade, MinPoint, MaxPoint uint32 }
type EmblemDisjointRule struct{ Template, Min, Max uint32 }
type EmblemGrowthGroup struct {
	Sockets []string
	Grades  map[uint32][]uint32
}

func ImportEmblemGrowthRules(a *pvf.Archive) (*EmblemGrowthRules, error) {
	if a == nil {
		return nil, fmt.Errorf("emblem growth source missing")
	}
	// Original US archives intentionally retain their old compound system.
	if _, ok := a.FindFile(SymbolEmblemSourceDirectory + "emblemupgradesystem.ctp"); !ok {
		return nil, nil
	}
	tables := map[string]*pvf.CTPTable{}
	for _, name := range []string{"emblemupgradesystem.ctp", "disjointemblem.ctp", "emblemgroup.ctp"} {
		p := SymbolEmblemSourceDirectory + name
		raw, e := a.ReadRaw(p)
		if e != nil {
			return nil, e
		}
		table, e := pvf.ReadCTP(p, raw)
		if e != nil {
			return nil, e
		}
		tables[name] = table
	}
	rules, e := ParseEmblemGrowthRules(a.Snapshot().Checksum, tables)
	if e != nil {
		return nil, e
	}
	// Existing native shop importer remains the single offer/limit parser.
	shops, e := catalog.ImportItemShops(a, catalog.ItemShopSourcePolicy{Version: 1, PurchaseLimitMode: "disabled", Routes: []catalog.ItemShopRoute{{ServerShopID: 1, ScriptPath: "itemshop/emblem.shp"}}})
	if e != nil {
		return nil, e
	}
	rules.Shop = shops.Catalog
	index, e := catalog.ResolveScript(a, "list/stackable.lst")
	if e != nil {
		return nil, e
	}
	entries, e := catalog.ParseIndex(index.Cells)
	if e != nil {
		return nil, e
	}
	rules.Names = map[uint32]string{}
	wanted := map[uint32]bool{}
	for id := range rules.ByItem {
		wanted[id] = true
	}
	for _, d := range rules.Disjoint {
		wanted[d.Template] = true
	}
	for _, v := range rules.Normal {
		wanted[v.Material.Template] = true
	}
	for _, v := range rules.Special {
		wanted[v.Material.Template] = true
		wanted[v.Replace.Template] = true
	}
	for _, offer := range rules.Shop.Shops["1"].Offers {
		wanted[offer.Template] = true
	}
	for _, entry := range entries {
		if !wanted[entry.ID] {
			continue
		}
		path := entry.Path
		if !strings.HasPrefix(path, "stackable/") {
			path = "stackable/" + path
		}
		path = catalog.ResolveScriptPath(a, path)
		name, found, e := a.LocalizedNameByPath(path)
		if e != nil {
			return nil, e
		}
		if found {
			rules.Names[entry.ID] = name
		}
	}
	return rules, nil
}
func ParseEmblemGrowthRules(source string, tables map[string]*pvf.CTPTable) (*EmblemGrowthRules, error) {
	digest, e := hex.DecodeString(source)
	if e != nil || len(digest) != 32 {
		return nil, fmt.Errorf("invalid emblem growth source")
	}
	r := &EmblemGrowthRules{Source: source, Tables: map[string]string{}, Normal: map[uint32]EmblemUpgradeRule{}, Special: map[uint32]EmblemUpgradeRule{}, Guarantee: map[uint32]EmblemGuaranteeRule{}, Disjoint: map[uint32]EmblemDisjointRule{}, ByItem: map[uint32]int{}}
	trees := map[string]*pvf.CTPTree{}
	for _, name := range []string{"emblemupgradesystem.ctp", "disjointemblem.ctp", "emblemgroup.ctp"} {
		table := tables[name]
		if table == nil || table.Path != SymbolEmblemSourceDirectory+name {
			return nil, fmt.Errorf("missing emblem growth table %s", name)
		}
		digest, e := hex.DecodeString(table.SHA256)
		if e != nil || len(digest) != 32 {
			return nil, fmt.Errorf("invalid emblem growth digest")
		}
		tree, e := pvf.NewCTPTree(table)
		if e != nil {
			return nil, e
		}
		trees[name] = tree
		r.Tables[name] = table.SHA256
	}
	u := trees["emblemupgradesystem.ctp"]
	values := func(t *pvf.CTPTree, parent int, name string, count int) ([]uint32, error) {
		row, e := t.One(parent, name)
		if e != nil {
			return nil, e
		}
		v, e := symbolIntegers(row)
		if e != nil || len(v) != count || len(t.Children(row.Index, "")) != 0 {
			return nil, fmt.Errorf("invalid emblem growth %s", name)
		}
		return v, nil
	}
	for _, row := range u.Children(-1, "[guarantee upgrade]") {
		grade, e := symbolInteger(u, row.Index, "[grade]")
		if e != nil {
			return nil, e
		}
		next, e := symbolInteger(u, row.Index, "[next grade]")
		if e != nil {
			return nil, e
		}
		p, e := values(u, row.Index, "[point]", 2)
		if e != nil {
			return nil, e
		}
		if _, ok := r.Guarantee[grade]; ok || next <= grade || p[0] == 0 || p[1] < p[0] {
			return nil, fmt.Errorf("invalid emblem guarantee")
		}
		r.Guarantee[grade] = EmblemGuaranteeRule{next, p[0], p[1]}
	}
	for _, kind := range []struct {
		name   string
		target map[uint32]EmblemUpgradeRule
	}{{"[normal upgrade]", r.Normal}, {"[special upgrade]", r.Special}} {
		for _, row := range u.Children(-1, kind.name) {
			grade, e := symbolInteger(u, row.Index, "[grade]")
			if e != nil {
				return nil, e
			}
			m, e := values(u, row.Index, "[material]", 2)
			if e != nil || m[0] < 2 || m[1] == 0 {
				return nil, fmt.Errorf("invalid emblem material")
			}
			rule := EmblemUpgradeRule{Material: EmblemMaterial{m[0], m[1]}}
			if len(u.Children(row.Index, "[replace material]")) != 0 {
				v, e := values(u, row.Index, "[replace material]", 2)
				if e != nil || v[0] < 2 || v[1] == 0 {
					return nil, fmt.Errorf("invalid replacement material")
				}
				rule.Replace = EmblemMaterial{v[0], v[1]}
			}
			seen := map[uint32]bool{}
			var total uint64
			for _, p := range u.Children(row.Index, "[prob]") {
				v, e := symbolIntegers(p)
				if e != nil || len(v) != 2 || v[0] < grade || v[1] == 0 || seen[v[0]] {
					return nil, fmt.Errorf("invalid emblem upgrade distribution")
				}
				seen[v[0]] = true
				total += uint64(v[1])
				rule.Results = append(rule.Results, EmblemGradeWeight{v[0], v[1]})
			}
			if total == 0 || total > 0xffffffff {
				return nil, fmt.Errorf("empty/overflow emblem upgrade distribution")
			}
			if _, ok := kind.target[grade]; ok {
				return nil, fmt.Errorf("duplicate emblem upgrade grade")
			}
			kind.target[grade] = rule
		}
	}
	d := trees["disjointemblem.ctp"]
	for _, row := range d.Children(-1, "[disjoint result info]") {
		rarity, e := symbolInteger(d, row.Index, "[rarity]")
		if e != nil {
			return nil, e
		}
		id, e := symbolInteger(d, row.Index, "[item index]")
		if e != nil {
			return nil, e
		}
		v, e := values(d, row.Index, "[random value]", 2)
		if e != nil || id < 2 || v[0] == 0 || v[1] < v[0] {
			return nil, fmt.Errorf("invalid emblem disjoint reward")
		}
		if _, ok := r.Disjoint[rarity]; ok {
			return nil, fmt.Errorf("duplicate emblem disjoint rarity")
		}
		r.Disjoint[rarity] = EmblemDisjointRule{id, v[0], v[1]}
	}
	g := trees["emblemgroup.ctp"]
	for _, row := range g.Children(-1, "[emblem group]") {
		socket, e := g.One(row.Index, "[avatar emblem target type]")
		if e != nil || len(socket.Cells) == 0 {
			return nil, fmt.Errorf("invalid emblem group socket")
		}
		group := EmblemGrowthGroup{Grades: map[uint32][]uint32{}}
		for _, c := range socket.Cells {
			if c.Kind != "name" || c.Name == "" {
				return nil, fmt.Errorf("invalid emblem group socket")
			}
			group.Sockets = append(group.Sockets, c.Name)
		}
		for _, list := range g.Children(row.Index, "[grade list]") {
			grade, e := symbolInteger(g, list.Index, "[grade]")
			if e != nil {
				return nil, e
			}
			items, e := g.One(list.Index, "[list]")
			if e != nil {
				return nil, e
			}
			ids, e := symbolIntegers(items)
			if e != nil || len(ids) == 0 {
				return nil, fmt.Errorf("empty emblem group grade")
			}
			if _, ok := group.Grades[grade]; ok {
				return nil, fmt.Errorf("duplicate emblem group grade")
			}
			for _, id := range ids {
				if id < 2 {
					return nil, fmt.Errorf("invalid emblem group item")
				}
				if prior, ok := r.ByItem[id]; ok && prior != len(r.Groups) {
					return nil, fmt.Errorf("ambiguous emblem group item %d", id)
				}
				r.ByItem[id] = len(r.Groups)
			}
			group.Grades[grade] = ids
		}
		r.Groups = append(r.Groups, group)
	}
	if len(r.Normal) == 0 || len(r.Special) == 0 || len(r.Guarantee) == 0 || len(r.Groups) == 0 || len(r.Disjoint) == 0 {
		return nil, fmt.Errorf("incomplete emblem growth rules")
	}
	return r, nil
}
