package catalog

// Legacy raid source reader. Structural layout follows CN PvfRaidProgram and
// PvfRaidCatalog; all content is resolved from the caller's US archive. This
// does not publish a channel or authorize a client battle.
import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

type LegacyRaidRules struct {
	Path, SHA256, SymbolSHA256 string
	Symbols                    map[string]int32
	Modes                      map[string][]LegacyRaidPhase
}
type LegacyRaidSection struct {
	Name   string
	Values []pvf.Token
}
type LegacyRaidEvent struct{ Trigger, Behavior []LegacyRaidSection }
type LegacyRaidDungeon struct {
	ID                                                    int32
	Kind, InitialState, HPSymbol                          string
	HP, PartyMax, ReturnSeconds, RetrySeconds, RetryGroup int32
}
type LegacyRaidPhase struct {
	Index    int
	Dungeons map[int32]LegacyRaidDungeon
	Events   []LegacyRaidEvent
	// Raw rewards remain source tokens until a raid-specific award contract is
	// established. No generic item grant is inferred from these rows.
	Rewards []LegacyRaidSection
}

func LegacyRaidSections(ts []pvf.Token) ([]LegacyRaidSection, error) {
	var out []LegacyRaidSection
	for _, t := range ts {
		if t.Type == 3 {
			if len(t.Text) < 2 || t.Text[0] != '[' || t.Text[len(t.Text)-1] != ']' {
				return nil, fmt.Errorf("invalid raid label %q", t.Text)
			}
			out = append(out, LegacyRaidSection{Name: strings.ToLower(t.Text[1 : len(t.Text)-1])})
		} else {
			if len(out) == 0 {
				return nil, fmt.Errorf("raid value before field")
			}
			out[len(out)-1].Values = append(out[len(out)-1].Values, t)
		}
	}
	return out, nil
}

