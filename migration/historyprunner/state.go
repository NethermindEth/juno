package historyprunner

import (
	"encoding/binary"
	"fmt"
)

// phase is a step of the migration, run in order.
type phase byte

const (
	// phaseStage copies the history entries to keep into the scratch space.
	phaseStage phase = iota
	// phaseRestoreHistory copies them back after the history buckets are wiped.
	phaseRestoreHistory
	// phaseRestoreLookups rebuilds the hash lookups of the kept blocks.
	phaseRestoreLookups
	phaseDone
)

func (p phase) String() string {
	switch p {
	case phaseStage:
		return "stage history"
	case phaseRestoreHistory:
		return "restore history"
	case phaseRestoreLookups:
		return "restore hash lookups"
	default:
		return "done"
	}
}

// stateHeaderSize is the fixed part of the intermediate state:
//
//	[0]     phase
//	[1]     layout id
//	[2:10]  oldestBlockKept (uint64 BE), pinned on the first run so a config
//	        change between runs does not retarget the cutoff mid-flight
//	[10:18] next block whose hash lookups to restore (uint64 BE)
//	[18:]   next key of the stage or restore scan, empty when not started
const stateHeaderSize = 18

func (m *Migrator) encodeState() []byte {
	buf := make([]byte, stateHeaderSize, stateHeaderSize+len(m.cursor))
	buf[0] = byte(m.phase)
	buf[1] = m.layout.id
	binary.BigEndian.PutUint64(buf[2:10], m.oldestBlockKept)
	binary.BigEndian.PutUint64(buf[10:18], m.lookupsFrom)
	return append(buf, m.cursor...)
}

func (m *Migrator) decodeState(state []byte) error {
	if len(state) < stateHeaderSize {
		return fmt.Errorf("historyprunner: invalid intermediate state %x", state)
	}
	l, err := layoutByID(state[1])
	if err != nil {
		return err
	}
	m.phase = phase(state[0])
	m.layout = l
	m.oldestBlockKept = binary.BigEndian.Uint64(state[2:10])
	m.lookupsFrom = binary.BigEndian.Uint64(state[10:18])
	if len(state) > stateHeaderSize {
		m.cursor = state[stateHeaderSize:]
	}
	return nil
}
