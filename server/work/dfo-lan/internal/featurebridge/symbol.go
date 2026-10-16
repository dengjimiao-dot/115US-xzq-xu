package featurebridge

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"
)

// These messages belong to the local extension, never the native packet stream.
const (
	SymbolSnapshot uint16 = 3
	SymbolExchange uint16 = 4
	SymbolRefresh  uint16 = 5
	SymbolGrowth   uint16 = 6
)

// Flags bit 0 is authoritative usability; bits 8..15 carry source item rarity.
type SymbolItem struct{ Template, ExpireTime, Flags uint32 }

func DecodeSymbolRefresh(p []byte) (SymbolRequest, error) {
	if len(p) != 80 {
		return SymbolRequest{}, fmt.Errorf("symbol refresh size")
	}
	r := SymbolRequest{Role: binary.LittleEndian.Uint64(p), Sequence: binary.LittleEndian.Uint64(p[8:]), Source: string(p[16:])}
	if r.Role == 0 || !hexString(r.Source, 64) {
		return SymbolRequest{}, fmt.Errorf("symbol refresh identity")
	}
	return r, nil
}

type SymbolGroup struct {
	Name       string
	Mask       uint16
	Items      []SymbolItem
	Attributes string
	Star       *SymbolStar
}
type SymbolStar struct {
	Grade, Level, Fame uint32
	Fixed, Resonance   [2]string
	PerEmblem          [4][2]string
}
type SymbolBagItem struct {
	Slot, Mask                   uint16
	Template, Amount, ExpireTime uint32
}
type SymbolView struct {
	Role, Sequence             uint64
	Source                     string
	Groups                     []SymbolGroup
	Bag                        []SymbolBagItem
	Attributes, BuffAttributes string
	Powers                     [][2]uint32
	Growth                     *SymbolGrowthView
}
type SymbolGrowthOption struct {
	Operation, Slot            uint16
	Template, Argument, Amount uint32
	Label                      string
	UI                         SymbolGrowthUIQuote
}
type SymbolGrowthUIQuote struct {
	MaterialTemplate, MaterialAmount, Points, SuccessRate, RateUnit uint32
	Results                                                         [][2]uint32
}
type SymbolGrowthUI struct {
	MaterialTemplate, FailRewardTemplate uint32
	MaterialBalance, FailRewardBalance   uint64
	Lights                               []uint32
}
type SymbolGrowthVisual struct {
	Operation, Outcome uint16
	Item, Amount       uint32
}
type SymbolGrowthView struct {
	Visual                              *SymbolGrowthVisual
	Materials, Result, Progress, Lights string
	Options                             []SymbolGrowthOption
	UI                                  *SymbolGrowthUI
}
type SymbolGrowthRequest struct {
	Role, Sequence             uint64
	Source                     string
	Operation, Slot            uint16
	Template, Argument, Amount uint32
}

func DecodeSymbolGrowthRequest(p []byte) (SymbolGrowthRequest, error) {
	var r SymbolGrowthRequest
	if len(p) != 96 {
		return r, fmt.Errorf("growth request width")
	}
	r = SymbolGrowthRequest{binary.LittleEndian.Uint64(p), binary.LittleEndian.Uint64(p[8:]), string(p[16:80]), binary.LittleEndian.Uint16(p[80:]), binary.LittleEndian.Uint16(p[82:]), binary.LittleEndian.Uint32(p[84:]), binary.LittleEndian.Uint32(p[88:]), binary.LittleEndian.Uint32(p[92:])}
	if r.Role == 0 || !hexString(r.Source, 64) || r.Operation < 1 || r.Operation > 8 {
		return SymbolGrowthRequest{}, fmt.Errorf("growth request identity")
	}
	return r, nil
}

type SymbolRequest struct {
	Role, Sequence   uint64
	Source, Group    string
	Socket           uint16
	ExpectedTemplate uint32
	BagSlot          uint16
	Template         uint32
}

