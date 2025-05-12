package tendermint

import (
	"bytes"
	"encoding/binary"
	"fmt"

	tmdb "github.com/NethermindEth/juno/consensus/db"
	db "github.com/NethermindEth/juno/db"
	"github.com/fxamacker/cbor/v2"
)

const NumBytesForHeight = 4

// walMsgCount tracks the number of wal entries at the current height
type walMsgCount uint32

type WalEntry[V Hashable[H], H Hash, A Addr] struct {
	Type  MessageType `cbor:"type"`
	Entry IsWALMsg    `cbor:"data"` // cbor serialised Msg or Timeout
}

func (w *WalEntry[V, H, A]) UnmarshalCBOR(data []byte) error {
	var wrapperType struct {
		Type    MessageType     `cbor:"type"`
		RawData cbor.RawMessage `cbor:"data"` // Golang can't unmarshal to an interface
	}
	if err := cbor.Unmarshal(data, &wrapperType); err != nil {
		return err
	}
	w.Type = wrapperType.Type
	switch wrapperType.Type {
	case MessageTypeTimeout:
		var to Timeout
		if err := cbor.Unmarshal(wrapperType.RawData, &to); err != nil {
			return err
		}
		w.Entry = to
	case MessageTypeProposal:
		var proposal Proposal[V, H, A]
		if err := cbor.Unmarshal(wrapperType.RawData, &proposal); err != nil {
			return err
		}
		w.Entry = proposal
	case MessageTypePrevote:
		var vote Prevote[H, A]
		if err := cbor.Unmarshal(wrapperType.RawData, &vote); err != nil {
			return err
		}
		w.Entry = vote
	case MessageTypePrecommit:
		var vote Precommit[H, A]
		if err := cbor.Unmarshal(wrapperType.RawData, &vote); err != nil {
			return err
		}
		w.Entry = vote
	default:
		return fmt.Errorf("failed to unmarshal walEntry, unknown type")
	}
	return nil
}

// MessageType represents the type of message stored in the WAL.
type MessageType uint8

const (
	MessageTypeProposal MessageType = iota
	MessageTypePrevote
	MessageTypePrecommit
	MessageTypeTimeout
	MessageTypeUnknown
)

