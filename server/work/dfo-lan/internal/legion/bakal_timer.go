package legion

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// N583 is the native raid timer map (+f8), separate from persistent N570
// symbols (+d8). Official20261009 s9 has136 timer records: id,sub,absolute
// expiry,source duration (four LE u32). Cancellation carries zero/zero.
func BakalTimerFrame(id, sub uint32, until time.Time, seconds uint32) ([]byte, error) {
	expiry := int64(0)
	if seconds != 0 {
		expiry = until.Unix()
		if expiry < 0 || expiry > math.MaxUint32 {
			return nil, fmt.Errorf("raid timer expiry outside native u32")
		}
	}
	p := make([]byte, 16)
	binary.LittleEndian.PutUint32(p, id)
	binary.LittleEndian.PutUint32(p[4:], sub)
	binary.LittleEndian.PutUint32(p[8:], uint32(expiry))
	binary.LittleEndian.PutUint32(p[12:], seconds)
	return p, nil
}
