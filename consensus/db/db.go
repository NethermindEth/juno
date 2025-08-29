package db

import (
	"encoding/binary"
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

const NumBytesForHeight = 4

// walMsgCount tracks the number of wal entries at the current height
type walMsgCount uint32

// TendermintDB defines the methods for interacting with the Tendermint WAL database.
//
// The purpose of the WAL is to record any event that may result in a state change.
// These events fall into the following categories:
// 1. Incoming messages. We do not need to store outgoing messages.
// 2. When we propose a value.
// 3. When a timeout is triggered (not when it is scheduled).
// The purpose of the WAL is to allow the node to recover the state it was in before the crash.
// No new messages should be broadcast during replay.
//
// We commit the WAL to disk when:
// 1. We start a new round
// 2. Right before we broadcast a message
//
// We call Delete when we start a new height and commit a block
//
//go:generate mockgen -destination=../mocks/mock_db.go -package=mocks github.com/NethermindEth/juno/consensus/db TendermintDB
type TendermintDB[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Flush writes the accumulated batch operations to the underlying database.
	Flush() error
	// GetWALEntries retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	GetWALEntries(height types.Height) iter.Seq2[wal.Entry[V, H, A], error]
	// SetWALEntry schedules the storage of a WAL message in the batch.
	SetWALEntry(entry wal.Entry[V, H, A]) error
	// DeleteWALEntries schedules the deletion of all WAL messages for a specific height in the batch.
	DeleteWALEntries(height types.Height) error
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

// Flush implements TMDBInterface.
func (s *tendermintDB[V, H, A]) Flush() error {
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
	prefix := WALEntryBucket.Key(encodeHeight(height))
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

// DeleteWALEntries iterates through the expected message keys based on the stored count.
// Note: This operates on the batch. Changes are only persisted after Flush() is called.
func (s *tendermintDB[V, H, A]) DeleteWALEntries(height types.Height) error {
	heightBytes := encodeHeight(height)
	startIterBytes := encodeNumMsgsAtHeight(walMsgCount(1))

	startKey := WALEntryBucket.Key(heightBytes, startIterBytes)
	endKey := WALEntryBucket.Key(encodeHeight(height + 1))
	if err := s.batch.DeleteRange(startKey, endKey); err != nil {
		return fmt.Errorf("DeleteWALEntries: failed to add delete range [%x, %x) to batch: %w", startKey, endKey, err)
	}

	delete(s.walCount, height)
	return nil
}

// SetWALEntry implements TMDBInterface.
func (s *tendermintDB[V, H, A]) SetWALEntry(entry wal.Entry[V, H, A]) error {
	marshaledEntry, err := encoder.Marshal(entry)
	if err != nil {
		return fmt.Errorf("SetWALEntry: marshal entry failed: %w", err)
	}
	height := entry.GetHeight()
	numMsgsAtHeight, ok := s.walCount[height]
	if !ok {
		numMsgsAtHeight = 0
	}
	nextNumMsgsAtHeight := numMsgsAtHeight + 1

	msgKey := WALEntryBucket.Key(encodeHeight(height), encodeNumMsgsAtHeight(nextNumMsgsAtHeight))
	if err := s.batch.Put(msgKey, marshaledEntry); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}

	s.walCount[height] = nextNumMsgsAtHeight
	return nil
}

// GetWALEntries implements TMDBInterface.
func (s *tendermintDB[V, H, A]) GetWALEntries(height types.Height) iter.Seq2[wal.Entry[V, H, A], error] {
	return func(yield func(wal.Entry[V, H, A], error) bool) {
		if s.walCount[height] == 0 {
			return
		}

		err := s.db.View(func(snap db.Snapshot) error {
			defer snap.Close()

			startKey := WALEntryBucket.Key(encodeHeight(height))
			iter, err := snap.NewIterator(startKey, true)
			if err != nil {
				return fmt.Errorf("failed to create iter: %w", err)
			}
			defer iter.Close()

			for iter.First(); iter.Valid(); iter.Next() {
				var walEntry wal.Entry[V, H, A]
				v, err := iter.Value()
				if err != nil {
					return fmt.Errorf("failed to get iter value: %w", err)
				}

				if err := encoder.Unmarshal(v, &walEntry); err != nil {
					return fmt.Errorf("failed to unmarshal walEntry: %w", err)
				}

				if !yield(walEntry, nil) {
					return nil
				}
			}
			return nil
		})
		if err != nil {
			yield(nil, err)
		}
	}
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
