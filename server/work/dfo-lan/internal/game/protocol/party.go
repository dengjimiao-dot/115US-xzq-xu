package protocol

import (
	"encoding/binary"
	"fmt"
)

// SoloPartyInfo preserves the upstream 99-byte raid projection when no context
// is supplied. Explicit ordinary-party context uses the native seat projection.
func SoloPartyInfo(actor uint16, contexts ...[2]byte) ([]byte, error) {
	if len(contexts) > 0 {
		return PartyRosterSeats115(1, contexts[0], [4]uint16{actor}, actor, 4)
	}

	if actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid solo party actor")
	}
	p := make([]byte, 99)
	binary.LittleEndian.PutUint16(p, 1)
	binary.LittleEndian.PutUint16(p[4:], 1)
	p[15] = 1 // one-player capacity
	p[61] = 1 // one member
	binary.LittleEndian.PutUint16(p[63:], actor)
	return p, nil

}
