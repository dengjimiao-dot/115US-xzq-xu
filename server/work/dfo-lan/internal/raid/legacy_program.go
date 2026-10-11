package raid

// The CN legacy raid program is translated at the rule/execution boundary.
// There are no CN packet identifiers or reward contents in this executor.
import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"sort"
	"strings"
)

type LegacyInstruction struct {
	Op, Symbol, Operator string
	Values               []pvf.Token
}
type LegacyEvent struct {
	Name, Symbol string
	A, B         int32
}
type legacyRule struct{ when, then []LegacyInstruction }
type LegacyTimerKey struct{ Kind, Key int32 }
type LegacyTimer struct {
	Due   int64
	State string
}
type LegacyEffect struct {
	Kind    string
	Dungeon int32
}
type LegacyView struct {
	Run, Source string
	Now         int64
	Cleared     bool
	Symbols     map[string]int32
	Dungeons    map[int32]string
	Occupants   map[int32][]byte
	Timers      map[LegacyTimerKey]LegacyTimer
	Effects     []LegacyEffect
}

// Bindings are pure, source-specific preparation functions. They receive a
// private view; no database/network side effects are allowed here. Failure
// rolls back the entire cascading event. Missing bindings reject construction.
type LegacyBinding func(*LegacyView, LegacyInstruction) error
type LegacyPhase struct {
	rules           []legacyRule
	source          catalog.LegacyRaidPhase
	state           LegacyView
	bindings        map[string]LegacyBinding
	initialized     bool
	requeueAuthored bool
}

