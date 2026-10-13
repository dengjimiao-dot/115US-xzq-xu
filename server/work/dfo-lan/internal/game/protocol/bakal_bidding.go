package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

type BakalBiddingParticipant struct {
	Actor    uint16
	Eligible bool
}
type BakalBiddingItem struct {
	Template, Amount uint32
	Flags            [3]byte
	Detail           []byte
	Final            bool
}

// ACK's common status/error dispatcher supplies false/u16 to1425C2DC0.
// Its refusal branch clears the pending-send latch before displaying errors.
func BakalBiddingFailure(code uint16) []byte {
	return binary.LittleEndian.AppendUint16([]byte{0}, code)
}

// Native1425C3970/3190/3B60 consume these source-owned bidding tables.
func BakalBiddingParticipants(rows []BakalBiddingParticipant) ([]byte, error) {
	p := binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
	for _, r := range rows {
		if r.Actor == 0 || r.Actor == 65535 {
			return nil, fmt.Errorf("invalid bidding participant")
		}
		p = binary.LittleEndian.AppendUint16(p, r.Actor)
		b := byte(0)
		if r.Eligible {
			b = 1
		}
		p = append(p, b)
	}
	return p, nil
}
func BakalBiddingItems(rows []BakalBiddingItem) ([]byte, error) {
	p := binary.LittleEndian.AppendUint32(nil, uint32(len(rows)))
	for _, r := range rows {
		if r.Template == 0 || r.Amount == 0 || len(r.Detail) != 0 && len(r.Detail) != 181 {
			return nil, fmt.Errorf("invalid native bidding lot")
		}
		p = binary.LittleEndian.AppendUint32(p, r.Template)
		p = binary.LittleEndian.AppendUint32(p, r.Amount)
		p = append(p, r.Flags[:]...)
		if len(r.Detail) == 0 {
			p = append(p, 0)
		} else {
			p = append(p, 1)
			p = append(p, r.Detail...)
		}
		b := byte(0)
		if r.Final {
			b = 1
		}
		p = append(p, b)
	}
	return p, nil
}
func BakalBiddingStart(item, count, price uint32) ([]byte, error) {
	if item == 0 || count == 0 || price == 0 {
		return nil, fmt.Errorf("invalid bidding start")
	}
	p := binary.LittleEndian.AppendUint32(nil, item)
	p = binary.LittleEndian.AppendUint32(p, count)
	return binary.LittleEndian.AppendUint32(p, price), nil
}

// Native1425C6830's static bid path emits1+u32. Current live20261013 also
// records0+310034448 while the user explicitly confirmed bidding, not leaving.
// Accept both witnessed bid variants with positive amounts. This does not
// implement a guessed withdrawal action. Static/live discrepancy is retained
// in the reconstruction record rather than claimed as a native confirmation rule.
func DecodeBakalBid(p []byte) (uint32, error) {
	if len(p) != 5 && len(p) != 8 && len(p) != 16 {
		return 0, fmt.Errorf("invalid native bid length")
	}
	if p[0] > 1 {
		return 0, fmt.Errorf("unsupported native bidding action")
	}
	for _, b := range p[5:] {
		if b != 0 {
			return 0, fmt.Errorf("nonzero bid padding")
		}
	}
	amount := binary.LittleEndian.Uint32(p[1:5])
	if amount == 0 {
		return 0, fmt.Errorf("zero native bid")
	}
	return amount, nil
}

// Native ACK1902 is also the server's per-lot terminal broadcast. outcome0
// updates the bid,1 selects a winner,2 ends unsold. The last u32 is the
// remaining countdown in milliseconds, not the source inter-lot delay.
func BakalBiddingResult(actor uint16, name string, lot byte, item, count, price uint32, outcome byte, remaining uint32) ([]byte, error) {
	if item == 0 || count == 0 || outcome > 2 || len(name) > 256 || !utf8.ValidString(name) || outcome != 2 && actor == 0 {
		return nil, fmt.Errorf("invalid native bidding result")
	}
	p := binary.LittleEndian.AppendUint16([]byte{1}, actor)
	p = binary.LittleEndian.AppendUint32(p, uint32(len(name)))
	p = append(p, name...)
	p = append(p, lot)
	p = binary.LittleEndian.AppendUint32(p, item)
	p = binary.LittleEndian.AppendUint32(p, count)
	p = binary.LittleEndian.AppendUint32(p, price)
	p = append(p, outcome)
	return binary.LittleEndian.AppendUint32(p, remaining), nil
}
