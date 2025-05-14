package db

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/consensus/types"
	db "github.com/NethermindEth/juno/db"
	"github.com/fxamacker/cbor/v2"
)

const NumBytesForHeight = 4

// walMsgCount tracks the number of wal entries at the current height
type walMsgCount uint32

type walEntry[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	Type  types.MessageType      `cbor:"type"`
	Entry types.Message[V, H, A] `cbor:"data"` // cbor serialised Msg or Timeout
}

func (w *walEntry[V, H, A]) UnmarshalCBOR(data []byte) error {
	var wrapperType struct {
		Type    types.MessageType `cbor:"type"`
		RawData cbor.RawMessage   `cbor:"data"` // Golang can't unmarshal to an interface
	}
	if err := cbor.Unmarshal(data, &wrapperType); err != nil {
		return err
	}
	w.Type = wrapperType.Type
	switch wrapperType.Type {
	case types.MessageTypeTimeout:
		var to types.Timeout
		if err := cbor.Unmarshal(wrapperType.RawData, &to); err != nil {
			return err
		}
		w.Entry = to
	case types.MessageTypeProposal:
		var proposal types.Proposal[V, H, A]
		if err := cbor.Unmarshal(wrapperType.RawData, &proposal); err != nil {
			return err
		}
		w.Entry = proposal
	case types.MessageTypePrevote:
		var vote types.Prevote[H, A]
		if err := cbor.Unmarshal(wrapperType.RawData, &vote); err != nil {
			return err
		}
		w.Entry = vote
	case types.MessageTypePrecommit:
		var vote types.Precommit[H, A]
		if err := cbor.Unmarshal(wrapperType.RawData, &vote); err != nil {
			return err
		}
		w.Entry = vote
	default:
		return fmt.Errorf("failed to unmarshal walEntry, unknown type")
	}
	return nil
}

// TendermintDB defines the methods for interacting with the Tendermint WAL database.
type TendermintDB[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// CommitBatch writes the accumulated batch operations to the underlying database.
	CommitBatch() error
	// GetWALMsgs retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	GetWALMsgs(height types.Height) ([]walEntry[V, H, A], error)
	// SetWALEntry schedules the storage of a WAL message in the batch.
	SetWALEntry(entry types.Message[V, H, A]) error
	// DeleteWALMsgs schedules the deletion of all WAL messages for a specific height in the batch.
	DeleteWALMsgs(height types.Height) error
}

// tendermintDB provides database access for Tendermint consensus state.
// We use a Batch to accumulate writes before committing them to the DB.
// This reduces expensive disk I/O. Reads check the batch first.
// WARNING: If the process crashes before Commit(), buffered messages are lost,
// risking consensus issues like equivocation (missing votes, double-signing).
type tendermintDB[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	db       db.KeyValueStore
	batch    db.Batch
	walCount map[types.Height]walMsgCount
}

// NewTendermintDB creates a new TMDB instance implementing the TMDBInterface.
func NewTendermintDB[V types.Hashable[H], H types.Hash, A types.Addr](db db.KeyValueStore, h types.Height) TendermintDB[V, H, A] {
	tmdb := tendermintDB[V, H, A]{db: db, batch: db.NewBatch()}

	walCount := make(map[types.Height]walMsgCount)
	walCount[h] = tmdb.getWALCount(h)
	tmdb.walCount = walCount
	return &tmdb
}

// CommitBatch implements TMDBInterface.
func (s *tendermintDB[V, H, A]) CommitBatch() error {
	if err := s.batch.Write(); err != nil {
		return err
	}
	// A batch must not be used after it has been committed. Reusing a batch after commit will result in a panic.
	s.batch = s.db.NewBatch()
	return nil
}

// getWALCount scans the DB for the number of WAL messages at a given height.
// It panics if the DB scan fails.
func (s *tendermintDB[V, H, A]) getWALCount(height types.Height) walMsgCount {
	prefix := WALEntry.Key(encodeHeight(height))
	count := walMsgCount(0)
	err := s.db.View(func(snap db.Snapshot) error {
		defer snap.Close()
		iter, err := snap.NewIterator(prefix, true)
		if err != nil {
			return err
		}

		defer iter.Close()
		for iter.First(); iter.Valid(); iter.Next() {
			count++
		}
		return nil
	})
	if err != nil {
		// Failing to retrieve the WAL msgs can result in a loss of funds (consensus slashing)
		panic(fmt.Sprintf("getWALCount: failed to scan WAL entries for height %d: %v", height, err))
	}
	return count
}

// DeleteWALMsgs iterates through the expected message keys based on the stored count.
// Note: This operates on the batch. Changes are only persisted after CommitBatch() is called.
func (s *tendermintDB[V, H, A]) DeleteWALMsgs(height types.Height) error {
	heightBytes := encodeHeight(height)
	startIterBytes := encodeNumMsgsAtHeight(walMsgCount(1))

	startKey := WALEntry.Key(heightBytes, startIterBytes)
	endKey := WALEntry.Key(encodeHeight(height + 1))
	if err := s.batch.DeleteRange(startKey, endKey); err != nil {
		return fmt.Errorf("DeleteWALMsgs: failed to add delete range [%x, %x) to batch: %w", startKey, endKey, err)
	}

	delete(s.walCount, height)
	return nil
}

// SetWALEntry implements TMDBInterface.
func (s *tendermintDB[V, H, A]) SetWALEntry(entry types.Message[V, H, A]) error {
	wrapper := walEntry[V, H, A]{
		Type:  entry.MsgType(),
		Entry: entry,
	}
	wrappedEntry, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("SetWALEntry: marshal wrapper failed: %w", err)
	}
	height := entry.GetHeight()
	numMsgsAtHeight, ok := s.walCount[height]
	if !ok {
		numMsgsAtHeight = 0
	}
	nextNumMsgsAtHeight := numMsgsAtHeight + 1

	msgKey := WALEntry.Key(encodeHeight(height), encodeNumMsgsAtHeight(nextNumMsgsAtHeight))
	if err := s.batch.Put(msgKey, wrappedEntry); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}

	s.walCount[height] = nextNumMsgsAtHeight
	return nil
}

// GetWALMsgs implements TMDBInterface.
func (s *tendermintDB[V, H, A]) GetWALMsgs(height types.Height) ([]walEntry[V, H, A], error) {
	numEntries := s.walCount[height]
	walMsgs := make([]walEntry[V, H, A], numEntries)
	if numEntries == 0 {
		return walMsgs, nil
	}
	startKey := WALEntry.Key(encodeHeight(height))
	err := s.db.View(func(snap db.Snapshot) error {
		defer snap.Close()
		iter, err := snap.NewIterator(startKey, true)
		if err != nil {
			return err
		}
		defer iter.Close()

		msgID := 0
		for iter.First(); iter.Valid(); iter.Next() {
			v, err := iter.Value()
			if err != nil {
				return fmt.Errorf("scanWALRaw: failed to get value: %w", err)
			}
			if err := cbor.Unmarshal(v, &walMsgs[msgID]); err != nil {
				return err
			}
			msgID++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanWALRaw: db view error: %w", err)
	}
	return walMsgs, nil
}

func encodeHeight(height types.Height) []byte {
	heightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(heightBytes, uint32(height))
	return heightBytes
}

func encodeNumMsgsAtHeight(numMsgsAtHeight walMsgCount) []byte {
	numMsgsAtHeightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(numMsgsAtHeightBytes, uint32(numMsgsAtHeight))
	return numMsgsAtHeightBytes
}