func legacyInts(v []pvf.Token, n int) bool {
	if len(v) != n {
		return false
	}
	for _, t := range v {
		if t.Type != 0 {
			return false
		}
	}
	return true
}
func legacySymbol(r *catalog.LegacyRaidRules, name string) (string, error) {
	s := "[" + strings.ToUpper(name) + "]"
	if _, ok := r.Symbols[s]; !ok {
		return "", fmt.Errorf("undeclared symbol %s", s)
	}
	return s, nil
}
func compileLegacy(r *catalog.LegacyRaidRules, sections []catalog.LegacyRaidSection, condition bool) ([]LegacyInstruction, error) {
	var out []LegacyInstruction
	for i := 0; i < len(sections); i++ {
		row := sections[i]
		ins := LegacyInstruction{Op: row.Name, Values: append([]pvf.Token(nil), row.Values...)}
		nextSymbol := func() error {
			i++
			if i >= len(sections) {
				return fmt.Errorf("truncated %s", row.Name)
			}
			s, err := legacySymbol(r, sections[i].Name)
			ins.Symbol = s
			return err
		}
		if condition {
			switch row.Name {
			case "on change symbol":
				if len(row.Values) != 0 {
					return nil, fmt.Errorf("unexpected symbol event args")
				}
				if err := nextSymbol(); err != nil {
					return nil, err
				}
				if len(sections[i].Values) != 0 {
					return nil, fmt.Errorf("invalid symbol event")
				}
			case "on clear dungeon", "on enter dungeon", "on enter map", "on giveup dungeon", "on kick out dungeon", "on kill boss", "on change dungeon state", "check timer end":
				if len(row.Values) > 2 || !legacyInts(row.Values, len(row.Values)) {
					return nil, fmt.Errorf("invalid event %s", row.Name)
				}
			case "check dungeon state":
				if len(row.Values) != 2 || row.Values[0].Type != 0 || row.Values[1].Type != 6 {
					return nil, fmt.Errorf("invalid dungeon condition")
				}
			case "check dungeon type incount":
				if len(row.Values) != 2 || row.Values[0].Type != 6 || row.Values[1].Type != 0 {
					return nil, fmt.Errorf("invalid occupancy condition")
				}
			case "if":
				if len(row.Values) != 0 {
					return nil, fmt.Errorf("invalid IF")
				}
				if err := nextSymbol(); err != nil {
					return nil, err
				}
				ins.Values = sections[i].Values
				ins.Operator = "="
				if len(ins.Values) == 0 {
					i++
					if i >= len(sections) {
						return nil, fmt.Errorf("missing comparison")
					}
					ins.Operator = sections[i].Name
					ins.Values = sections[i].Values
				}
				if !legacyInts(ins.Values, 1) {
					return nil, fmt.Errorf("invalid predicate operand")
				}
				switch ins.Operator {
				case "=", "==", "!=", "<", ">", "<=", ">=":
				default:
					return nil, fmt.Errorf("unknown comparison %s", ins.Operator)
				}
			default:
				return nil, fmt.Errorf("unbound legacy predicate %s", row.Name)
			}
		} else {
			switch row.Name {
			case "set":
				if len(row.Values) != 0 {
					return nil, fmt.Errorf("invalid SET")
				}
				if err := nextSymbol(); err != nil {
					return nil, err
				}
				ins.Values = sections[i].Values
				if len(ins.Values) == 0 {
					i++
					if i >= len(sections) {
						return nil, fmt.Errorf("missing arithmetic")
					}
					ins.Operator = sections[i].Name
					ins.Values = sections[i].Values
					if ins.Operator != "+" && ins.Operator != "-" && ins.Operator != "sub" {
						return nil, fmt.Errorf("unknown arithmetic")
					}
					if len(ins.Values) == 0 {
						i++
						if i >= len(sections) || len(sections[i].Values) != 0 {
							return nil, fmt.Errorf("invalid symbol operand")
						}
						s, err := legacySymbol(r, sections[i].Name)
						if err != nil {
							return nil, err
						}
						ins.Values = []pvf.Token{{Type: 3, Text: s}}
					}
				}
				if len(ins.Values) != 1 || (ins.Values[0].Type != 0 && ins.Values[0].Type != 3) {
					return nil, fmt.Errorf("invalid assignment")
				}
			case "sub":
				if len(row.Values) != 0 {
					return nil, fmt.Errorf("invalid SUB")
				}
				if err := nextSymbol(); err != nil {
					return nil, err
				}
				if len(sections[i].Values) != 0 {
					return nil, fmt.Errorf("invalid subtraction target")
				}
				i++
				if i >= len(sections) || len(sections[i].Values) != 0 {
					return nil, fmt.Errorf("missing sub operand")
				}
				s, err := legacySymbol(r, sections[i].Name)
				if err != nil {
					return nil, err
				}
				ins.Values = []pvf.Token{{Type: 3, Text: s}}
			case "sub max":
				if len(row.Values) != 0 {
					return nil, fmt.Errorf("invalid SUB MAX")
				}
				ins.Values = nil
				for n := 0; n < 4; n++ {
					i++
					if i >= len(sections) || len(sections[i].Values) != 0 {
						return nil, fmt.Errorf("truncated shared HP expression")
					}
					s, err := legacySymbol(r, sections[i].Name)
					if err != nil {
						return nil, err
					}
					ins.Values = append(ins.Values, pvf.Token{Type: 3, Text: s})
				}
			case "set dungeon state":
				if len(ins.Values) != 2 || ins.Values[0].Type != 0 || ins.Values[1].Type != 6 {
					return nil, fmt.Errorf("invalid state action")
				}
			case "reserve dungeon state":
				if len(ins.Values) != 3 || ins.Values[0].Type != 0 || ins.Values[1].Type != 6 || ins.Values[2].Type != 0 || ins.Values[2].Value <= 0 {
					return nil, fmt.Errorf("invalid reserved state")
				}
			case "set timer":
				if !legacyInts(ins.Values, 3) || ins.Values[2].Value <= 0 {
					return nil, fmt.Errorf("invalid timer")
				}
			case "reset timer":
				if !legacyInts(ins.Values, 2) {
					return nil, fmt.Errorf("invalid reset")
				}
			case "kick out dungeon", "kick out dungeon no penalty":
				if !legacyInts(ins.Values, 1) {
					return nil, fmt.Errorf("invalid evacuation")
				}
			case "kick out character":
				if len(ins.Values) > 1 || !legacyInts(ins.Values, len(ins.Values)) {
					return nil, fmt.Errorf("invalid character evacuation")
				}
			case "clear phase":
				if len(ins.Values) != 0 {
					return nil, fmt.Errorf("invalid clear")
				}
			default:
				// The exact source operands are retained. A binding must be supplied;
				// unknown operations are never converted to a successful no-op.
			}
		}
		ins.Values = append([]pvf.Token(nil), ins.Values...)
		out = append(out, ins)
	}
	return out, nil
}

func NewLegacyPhase(r *catalog.LegacyRaidRules, mode string, index int, run string, bindings map[string]LegacyBinding) (*LegacyPhase, error) {
	return NewLegacyPhaseAt(r, mode, index, run, LegacyStart{}, bindings)
}

// LegacyStart is supplied by the owning run, not by an untrusted client frame.
// Now uses milliseconds, matching Apply. SingleType is the resolved route value
// for the source's SINGLE RAID TYPE symbol, not an inferred mode ordinal.
type LegacyStart struct {
	Now        int64
	SingleType int32
}

