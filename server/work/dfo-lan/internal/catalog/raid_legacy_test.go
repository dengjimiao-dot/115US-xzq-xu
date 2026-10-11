package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func TestLegacyRaidsCurrentUSSource(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native archive required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"etc/raid/siroco/siroco.etc", "contents/2021/ozma_raid/etc/ozma.etc"} {
		r, err := ImportLegacyRaidRules(a, path)
		if err != nil {
			t.Fatal(path, err)
		}
		total := 0
		for mode, phases := range r.Modes {
			n, rooms := 0, 0
			for _, p := range phases {
				n += len(p.Events)
				rooms += len(p.Dungeons)
			}
			total += n
			t.Logf("%s %s phases=%d events=%d rooms=%d", path, mode, len(phases), n, rooms)
		}
		if total == 0 || r.SHA256 == "" || r.SymbolSHA256 == "" {
			t.Fatal("missing source identity")
		}
		// Break a real source boundary: no partial mode may be accepted.
		source, err := ResolveScript(a, path)
		if err != nil {
			t.Fatal(err)
		}
		symbols, err := ResolveScript(a, path+".symbol")
		if err != nil {
			t.Fatal(err)
		}
		for i, v := range source.Cells {
			if v.Text == "[/BEHAVIOR]" {
				source.Cells = append(source.Cells[:i], source.Cells[i+1:]...)
				break
			}
		}
		if _, err = ParseLegacyRaidRules(source, symbols); err == nil {
			t.Fatal("broken source accepted")
		}
	}
}

func TestLegacyRaidSectionsPreserveTypedValues(t *testing.T) {
	sections, err := LegacyRaidSections([]pvf.Token{{Type: 3, Text: "[SET]"}, {Type: 3, Text: "[HP]"}, {Type: 0, Value: 7}, {Type: 10, Value: 42}})
	if err != nil || len(sections) != 2 || sections[1].Values[1].Type != 10 {
		t.Fatalf("lost typed operands: %+v %v", sections, err)
	}
	if _, err = LegacyRaidSections([]pvf.Token{{Type: 0, Value: 1}}); err == nil {
		t.Fatal("orphan value accepted")
	}
	if _, err = legacyRaidBlocks([]pvf.Token{{Type: 3, Text: "[PHASE]"}}, "[PHASE]"); err == nil {
		t.Fatal("unclosed block accepted")
	}
	if _, err = legacyRaidBlocks([]pvf.Token{{Type: 3, Text: "[/PHASE]"}}, "[PHASE]"); err == nil {
		t.Fatal("orphan closing accepted")
	}
}