// Blocks rejects a broken boundary rather than silently importing a prefix.
func legacyRaidBlocks(ts []pvf.Token, tag string) ([][]pvf.Token, error) {
	var blocks [][]pvf.Token
	start := -1
	end := "[/" + tag[1:]
	for i, t := range ts {
		if t.Type != 3 {
			continue
		}
		if t.Text == tag {
			if start >= 0 {
				return nil, fmt.Errorf("nested %s", tag)
			}
			start = i + 1
		}
		if t.Text == end {
			if start < 0 {
				return nil, fmt.Errorf("orphan %s", end)
			}
			blocks = append(blocks, append([]pvf.Token(nil), ts[start:i]...))
			start = -1
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("unclosed %s", tag)
	}
	return blocks, nil
}

func legacyRaidField(ts []pvf.Token, tag string) []pvf.Token {
	for i, t := range ts {
		if t.Type == 3 && t.Text == tag {
			j := i + 1
			for j < len(ts) && ts[j].Type != 3 {
				j++
			}
			return ts[i+1 : j]
		}
	}
	return nil
}
func legacyRaidInt(ts []pvf.Token, tag string, optional bool) (int32, error) {
	v := legacyRaidField(ts, tag)
	if len(v) == 0 && optional {
		return 0, nil
	}
	if len(v) != 1 || v[0].Type != 0 || v[0].Value < 0 {
		return 0, fmt.Errorf("invalid legacy raid %s", tag)
	}
	return v[0].Value, nil
}

func ParseLegacyRaidRules(s, symbol ScriptRecord) (*LegacyRaidRules, error) {
	r := &LegacyRaidRules{Path: s.Path, SHA256: s.SHA256, SymbolSHA256: symbol.SHA256, Symbols: map[string]int32{}, Modes: map[string][]LegacyRaidPhase{}}
	ids := map[int32]bool{}
	for i := 0; i < len(symbol.Cells); i += 2 {
		if i+1 >= len(symbol.Cells) || symbol.Cells[i].Type != 0 || symbol.Cells[i].Value < 0 || symbol.Cells[i+1].Type != 3 {
			return nil, fmt.Errorf("invalid legacy raid symbol pair %d", i)
		}
		id, name := symbol.Cells[i].Value, symbol.Cells[i+1].Text
		if _, ok := r.Symbols[name]; ok || ids[id] || name == "" {
			return nil, fmt.Errorf("duplicate/empty legacy raid symbol %q", name)
		}
		r.Symbols[name] = id
		ids[id] = true
	}
	count, err := legacyRaidInt(s.Cells, "[PHASE MAX]", false)
	if err != nil || count == 0 || len(r.Symbols) == 0 {
		return nil, fmt.Errorf("missing legacy raid phases/symbols: %v", err)
	}
	for _, mode := range []struct{ name, tag string }{{"normal", "[PHASE]"}, {"hard", "[RAID PHASE OF HARDMODE]"}, {"guide", "[SINGLE RAID PHASE]"}, {"squad", "[SINGLE APC RAID PHASE]"}} {
		blocks, err := legacyRaidBlocks(s.Cells, mode.tag)
		if err != nil {
			return nil, err
		}
		if len(blocks) == 0 {
			continue
		}
		if len(blocks) != int(count) {
			return nil, fmt.Errorf("%s phase count %d != source %d", mode.name, len(blocks), count)
		}
		for index, block := range blocks {
			phase := LegacyRaidPhase{Index: index, Dungeons: map[int32]LegacyRaidDungeon{}}
			rooms, err := legacyRaidBlocks(block, "[DUNGEON INFO]")
			if err != nil {
				return nil, err
			}
			for _, room := range rooms {
				d := LegacyRaidDungeon{}
				for _, field := range []struct {
					tag      string
					dst      *int32
					optional bool
				}{{"[INDEX]", &d.ID, false}, {"[PARTY MAX]", &d.PartyMax, false}, {"[RETURN VILLAGE TIME]", &d.ReturnSeconds, true}, {"[MEMBER RETRY TIME]", &d.RetrySeconds, true}, {"[RETRY TIME GROUP]", &d.RetryGroup, true}} {
					*field.dst, err = legacyRaidInt(room, field.tag, field.optional)
					if err != nil {
						return nil, err
					}
				}
				for _, field := range []struct {
					tag      string
					dst      *string
					optional bool
				}{{"[TYPE]", &d.Kind, true}, {"[INIT STATE]", &d.InitialState, false}} {
					v := legacyRaidField(room, field.tag)
					if len(v) == 0 && field.optional {
						continue
					}
					if len(v) != 1 || v[0].Type != 6 {
						return nil, fmt.Errorf("invalid room %s", field.tag)
					}
					*field.dst = v[0].Text
				}
				if d.InitialState != "open" && d.InitialState != "hide" && d.InitialState != "close" {
					return nil, fmt.Errorf("invalid initial room state %q", d.InitialState)
				}
				sections, err := LegacyRaidSections(room)
				if err != nil {
					return nil, err
				}
				found := false
				for j, section := range sections {
					if section.Name == "hp info" {
						if found || j+1 >= len(sections) {
							return nil, fmt.Errorf("invalid HP field")
						}
						hp := sections[j+1]
						d.HPSymbol = "[" + strings.ToUpper(hp.Name) + "]"
						if _, ok := r.Symbols[d.HPSymbol]; !ok || len(hp.Values) != 1 || hp.Values[0].Type != 0 || hp.Values[0].Value < 0 {
							return nil, fmt.Errorf("invalid HP symbol %s", d.HPSymbol)
						}
						d.HP = hp.Values[0].Value
						found = true
					}
				}
				if _, dup := phase.Dungeons[d.ID]; dup || d.ID <= 0 || d.PartyMax <= 0 || !found {
					return nil, fmt.Errorf("invalid/duplicate room %d", d.ID)
				}
				phase.Dungeons[d.ID] = d
			}
			sections, err := LegacyRaidSections(block)
			if err != nil {
				return nil, err
			}
			for i := 0; i < len(sections); i++ {
				if sections[i].Name == "/trigger" || sections[i].Name == "behavior" || sections[i].Name == "/behavior" {
					return nil, fmt.Errorf("orphan program boundary %s", sections[i].Name)
				}
				if sections[i].Name != "trigger" {
					continue
				}
				begin := i + 1
				i++
				for i < len(sections) && sections[i].Name != "/trigger" {
					if sections[i].Name == "trigger" || sections[i].Name == "behavior" || sections[i].Name == "/behavior" {
						return nil, fmt.Errorf("broken trigger")
					}
					i++
				}
				if i+1 >= len(sections) || sections[i+1].Name != "behavior" {
					return nil, fmt.Errorf("unpaired trigger")
				}
				trigger := append([]LegacyRaidSection(nil), sections[begin:i]...)
				begin = i + 2
				i += 2
				for i < len(sections) && sections[i].Name != "/behavior" {
					if sections[i].Name == "trigger" || sections[i].Name == "/trigger" || sections[i].Name == "behavior" {
						return nil, fmt.Errorf("broken behavior")
					}
					i++
				}
				if i >= len(sections) || len(trigger) == 0 || begin == i {
					return nil, fmt.Errorf("incomplete event")
				}
				phase.Events = append(phase.Events, LegacyRaidEvent{trigger, append([]LegacyRaidSection(nil), sections[begin:i]...)})
			}
			rewardBlocks, err := legacyRaidBlocks(block, "[CLEAR REWARD ITEM]")
			if err != nil {
				return nil, err
			}
			for _, reward := range rewardBlocks {
				rows, err := LegacyRaidSections(reward)
				if err != nil {
					return nil, err
				}
				phase.Rewards = append(phase.Rewards, rows...)
			}
			if len(phase.Dungeons) == 0 || len(phase.Events) == 0 {
				return nil, fmt.Errorf("empty legacy raid %s phase %d", mode.name, index)
			}
			r.Modes[mode.name] = append(r.Modes[mode.name], phase)
		}
	}
	if len(r.Modes) == 0 {
		return nil, fmt.Errorf("no legacy raid modes")
	}
	return r, nil
}

func ImportLegacyRaidRules(a *pvf.Archive, path string) (*LegacyRaidRules, error) {
	s, err := ResolveScript(a, path)
	if err != nil {
		return nil, err
	}
	symbols, err := ResolveScript(a, path+".symbol")
	if err != nil {
		return nil, err
	}
	return ParseLegacyRaidRules(s, symbols)
}
