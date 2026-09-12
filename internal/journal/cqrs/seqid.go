package cqrs

import (
	"encoding/binary"
	"fmt"

	"github.com/larsartmann/go-cqrs-lite/id/v4"
	"github.com/oklog/ulid/v2"
)

// Synthetic event-ID layout inside the 16 ULID bytes:
//
//	[0:6]   zero timestamp — the marker that an ID is sequence-derived
//	[6:14]  fact Seq, big-endian
//	[14:16] zero tail
//
// Because every synthetic ID shares the zero timestamp prefix, canonical
// ULID string order equals ascending Seq, and the classic ULID first-char
// sort caveat cannot apply (the first char is constant '0'). A ULID whose
// marker or tail bytes are non-zero was not minted here (random cursor,
// foreign producer); it decodes to no position, which callers must treat
// as "drain nothing" — never as "replay everything".
const (
	seqEpochLen = 6
	seqBytesLen = 8
)

func seqEventID(seq int64) (id.EventID, error) {
	if seq <= 0 {
		return id.EventID{}, fmt.Errorf("cqrs: sequence %d is not positive", seq)
	}

	var raw ulid.ULID
	binary.BigEndian.PutUint64(raw[seqEpochLen:seqEpochLen+seqBytesLen], uint64(seq))

	eventID, err := id.ParseEventID(raw.String())
	if err != nil {
		return id.EventID{}, fmt.Errorf("cqrs: encode sequence %d as event id: %w", seq, err)
	}

	return eventID, nil
}

func seqFromEventID(eventID id.EventID) (int64, bool) {
	raw := eventID.Get()

	for _, b := range raw[:seqEpochLen] {
		if b != 0 {
			return 0, false
		}
	}

	for _, b := range raw[seqEpochLen+seqBytesLen:] {
		if b != 0 {
			return 0, false
		}
	}

	return int64(binary.BigEndian.Uint64(raw[seqEpochLen : seqEpochLen+seqBytesLen])), true
}
