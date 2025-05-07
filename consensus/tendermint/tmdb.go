package tendermint

import (
	"bytes"
	"encoding/binary"
	"fmt"

	tmdb "github.com/NethermindEth/juno/consensus/db"
	db "github.com/NethermindEth/juno/db"
	"github.com/fxamacker/cbor/v2"
)

// Example DB layout for storing Tendermint WAL messages per height:
//
// We use two primary key prefixes based on db.BucketConsensus:
//
// 1. db.WALEntryCount: Stores the total number of messages (proposals, votes, timeouts)
//    for a given height.
//    - Key: db.WALEntryCount + <heightBytes>
//    - Value: <countBytes>
//    - Acts as a 1-based index for the individual message keys.
//
// 2. db.WALEntry: Stores the actual CBOR-encoded wrapped messages or timeouts.
//    - Key: db.WALEntry + <heightBytes> + <iterBytes>
//    - Value: CBOR-encoded wrapped Msg/Timeout
//    - <iterBytes> ranges from 1 to the count stored under db.WALEntryCount.
//
// Both <heightBytes> and <iterBytes> are 4-byte big-endian uint32 representations.
//
// Batching:
// We use a Batch (db.IndexedBatch) to accumulate writes before committing them to the DB.
// This reduces expensive disk I/O. Reads check the batch first.
// WARNING: If the process crashes before Commit(), buffered messages are lost,
// risking consensus issues like equivocation (missing votes, double-signing).

const NumBytesForHeight = 4

type walIter uint32