func NewLegacyPhaseAt(r *catalog.LegacyRaidRules, mode string, index int, run string, start LegacyStart, bindings map[string]LegacyBinding) (*LegacyPhase, error) {
	if r == nil || run == "" || r.SHA256 == "" || index < 0 || index >= len(r.Modes[mode]) {
		return nil, fmt.Errorf("missing owned raid/source/mode")
	}
	if start.Now < 0 || start.SingleType < 0 {
		return nil, fmt.Errorf("invalid raid start clock/route")
	}
	// Freeze all caller-owned slices/maps; later catalog mutation cannot change
	// a live phase or its source-bound behavior.
	p := &LegacyPhase{source: catalog.LegacyRaidPhase{Index: index, Dungeons: map[int32]catalog.LegacyRaidDungeon{}}, bindings: map[string]LegacyBinding{}}
	p.state = LegacyView{Run: run, Source: r.SHA256, Symbols: map[string]int32{}, Dungeons: map[int32]string{}, Occupants: map[int32][]byte{}, Timers: map[LegacyTimerKey]LegacyTimer{}}
	for s := range r.Symbols {
		p.state.Symbols[s] = 0
	}
	p.state.Now = start.Now
	if _, ok := p.state.Symbols["[SINGLE RAID TYPE]"]; ok {
		p.state.Symbols["[SINGLE RAID TYPE]"] = start.SingleType
	} else if start.SingleType != 0 {
		return nil, fmt.Errorf("route supplied without source symbol")
	}
	for k, v := range bindings {
		p.bindings[k] = v
	}
	p.requeueAuthored = strings.EqualFold(r.Path, "etc/raid/siroco/siroco.etc")
	phase := r.Modes[mode][index]
	for id, d := range phase.Dungeons {
		p.source.Dungeons[id] = d
		p.state.Dungeons[id] = d.InitialState
		p.state.Symbols[d.HPSymbol] = d.HP
	}
	for j, event := range phase.Events {
		when, err := compileLegacy(r, event.Trigger, true)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", j, err)
		}
		then, err := compileLegacy(r, event.Behavior, false)
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", j, err)
		}
		for _, ins := range then {
			switch ins.Op {
			case "set", "sub", "set dungeon state", "reserve dungeon state", "set timer", "reset timer", "kick out dungeon", "kick out dungeon no penalty", "kick out character", "clear phase":
			default:
				if p.bindings[ins.Op] == nil {
					return nil, fmt.Errorf("event %d requires binding [%s]", j, ins.Op)
				}
			}
		}
		p.rules = append(p.rules, legacyRule{when, then})
	}
	if len(p.rules) == 0 {
		return nil, fmt.Errorf("missing raid program")
	}
	if err := p.validateView(p.state); err != nil {
		return nil, err
	}
	if err := p.Apply(run, r.SHA256, start.Now, LegacyEvent{Name: "initial"}); err != nil {
		return nil, err
	}
	return p, nil
}
func cloneLegacy(v LegacyView) LegacyView {
	n := v
	n.Symbols = map[string]int32{}
	for k, x := range v.Symbols {
		n.Symbols[k] = x
	}
	n.Dungeons = map[int32]string{}
	for k, x := range v.Dungeons {
		n.Dungeons[k] = x
	}
	n.Occupants = map[int32][]byte{}
	for k, x := range v.Occupants {
		n.Occupants[k] = append([]byte(nil), x...)
	}
	n.Timers = map[LegacyTimerKey]LegacyTimer{}
	for k, x := range v.Timers {
		n.Timers[k] = x
	}
	n.Effects = append([]LegacyEffect(nil), v.Effects...)
	return n
}
func (p *LegacyPhase) View() LegacyView { return cloneLegacy(p.state) }
func (p *LegacyPhase) matches(v LegacyView, code []LegacyInstruction, e LegacyEvent) bool {
	eventBound := false
	for _, i := range code {
		switch i.Op {
		case "if":
			a, b := v.Symbols[i.Symbol], i.Values[0].Value
			ok := false
			switch i.Operator {
			case "=", "==":
				ok = a == b
			case "!=":
				ok = a != b
			case "<":
				ok = a < b
			case ">":
				ok = a > b
			case "<=":
				ok = a <= b
			case ">=":
				ok = a >= b
			}
			if !ok {
				return false
			}
		case "check dungeon state":
			if v.Dungeons[i.Values[0].Value] != i.Values[1].Text {
				return false
			}
		case "check dungeon type incount":
			n := 0
			for id, d := range p.source.Dungeons {
				if d.Kind == i.Values[0].Text {
					n += len(v.Occupants[id])
				}
			}
			if int32(n) != i.Values[1].Value {
				return false
			}
		default:
			eventBound = true
			if e.Name != i.Op {
				return false
			}
			if i.Op == "on change symbol" {
				if e.Symbol != i.Symbol {
					return false
				}
			} else if (len(i.Values) > 0 && i.Values[0].Value != e.A) || (len(i.Values) > 1 && i.Values[1].Value != e.B) {
				return false
			}
		}
	}
	return eventBound || e.Name == "initial"
}
func legacySetState(v *LegacyView, id int32, state string, queue *[]LegacyEvent) error {
	if _, ok := v.Dungeons[id]; !ok {
		return fmt.Errorf("unknown source dungeon %d", id)
	}
	switch state {
	case "open", "hide", "close", "clear":
	default:
		return fmt.Errorf("unknown dungeon state %q", state)
	}
	if v.Dungeons[id] != state {
		v.Dungeons[id] = state
		*queue = append(*queue, LegacyEvent{Name: "on change dungeon state", A: id})
	}
	return nil
}
func legacySetSymbol(v *LegacyView, name string, value int64, queue *[]LegacyEvent) error {
	if _, ok := v.Symbols[name]; !ok || value < math.MinInt32 || value > math.MaxInt32 {
		return fmt.Errorf("invalid/overflow source symbol %s", name)
	}
	if v.Symbols[name] != int32(value) {
		v.Symbols[name] = int32(value)
		*queue = append(*queue, LegacyEvent{Name: "on change symbol", Symbol: name})
	}
	return nil
}
func (p *LegacyPhase) execute(v *LegacyView, code []LegacyInstruction, queue *[]LegacyEvent) error {
	for _, i := range code {
		switch i.Op {
		case "set", "sub":
			value := int64(i.Values[0].Value)
			if i.Values[0].Type == 3 {
				value = int64(v.Symbols[i.Values[0].Text])
			}
			if i.Op == "sub" || i.Operator == "-" || i.Operator == "sub" {
				value = int64(v.Symbols[i.Symbol]) - value
			} else if i.Operator == "+" {
				value += int64(v.Symbols[i.Symbol])
			}
			same := int64(v.Symbols[i.Symbol]) == value
			if err := legacySetSymbol(v, i.Symbol, value, queue); err != nil {
				return err
			}
			if same && p.requeueAuthored {
				*queue = append(*queue, LegacyEvent{Name: "on change symbol", Symbol: i.Symbol})
			}
		case "set dungeon state":
			if err := legacySetState(v, i.Values[0].Value, i.Values[1].Text, queue); err != nil {
				return err
			}
		case "reserve dungeon state":
			id, seconds := i.Values[0].Value, i.Values[2].Value
			if !legacyStateValid(i.Values[1].Text) {
				return fmt.Errorf("invalid reserved dungeon state")
			}
			if _, ok := v.Dungeons[id]; !ok {
				return fmt.Errorf("reserved unknown dungeon")
			}
			if v.Now > math.MaxInt64-int64(seconds)*1000 {
				return fmt.Errorf("timer overflow")
			}
			v.Timers[LegacyTimerKey{-1, id}] = LegacyTimer{v.Now + int64(seconds)*1000, i.Values[1].Text}
		case "set timer":
			seconds := int64(i.Values[2].Value) * 1000
			if v.Now > math.MaxInt64-seconds {
				return fmt.Errorf("timer overflow")
			}
			v.Timers[LegacyTimerKey{i.Values[0].Value, i.Values[1].Value}] = LegacyTimer{Due: v.Now + seconds}
		case "reset timer":
			delete(v.Timers, LegacyTimerKey{i.Values[0].Value, i.Values[1].Value})
		case "kick out dungeon", "kick out dungeon no penalty":
			id := i.Values[0].Value
			if _, ok := v.Dungeons[id]; !ok {
				return fmt.Errorf("unknown evacuation room")
			}
			delete(v.Occupants, id)
			v.Effects = append(v.Effects, LegacyEffect{i.Op, id})
		case "kick out character":
			v.Occupants = map[int32][]byte{}
			v.Effects = append(v.Effects, LegacyEffect{Kind: i.Op})
		case "clear phase":
			v.Cleared = true
			v.Effects = append(v.Effects, LegacyEffect{Kind: i.Op})
		default:
			before := cloneLegacy(*v)
			if err := p.bindings[i.Op](v, LegacyInstruction{Op: i.Op, Symbol: i.Symbol, Operator: i.Operator, Values: append([]pvf.Token(nil), i.Values...)}); err != nil {
				return err
			}
			if v.Run != before.Run || v.Source != before.Source || v.Now != before.Now {
				return fmt.Errorf("binding changed owned identity/clock")
			}
			if err := p.validateView(*v); err != nil {
				return err
			}
			for key, timer := range v.Timers {
				if old, ok := before.Timers[key]; (!ok || old != timer) && timer.Due <= v.Now {
					return fmt.Errorf("binding timer must advance time")
				}
			}
			// Detach retained binding map/slice references before another action.
			*v = cloneLegacy(*v)
			// Bound actions may not create undeclared symbols or dungeon identities.
			if len(v.Symbols) != len(before.Symbols) || len(v.Dungeons) != len(before.Dungeons) {
				return fmt.Errorf("binding changed source schema")
			}
			symbolKeys := make([]string, 0, len(v.Symbols))
			for k := range v.Symbols {
				symbolKeys = append(symbolKeys, k)
			}
			sort.Strings(symbolKeys)
			for _, k := range symbolKeys {
				x := v.Symbols[k]
				old, ok := before.Symbols[k]
				if !ok {
					return fmt.Errorf("binding added symbol")
				}
				if old != x {
					*queue = append(*queue, LegacyEvent{Name: "on change symbol", Symbol: k})
				}
			}
			roomKeys := make([]int32, 0, len(v.Dungeons))
			for k := range v.Dungeons {
				roomKeys = append(roomKeys, k)
			}
			sort.Slice(roomKeys, func(a, b int) bool { return roomKeys[a] < roomKeys[b] })
			for _, k := range roomKeys {
				x := v.Dungeons[k]
				switch x {
				case "open", "hide", "close", "clear":
				default:
					return fmt.Errorf("binding set invalid state")
				}
				old, ok := before.Dungeons[k]
				if !ok {
					return fmt.Errorf("binding added dungeon")
				}
				if old != x {
					*queue = append(*queue, LegacyEvent{Name: "on change dungeon state", A: k})
				}
			}
		}
	}
	return nil
}
func (p *LegacyPhase) drain(v *LegacyView, queue []LegacyEvent) error {
	for n := 0; len(queue) > 0 && !v.Cleared; n++ {
		if n >= 10000 {
			return fmt.Errorf("legacy event cycle limit")
		}
		e := queue[0]
		queue = queue[1:]
		for _, r := range p.rules {
			if p.matches(*v, r.when, e) {
				if err := p.execute(v, r.then, &queue); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Apply accepts domain events only; the game gateway must prove ownership and
// actual combat completion before calling it. Client raw values are not proof.
func (p *LegacyPhase) Apply(run, source string, now int64, event LegacyEvent) error {
	if run != p.state.Run || source != p.state.Source || p.state.Cleared || now < p.state.Now {
		return fmt.Errorf("stale/terminal raid event")
	}
	if event.Name == "initial" && p.initialized {
		return fmt.Errorf("repeated initialization")
	}
	next := cloneLegacy(p.state)
	// Repeatedly select the actual earliest due timer: cascades may cancel or
	// reschedule other timers before the supplied time is reached.
	for ticks := 0; ; ticks++ {
		if ticks >= 10000 {
			return fmt.Errorf("timer cascade limit")
		}
		keys := make([]LegacyTimerKey, 0, len(next.Timers))
		for k, t := range next.Timers {
			if t.Due <= now {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 || next.Cleared {
			break
		}
		sort.Slice(keys, func(a, b int) bool {
			x, y := keys[a], keys[b]
			if next.Timers[x].Due != next.Timers[y].Due {
				return next.Timers[x].Due < next.Timers[y].Due
			}
			if x.Kind != y.Kind {
				return x.Kind < y.Kind
			}
			return x.Key < y.Key
		})
		k := keys[0]
		timer := next.Timers[k]
		delete(next.Timers, k)
		next.Now = timer.Due
		q := []LegacyEvent{}
		if k.Kind == -1 {
			if err := legacySetState(&next, k.Key, timer.State, &q); err != nil {
				return err
			}
		} else {
			q = append(q, LegacyEvent{Name: "check timer end", A: k.Kind, B: k.Key})
		}
		if err := p.drain(&next, q); err != nil {
			return err
		}
	}
	if !next.Cleared {
		next.Now = now
		if event.Name != "" {
			if err := p.drain(&next, []LegacyEvent{event}); err != nil {
				return err
			}
		}
	}
	p.state = cloneLegacy(next)
	p.initialized = true
	return nil
}
