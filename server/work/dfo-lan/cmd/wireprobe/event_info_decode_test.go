package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

// Follow the native reader's variable-width cursor, rather than assuming
// every future record has the captured Ispins record's length.
func decodeOpenEventKeys(t *testing.T, body []byte) []uint16 {
	t.Helper()
	r := bytes.NewReader(body)
	read := func(v any) {
		if err := binary.Read(r, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	var count uint16
	read(&count)
	var ids []uint16
	for i := 0; i < int(count); i++ {
		var id uint16
		var present, flags [1]byte
		read(&id)
		read(&present)
		if present[0] != 1 {
			t.Fatal("missing complete open record")
		}
		read(&flags)
		read(&flags)
		str := func() {
			var n uint32
			read(&n)
			if uint64(n) > uint64(r.Len()) {
				t.Fatal("string crosses record body")
			}
			if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
				t.Fatal(err)
			}
		}
		str()
		str()
		str()
		var start, end uint32
		read(&start)
		read(&end)
		if start >= end {
			t.Fatal("invalid availability window")
		}
		str()
		str()
		read(&flags)
		ids = append(ids, id)
	}
	// The captured table terminates with an empty external-data count.
	var external uint8
	read(&external)
	if external != 0 || r.Len() != 0 {
		t.Fatalf("invalid external tail: count=%d bytes=%d", external, r.Len())
	}
	return ids
}
