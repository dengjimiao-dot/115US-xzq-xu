package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// Current144CE6990 reads material display rows of exactly37 bytes.
// The witnessed material rows have zero auxiliary fields. Equipment previews
// need their own proved attributes; this type does not invent them.
type BakalRewardMaterial struct {
	Template uint32
	Amount   uint16
}
type BakalRewardActor struct {
	Actor uint16
	Name  string
	// Native1451B3D70: signed card-object index; -1 selects no card object.
	CardIndex int8
	Materials []BakalRewardMaterial
}

func BakalRewardList115(branch byte, actors []BakalRewardActor) ([]byte, error) {
	if (branch != 0 && branch != 3) || len(actors) > 255 {
		return nil, fmt.Errorf("unsupported native Bakal reward-list branch/size")
	}
	p := binary.LittleEndian.AppendUint32([]byte{branch}, 0xffffffff)
	p = append(p, byte(len(actors)))
	for _, a := range actors {
		if a.Actor == 0 || a.Actor == 65535 || len(a.Name) > 256 || !utf8.ValidString(a.Name) || a.CardIndex < -1 || len(a.Materials) > 255 {
			return nil, fmt.Errorf("invalid native Bakal reward actor")
		}
		p = binary.LittleEndian.AppendUint16(p, a.Actor)
		p = append(p, 0) // reader consumes but does not use this byte
		p = binary.LittleEndian.AppendUint32(p, uint32(len(a.Name)))
		p = append(p, a.Name...)
		p = append(p, byte(a.CardIndex), byte(len(a.Materials)))
		for _, item := range a.Materials {
			if item.Template == 0 || item.Amount == 0 {
				return nil, fmt.Errorf("invalid native Bakal reward material")
			}
			row := make([]byte, 37)
			binary.LittleEndian.PutUint32(row[1:5], item.Template)
			binary.LittleEndian.PutUint16(row[5:7], item.Amount)
			p = append(p, row...)
		}
	}
	return p, nil
}