func symbolGroupName(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if c != '_' && (c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}
func DecodeSymbolRequest(p []byte) (SymbolRequest, error) {
	var r SymbolRequest
	if len(p) < 94 {
		return r, fmt.Errorf("short symbol request")
	}
	r.Role = binary.LittleEndian.Uint64(p)
	r.Sequence = binary.LittleEndian.Uint64(p[8:])
	r.Source = string(p[16:80])
	n := int(binary.LittleEndian.Uint16(p[80:]))
	if len(p) != 94+n {
		return SymbolRequest{}, fmt.Errorf("symbol request size")
	}
	r.Group = string(p[82 : 82+n])
	p = p[82+n:]
	r.Socket = binary.LittleEndian.Uint16(p)
	r.ExpectedTemplate = binary.LittleEndian.Uint32(p[2:])
	r.BagSlot = binary.LittleEndian.Uint16(p[6:])
	r.Template = binary.LittleEndian.Uint32(p[8:])
	if r.Role == 0 || !hexString(r.Source, 64) || !symbolGroupName(r.Group) {
		return SymbolRequest{}, fmt.Errorf("invalid symbol identity")
	}
	return r, nil
}
func (v SymbolView) Bytes() ([]byte, error) {
	if v.Role == 0 || !hexString(v.Source, 64) || len(v.Groups) == 0 || len(v.Groups) > 16 || len(v.Bag) > 1024 {
		return nil, fmt.Errorf("invalid symbol view")
	}
	b := new(bytes.Buffer)
	put := func(x any) { _ = binary.Write(b, binary.LittleEndian, x) }
	put(v.Role)
	put(v.Sequence)
	b.WriteString(v.Source)
	put(uint16(len(v.Groups)))
	put(uint16(len(v.Bag)))
	seen := map[string]bool{}
	for _, g := range v.Groups {
		if !symbolGroupName(g.Name) || seen[g.Name] || g.Mask == 0 || len(g.Items) == 0 || len(g.Items) > 256 {
			return nil, fmt.Errorf("invalid symbol group")
		}
		seen[g.Name] = true
		put(uint16(len(g.Name)))
		b.WriteString(g.Name)
		put(g.Mask)
		put(uint16(len(g.Items)))
		for _, i := range g.Items {
			if i.Template == 1 || i.Flags & ^uint32(0xffff01) != 0 || i.Template == 0 && (i.ExpireTime != 0 || i.Flags != 0) {
				return nil, fmt.Errorf("invalid socket item")
			}
			put(i.Template)
			put(i.ExpireTime)
			put(i.Flags)
		}
	}
	slots := map[uint16]bool{}
	for _, i := range v.Bag {
		if slots[i.Slot] || i.Mask == 0 || i.Template < 2 || i.Amount == 0 {
			return nil, fmt.Errorf("invalid symbol bag row")
		}
		slots[i.Slot] = true
		put(i.Slot)
		put(i.Mask)
		put(i.Template)
		put(i.Amount)
		put(i.ExpireTime)
	}
	if v.Attributes != "" || v.BuffAttributes != "" || len(v.Powers) > 0 || v.Growth != nil {
		put(uint32(0x31544153)) // SAT1: optional, bounded UTF-8 display trailer.
		texts := []string{v.Attributes, v.BuffAttributes}
		for _, g := range v.Groups {
			texts = append(texts, g.Attributes)
		}
		for _, s := range texts {
			if len(s) > 4096 || !utf8.ValidString(s) || bytes.IndexByte([]byte(s), 0) >= 0 {
				return nil, fmt.Errorf("invalid symbol attributes")
			}
			put(uint16(len(s)))
			b.WriteString(s)
		}
	}
	if len(v.Powers) > 0 || v.Growth != nil {
		if len(v.Powers) > 64 {
			return nil, fmt.Errorf("too many symbol powers")
		}
		put(uint32(0x31525353))
		put(uint16(len(v.Powers))) // SSR1
		for _, pair := range v.Powers {
			if pair[0] < 2 || pair[1] < 2 {
				return nil, fmt.Errorf("invalid native star option")
			}
			put(pair[0])
			put(pair[1])
		}
		for _, g := range v.Groups {
			if g.Star == nil {
				put(uint8(0))
				continue
			}
			s := g.Star
			if s.Grade > 3 || s.Level < 1 || s.Level > 5 {
				return nil, fmt.Errorf("invalid star progression")
			}
			put(uint8(1))
			put(s.Grade)
			put(s.Level)
			put(s.Fame)
			texts := []string{s.Fixed[0], s.Fixed[1], s.Resonance[0], s.Resonance[1]}
			for _, pair := range s.PerEmblem {
				texts = append(texts, pair[:]...)
			}
			for _, s := range texts {
				if len(s) > 4096 || !utf8.ValidString(s) || bytes.IndexByte([]byte(s), 0) >= 0 {
					return nil, fmt.Errorf("invalid star text")
				}
				put(uint16(len(s)))
				b.WriteString(s)
			}
		}
	}
	if v.Growth != nil {
		put(uint32(0x31524753)) // SGR1: paired local growth extension.
		g := v.Growth
		str := func(s string) error {
			if len(s) > 4096 || !utf8.ValidString(s) || bytes.IndexByte([]byte(s), 0) >= 0 {
				return fmt.Errorf("growth text invalid")
			}
			put(uint16(len(s)))
			b.WriteString(s)
			return nil
		}
		for _, s := range []string{g.Materials, g.Result, g.Progress, g.Lights} {
			if e := str(s); e != nil {
				return nil, e
			}
		}
		if len(g.Options) > 1024 {
			return nil, fmt.Errorf("growth option count")
		}
		put(uint16(len(g.Options)))
		for _, o := range g.Options {
			if o.Operation < 1 || o.Operation > 8 {
				return nil, fmt.Errorf("growth option kind")
			}
			put(o.Operation)
			put(o.Slot)
			put(o.Template)
			put(o.Argument)
			put(o.Amount)
			if e := str(o.Label); e != nil {
				return nil, e
			}
		}
		if u := g.UI; u != nil {
			if u.MaterialTemplate < 2 || u.FailRewardTemplate < 2 || (len(u.Lights) != 0 && len(u.Lights) != 10) {
				return nil, fmt.Errorf("growth UI identity or lights")
			}
			put(uint32(0x31554753)) // SGU1: paired presentation fields, no request changes.
			put(u.MaterialTemplate)
			put(u.FailRewardTemplate)
			put(u.MaterialBalance)
			put(u.FailRewardBalance)
			put(uint16(len(u.Lights)))
			for _, color := range u.Lights {
				if color > 4 {
					return nil, fmt.Errorf("growth UI light color")
				}
				put(uint8(color))
			}
			put(uint16(len(g.Options)))
			for _, o := range g.Options {
				q := o.UI
				if len(q.Results) > 8 || q.Points > 100 || (q.SuccessRate > 0 && (q.RateUnit == 0 || q.SuccessRate > q.RateUnit)) {
					return nil, fmt.Errorf("growth UI quote bounds")
				}
				put(q.MaterialTemplate)
				put(q.MaterialAmount)
				put(q.Points)
				put(q.SuccessRate)
				put(q.RateUnit)
				put(uint16(len(q.Results)))
				for _, result := range q.Results {
					put(result[0])
					put(result[1])
				}
			}
		}
		if a := g.Visual; a != nil {
			if a.Operation < 1 || a.Operation > 8 || a.Outcome < 1 || a.Outcome > 4 || a.Item == 1 {
				return nil, fmt.Errorf("growth visual bounds")
			}
			put(uint32(0x31564753))
			put(a.Operation)
			put(a.Outcome)
			put(a.Item)
			put(a.Amount)
		}
	}
	if b.Len() > MaxBody {
		return nil, io.ErrShortBuffer
	}
	return b.Bytes(), nil
}
