package inventory

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmblemGrowthCurrentCNTables(t *testing.T) {
	dir := os.Getenv("SYMBOL_GROWTH_CN_TABLES")
	if dir == "" {
		t.Skip("explicit CN source evidence")
	}
	tables := map[string]*pvf.CTPTable{}
	for _, name := range []string{"emblemupgradesystem.ctp", "disjointemblem.ctp", "emblemgroup.ctp"} {
		raw, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil {
			t.Fatal(e)
		}
		tables[name], e = pvf.ReadCTP(SymbolEmblemSourceDirectory+name, raw)
		if e != nil {
			t.Fatal(e)
		}
	}
	r, e := ParseEmblemGrowthRules(strings.Repeat("a", 64), tables)
	if e != nil {
		t.Fatal(e)
	}
	if r.Normal[2].Material.Amount != 15 || r.Special[7].Material.Amount != 750 || r.Guarantee[3].MinPoint != 10 {
		t.Fatalf("wrong current source %+v", r)
	}
	if r.Disjoint[4].Min != 35 || r.Disjoint[2].Template != 490718835 {
		t.Fatal("wrong source disjoint")
	}
	t.Logf("%d groups, %d emblem identities", len(r.Groups), len(r.ByItem))
}
