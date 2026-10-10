package protocol

import (
	"encoding/binary"
	"fmt"
)

// BakalCommanderPresentation is NOTI2289 mode2. Current reader144CDE830
// copies exactly18 bytes; consumer142543E70 reads mode at0, raid team color
// at1, and commander slot(u32) at2. After matching the local member's team,
// it calls1421F4FD0/1421F4210 to play the resource-defined NPC cut-in.
// Official s9 S1352/S1438/S2270/S2276 and later samples cover all5 slots.
func BakalCommanderPresentation(slot, team byte) ([]byte, error) {
	if slot > 4 || team < 1 || team > 3 {
		return nil, fmt.Errorf("invalid native commander presentation slot/team")
	}
	p := make([]byte, 18)
	p[0], p[1] = 2, team
	binary.LittleEndian.PutUint32(p[2:6], uint32(slot))
	return p, nil
}