// String returns the string representation of the MessageType.
func (m MessageType) String() string {
	switch m {
	case MessageTypeProposal:
		return "proposal"
	case MessageTypePrevote:
		return "prevote"
	case MessageTypePrecommit:
		return "precommit"
	case MessageTypeTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// TendermintDB defines the methods for interacting with the Tendermint WAL database.
//
//go:generate mockgen -destination=../mocks/mock_db.go -package=mocks github.com/NethermindEth/juno/consensus/tendermint TendermintDB
type TendermintDB[V Hashable[H], H Hash, A Addr] interface {
	// FlushWAL writes the accumulated batch operations to the underlying database.
	FlushWAL() error
	// GetWALMsgs retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	GetWALMsgs(height Height) ([]WalEntry[V, H, A], error)
	// SetWALEntry schedules the storage of a WAL message in the batch.
	SetWALEntry(entry IsWALMsg) error
	// DeleteWALMsgs schedules the deletion of all WAL messages for a specific height in the batch.
	DeleteWALMsgs(height Height) error
}

// tendermintDB provides database access for Tendermint consensus state.
// We use a Batch to accumulate writes before committing them to the DB.
// This reduces expensive disk I/O. Reads check the batch first.
// WARNING: If the process crashes before Commit(), buffered messages are lost,
// risking consensus issues like equivocation (missing votes, double-signing).
type tendermintDB[V Hashable[H], H Hash, A Addr] struct {
	db       db.KeyValueStore
	batch    db.Batch
	walCount map[Height]walMsgCount
}

// NewTendermintDB creates a new TMDB instance implementing the TMDBInterface.
func NewTendermintDB[V Hashable[H], H Hash, A Addr](db db.KeyValueStore, h Height) TendermintDB[V, H, A] {
	tmdb := tendermintDB[V, H, A]{db: db, batch: db.NewBatch()}

	walCount := make(map[Height]walMsgCount)
	walCount[h] = tmdb.getWALCount(h)
	tmdb.walCount = walCount
	return &tmdb
}

// FlushWAL implements TMDBInterface.
func (s *tendermintDB[V, H, A]) FlushWAL() error {
	if err := s.batch.Write(); err != nil {
		return err
	}
	// A batch must not be used after it has been committed. Reusing a batch after commit will result in a panic.
	s.batch = s.db.NewBatch()
	return nil
}

// getWALCount scans the DB for the number of WAL messages at a given height.
// It panics if the DB scan fails.
func (s *tendermintDB[V, H, A]) getWALCount(height Height) walMsgCount {
	prefix := tmdb.WALEntry.Key(encodeHeight(height))
	count := walMsgCount(0)
	err := s.db.View(func(snap db.Snapshot) error {
		defer snap.Close()
		iter, err := snap.NewIterator(prefix, true)
		if err != nil {
			return err
		}

		defer iter.Close()
		for iter.Seek(prefix); iter.Valid(); iter.Next() {
			key := iter.Key()
			if !bytes.HasPrefix(key, prefix) {
				break
			}
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
// Note: This operates on the batch. Changes are only persisted after FlushWAL() is called.
func (s *tendermintDB[V, H, A]) DeleteWALMsgs(height Height) error {
	heightBytes := encodeHeight(height)
	startIterBytes := encodeNumMsgsAtHeight(walMsgCount(1))

	startKey := tmdb.WALEntry.Key(heightBytes, startIterBytes)
	endKey := tmdb.WALEntry.Key(encodeHeight(height + 1))
	if err := s.batch.DeleteRange(startKey, endKey); err != nil {
		return fmt.Errorf("DeleteWALMsgs: failed to add delete range [%x, %x) to batch: %w", startKey, endKey, err)
	}

	delete(s.walCount, height)
	return nil
}

// SetWALEntry implements TMDBInterface.
func (s *tendermintDB[V, H, A]) SetWALEntry(entry IsWALMsg) error {
	wrapper := WalEntry[V, H, A]{
		Type:  entry.msgType(),
		Entry: entry,
	}
	wrappedEntry, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("SetWALEntry: marshal wrapper failed: %w", err)
	}
	height := entry.height()
	numMsgsAtHeight, ok := s.walCount[height]
	if !ok {
		numMsgsAtHeight = 0
	}
	nextNumMsgsAtHeight := numMsgsAtHeight + 1

	msgKey := tmdb.WALEntry.Key(encodeHeight(height), encodeNumMsgsAtHeight(nextNumMsgsAtHeight))
	if err := s.batch.Put(msgKey, wrappedEntry); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}

	s.walCount[height] = nextNumMsgsAtHeight
	return nil
}

// GetWALMsgs implements TMDBInterface.
func (s *tendermintDB[V, H, A]) GetWALMsgs(height Height) ([]WalEntry[V, H, A], error) {
	numEntries := s.walCount[height]
	walMsgs := make([]WalEntry[V, H, A], numEntries)
	if numEntries == 0 {
		return walMsgs, nil
	}
	startKey := tmdb.WALEntry.Key(encodeHeight(height))
	err := s.db.View(func(snap db.Snapshot) error {
		defer snap.Close()
		iter, err := snap.NewIterator(startKey, true)
		if err != nil {
			return err
		}
		defer iter.Close()

		if !iter.Seek(startKey) {
			// Changed error message slightly for clarity
			return fmt.Errorf("failed to seek to start key when scanning WAL msgs")
		}
		msgID := 0
		for ; iter.Valid(); iter.Next() {
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

func encodeHeight(height Height) []byte {
	heightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(heightBytes, uint32(height))
	return heightBytes
}

func encodeNumMsgsAtHeight(numMsgsAtHeight walMsgCount) []byte {
	numMsgsAtHeightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(numMsgsAtHeightBytes, uint32(numMsgsAtHeight))
	return numMsgsAtHeightBytes
}