type walEntry struct {
	Type  MessageType `cbor:"type"`
	Entry IsWALMsg    `cbor:"data"` // cbor serialised Msg or Timeout
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
type TendermintDB[V Hashable[H], H Hash, A Addr] interface {
	// CommitBatch writes the accumulated batch operations to the underlying database.
	CommitBatch() error
	// GetWALMsgs retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	GetWALMsgs(height height) ([]walEntry, error)
	// SetWALEntry schedules the storage of a WAL message in the batch.
	SetWALEntry(entry IsWALMsg, height height) error
	// DeleteWALMsgs schedules the deletion of all WAL messages for a specific height in the batch.
	DeleteWALMsgs(height height) error
}

// tendermintDB provides database access for Tendermint consensus state.
type tendermintDB[V Hashable[H], H Hash, A Addr] struct {
	db       db.KeyValueStore
	batch    db.Batch
	walCount map[height]walIter
}

// NewTMDB creates a new TMDB instance implementing the TMDBInterface.
func NewTMDB[V Hashable[H], H Hash, A Addr](db db.KeyValueStore, h height) TendermintDB[V, H, A] {
	tmdb := tendermintDB[V, H, A]{db: db, batch: db.NewBatch()}

	walCount := make(map[height]walIter)
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
func (s *tendermintDB[V, H, A]) getWALCount(height height) walIter {
	prefix := tmdb.WALEntry.Key(heightToBytes(height))
	count := walIter(0)

	err := s.db.View(func(snap db.Snapshot) error {
		iter, err := snap.NewIterator(prefix, false)
		if err != nil {
			return err
		}
		defer iter.Close()

		if !iter.Seek(prefix) {
			return nil // No entries for this height
		}
		for ; iter.Valid(); iter.Next() {
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
// Note: This operates on the batch. Changes are only persisted after CommitBatch() is called.
func (s *tendermintDB[V, H, A]) DeleteWALMsgs(height height) error {
	numMsgs, ok := s.walCount[height]
	if !ok {
		return nil
	}

	heightBytes := heightToBytes(height)

	startIterBytes := encodeNumMsgsAtHeight(walIter(1))
	endIterBytes := encodeNumMsgsAtHeight(walIter(uint32(numMsgs + 1)))

	startKey := tmdb.WALEntry.Key(heightBytes, startIterBytes)
	endKey := tmdb.WALEntry.Key(heightBytes, endIterBytes)
	if err := s.batch.DeleteRange(startKey, endKey); err != nil {
		return fmt.Errorf("DeleteWALMsgs: failed to add delete range [%x, %x) to batch: %w", startKey, endKey, err)
	}

	delete(s.walCount, height)
	return nil
}

// SetWALEntry implements TMDBInterface.
func (s *tendermintDB[V, H, A]) SetWALEntry(entry IsWALMsg, height height) error {
	wrapper := walEntry{
		Type:  entry.msgType(),
		Entry: entry,
	}
	wrappedEntry, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("SetWALEntry: marshal wrapper failed: %w", err)
	}

	numMsgsAtHeight, ok := s.walCount[height]
	if !ok {
		numMsgsAtHeight = 0
	}
	nextNumMsgsAtHeight := numMsgsAtHeight + 1
	nextNumMsgsAtHeightBytes := encodeNumMsgsAtHeight(nextNumMsgsAtHeight)

	msgKey := tmdb.WALEntry.Key(heightToBytes(height), nextNumMsgsAtHeightBytes)
	if err := s.batch.Put(msgKey, wrappedEntry); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}

	s.walCount[height] = nextNumMsgsAtHeight
	return nil
}

// GetWALMsgs implements TMDBInterface.
func (s *tendermintDB[V, H, A]) GetWALMsgs(height height) ([]walEntry, error) {
	rawEntries, err := s.scanWALRaw(height)
	if err != nil {
		return nil, fmt.Errorf("GetWALMsgs: failed to scan raw WAL entries for height %d: %w", height, err)
	}

	walMsgs := make([]walEntry, len(rawEntries))

	for i, rawEntry := range rawEntries {
		var wrapperType struct {
			Type    MessageType     `cbor:"type"`
			RawData cbor.RawMessage `cbor:"data"` // Golang can't unmarshal to an interface
		}
		if err := cbor.Unmarshal(rawEntry, &wrapperType); err != nil {
			return nil, fmt.Errorf("GetWALMsgs: failed to decode CBOR wrapper for entry %d at height %d: %w", i, height, err)
		}
		switch wrapperType.Type {
		case MessageTypeTimeout:
			var to timeout
			if err := cbor.Unmarshal(wrapperType.RawData, &to); err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to unmarshal timeout data for entry %d at height %d: %w", i, height, err)
			}
			walMsgs[i] = walEntry{Type: MessageTypeTimeout, Entry: to}
		case MessageTypeProposal, MessageTypePrevote, MessageTypePrecommit:
			walEntry, err := decodeWALMessageData[V, H, A](wrapperType.Type, wrapperType.RawData)
			if err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to decode message type %q for entry %d at height %d: %w", wrapperType.Type, i, height, err)
			}
			walMsgs[i] = walEntry
		default:
			return nil, fmt.Errorf("GetWALMsgs: failed to decode message type %q for entry %d at height %d: %w", wrapperType.Type, i, height, err)
		}
	}
	return walMsgs, nil
}

// scanWALRaw iterates over raw WAL entries in the database for a given height.
func (s *tendermintDB[V, H, A]) scanWALRaw(height height) ([][]byte, error) {
	startKey := tmdb.WALEntry.Key(encodeNumMsgsAtHeight(walIter(height)))
	rawEntries := [][]byte{}

	err := s.db.View(func(snap db.Snapshot) error {
		iter, err := snap.NewIterator(startKey, false)
		if err != nil {
			return err
		}
		defer iter.Close()

		if !iter.Seek(startKey) {
			// Changed error message slightly for clarity
			return fmt.Errorf("failed to seek to start key when scanning WAL msgs")
		}
		if !iter.Valid() {
			// It's possible there are no entries, return empty slice and no error
			return nil
		}

		for ; iter.Valid(); iter.Next() {
			// Check if the key still has the correct prefix (height)
			key := iter.Key()
			if !bytes.HasPrefix(key, startKey) {
				break // Stop if we moved past keys for this height
			}
			v, err := iter.Value()
			if err != nil {
				return fmt.Errorf("scanWALRaw: failed to get value: %w", err)
			}
			valueCopy := make([]byte, len(v))
			copy(valueCopy, v)
			rawEntries = append(rawEntries, valueCopy)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanWALRaw: db view error: %w", err)
	}
	return rawEntries, nil
}

// decodeWALMessageData decodes the inner CBOR message data based on the provided type string.
// It assumes the outer wrapper has already been decoded.
func decodeWALMessageData[V Hashable[H], H Hash, A Addr](msgType MessageType, data cbor.RawMessage) (walEntry, error) {
	switch msgType {
	case MessageTypeProposal:
		var proposal Proposal[V, H, A]
		if err := cbor.Unmarshal(data, &proposal); err != nil {
			return walEntry{}, fmt.Errorf("decodeWALMessageData: Proposal unmarshal failed: %w", err)
		}
		return walEntry{Type: MessageTypeProposal, Entry: proposal}, nil
	case MessageTypePrevote:
		var vote Prevote[H, A]
		if err := cbor.Unmarshal(data, &vote); err != nil {
			return walEntry{}, fmt.Errorf("decodeWALMessageData: Prevote unmarshal failed: %w", err)
		}
		return walEntry{Type: MessageTypePrevote, Entry: vote}, nil
	case MessageTypePrecommit:
		var vote Precommit[H, A]
		if err := cbor.Unmarshal(data, &vote); err != nil {
			return walEntry{}, fmt.Errorf("decodeWALMessageData: Precommit unmarshal failed: %w", err)
		}
		return walEntry{Type: MessageTypePrecommit, Entry: vote}, nil
	default:
		return walEntry{}, fmt.Errorf("decodeWALMessageData: unknown message type string %q", msgType.String())
	}
}

func heightToBytes(height height) []byte {
	heightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(heightBytes, uint32(height))
	return heightBytes
}

func encodeNumMsgsAtHeight(numMsgsAtHeight walIter) []byte {
	numMsgsAtHeightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(numMsgsAtHeightBytes, uint32(numMsgsAtHeight))
	return numMsgsAtHeightBytes
}
