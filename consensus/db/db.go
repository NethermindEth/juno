package db

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/dbutils"
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
type TendermintDB[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Flush writes the accumulated batch operations to the underlying database.
	Flush() error
	// GetWALEntries retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	LoadAllEntries() iter.Seq2[wal.Entry[V, H, A], error]
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
func NewTendermintDB[V types.Hashable[H], H types.Hash, A types.Addr](db db.KeyValueStore) TendermintDB[V, H, A] {
	return &tendermintDB[V, H, A]{
		db:       db,
		batch:    db.NewBatch(),
		walCount: make(map[types.Height]walMsgCount),
	}
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

// DeleteWALEntries iterates through the expected message keys based on the stored count.
// Note: This operates on the batch. Changes are only persisted after Flush() is called.
func (s *tendermintDB[V, H, A]) DeleteWALEntries(height types.Height) error {
	startKey := WALEntryBucket.Key(encodeHeight(height))
	endKey := dbutils.UpperBound(startKey)

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

	msgKey := s.nextKey(entry.GetHeight())
	if err := s.batch.Put(msgKey, marshaledEntry); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}

	return nil
}

// LoadAllEntries implements TMDBInterface.
func (s *tendermintDB[V, H, A]) LoadAllEntries() iter.Seq2[wal.Entry[V, H, A], error] {
	return func(yield func(wal.Entry[V, H, A], error) bool) {
		err := s.db.View(func(snap db.Snapshot) error {
			defer snap.Close()

			iter, err := snap.NewIterator(WALEntryBucket.Key(), true)
			if err != nil {
				return fmt.Errorf("failed to create iter: %w", err)
			}
			defer iter.Close()

			for iter.First(); iter.Valid(); iter.Next() {
				v, err := iter.Value()
				if err != nil {
					return fmt.Errorf("failed to get iter value: %w", err)
				}

				var walEntry wal.Entry[V, H, A]
				if err := encoder.Unmarshal(v, &walEntry); err != nil {
					return fmt.Errorf("failed to unmarshal walEntry: %w", err)
				}

				expectedKey := s.nextKey(walEntry.GetHeight())
				if !bytes.Equal(iter.Key(), expectedKey) {
					return fmt.Errorf("unexpected key %x, expected %x", iter.Key(), expectedKey)
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

func (s *tendermintDB[V, H, A]) nextKey(height types.Height) []byte {
	nextIndex := s.walCount[height]
	s.walCount[height] = nextIndex + 1
	return WALEntryBucket.Key(encodeHeight(height), encodeNumMsgsAtHeight(nextIndex))
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
