package tendermint

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	db "github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
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

type wrappedMsg struct {
	Type string          `cbor:"type"`
	Data cbor.RawMessage `cbor:"data"` // cbor serialised Msg or Timeout
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

// TMDBInterface defines the methods for interacting with the Tendermint WAL database.
type TMDBInterface[V Hashable[H], H Hash, A Addr] interface {
	// CommitBatch writes the accumulated batch operations to the underlying database.
	CommitBatch() error
	// GetWALCount retrieves the number of WAL messages stored for a given height.
	GetWALCount(height height) (uint32, error)
	// GetWALMsgs retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	GetWALMsgs(height height) ([]IsWALMsg, error)
	// SetWALEntry schedules the storage of a WAL message in the batch.
	SetWALEntry(entry IsWALMsg, height height) error
	// DeleteWALMsgs schedules the deletion of all WAL messages for a specific height in the batch.
	DeleteWALMsgs(height height) error
}

// TMDB provides database access for Tendermint consensus state.
type TMDB[V Hashable[H], H Hash, A Addr] struct {
	db    db.KeyValueStore
	batch db.IndexedBatch
}

// NewTMDB creates a new TMDB instance implementing the TMDBInterface.
func NewTMDB[V Hashable[H], H Hash, A Addr](db db.KeyValueStore) TMDBInterface[V, H, A] {
	return &TMDB[V, H, A]{db: db, batch: db.NewIndexedBatch()}
}

// CommitBatch implements TMDBInterface.
func (s *TMDB[V, H, A]) CommitBatch() error {
	return s.batch.Write()
}

// GetWALCount implements TMDBInterface.
// It reads from the batch first, then the main db if not found in batch.
func (s *TMDB[V, H, A]) GetWALCount(height height) (uint32, error) {
	heightBytes := heightToBytes(height)
	key := db.WALEntryCount.Key(heightBytes)

	var value []byte
	foundInBatch := false

	// Try reading from the batch first
	err := s.batch.Get(key, func(val []byte) error {
		value = append([]byte(nil), val...)
		foundInBatch = true
		return nil
	})

	// If not found in batch or another error occurred (excluding ErrNotFound)
	if err != nil && !errors.Is(err, pebble.ErrNotFound) {
		return 0, fmt.Errorf("GetWALCount: batch.Get error: %w", err)
	}

	// If not found in the batch, try reading from the main db
	if !foundInBatch {
		err = s.db.Get(key, func(val []byte) error {
			value = append([]byte(nil), val...)
			return nil
		})

		// If not found in db either, return ErrKeyNotFound
		if errors.Is(err, pebble.ErrNotFound) {
			return 0, db.ErrKeyNotFound
		}
		// Handle other potential errors from s.db.Get
		if err != nil {
			return 0, fmt.Errorf("GetWALCount: db.Get error: %w", err)
		}
	}

	// At this point, 'value' holds the data either from batch or db
	if len(value) != NumBytesForHeight {
		return 0, fmt.Errorf("GetWALCount: unexpected value size %d", len(value))
	}

	return binary.BigEndian.Uint32(value), nil
}

// DeleteWALMsgs iterates through the expected message keys based on the stored count.
// Note: This operates on the batch. Changes are only persisted after CommitBatch() is called.
func (s *TMDB[V, H, A]) DeleteWALMsgs(height height) error {
	numMsgs, err := s.GetWALCount(height) // Call the method receiver
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return fmt.Errorf("DeleteWALMsgs: failed to get message count for height %d: %w", height, err)
	}

	heightBytes := heightToBytes(height)

	// Delete individual message keys. Message iterators are 1-based.
	for i := uint32(1); i <= numMsgs; i++ {
		iterBytes := encodeNumMsgsAtHeight(i) // Get the bytes for the iterator number
		msgKey := db.WALEntry.Key(heightBytes, iterBytes)
		if err := s.batch.Delete(msgKey); err != nil {
			return fmt.Errorf("DeleteWALMsgs: failed to add deletion for message key %d/%d to batch: %w", height, i, err)
		}
	}

	// Delete the count key itself
	numMsgsKey := db.WALEntryCount.Key(heightBytes)
	if err := s.batch.Delete(numMsgsKey); err != nil {
		// If adding the count key deletion fails, return the error.
		return fmt.Errorf("DeleteWALMsgs: failed to add count key deletion for height %d to batch: %w", height, err)
	}

	// All delete operations successfully added to the batch.
	return nil
}

// setNumMsgsAtHeight stores the total message count for a height in the batch.
func (s *TMDB[V, H, A]) setNumMsgsAtHeight(height height, walIter uint32) error {
	heightBytes := heightToBytes(height)
	key := db.WALEntryCount.Key(heightBytes)
	val := encodeNumMsgsAtHeight(walIter)
	return s.batch.Put(key, val)
}

// SetWALEntry implements TMDBInterface.
func (s *TMDB[V, H, A]) SetWALEntry(entry IsWALMsg, height height) error {
	var msgType MessageType
	var msgDataInner []byte
	var err error

	switch m := entry.(type) {
	case Proposal[V, H, A]:
		msgType = MessageTypeProposal
		msgDataInner, err = cbor.Marshal(m)
		if err != nil {
			return fmt.Errorf("SetWALEntry: marshal Proposal failed: %w", err)
		}
	case Prevote[H, A]:
		msgType = MessageTypePrevote
		msgDataInner, err = cbor.Marshal(m)
		if err != nil {
			return fmt.Errorf("SetWALEntry: marshal Prevote failed: %w", err)
		}
	case Precommit[H, A]:
		msgType = MessageTypePrecommit
		msgDataInner, err = cbor.Marshal(m)
		if err != nil {
			return fmt.Errorf("SetWALEntry: marshal Precommit failed: %w", err)
		}
	case *timeout:
		// Handle pointer to timeout, as SetWALTimeout took *timeout
		msgType = MessageTypeTimeout
		msgDataInner, err = cbor.Marshal(m)
		if err != nil {
			return fmt.Errorf("SetWALEntry: marshal Timeout failed: %w", err)
		}
	case timeout:
		// Also handle timeout by value if necessary, though pointer is expected
		msgType = MessageTypeTimeout
		msgDataInner, err = cbor.Marshal(m)
		if err != nil {
			return fmt.Errorf("SetWALEntry: marshal Timeout failed: %w", err)
		}
	default:
		// This case should ideally not be reached if only valid WAL types are passed.
		return fmt.Errorf("SetWALEntry: unknown type implementing IsWALMsg: %T", entry)
	}

	// Call the renamed internal helper
	return s.writeWALEntryToBatch(height, msgType, msgDataInner)
}

// writeWALEntryToBatch is an internal helper to schedule a WAL entry write in the batch.
// Renamed from setWALEntry to avoid conflict with the public SetWALEntry method.
func (s *TMDB[V, H, A]) writeWALEntryToBatch(height height, msgType MessageType, innerData cbor.RawMessage) error {
	wrapper := wrappedMsg{
		Type: msgType.String(),
		Data: innerData,
	}

	msgData, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("writeWALEntryToBatch: marshal wrapper failed: %w", err)
	}

	// Get NumMsgsAtHeight and increment
	numMsgsAtHeight, err := s.GetWALCount(height)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			numMsgsAtHeight = 0 // first time storing a msg
		} else {
			return fmt.Errorf("writeWALEntryToBatch: failed to get numMsgsAtHeight: %w", err)
		}
	}
	nextNumMsgsAtHeight := numMsgsAtHeight + 1
	nextNumMsgsAtHeightBytes := encodeNumMsgsAtHeight(nextNumMsgsAtHeight)

	// Set the WAL msg/timeout, and the new iterator
	msgKey := db.WALEntry.Key(heightToBytes(height), nextNumMsgsAtHeightBytes)
	if err := s.batch.Put(msgKey, msgData); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}
	if err := s.setNumMsgsAtHeight(height, nextNumMsgsAtHeight); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set NumMsgsAtHeight: %w", err)
	}
	return nil
}

// GetWALMsgs implements TMDBInterface.
func (s *TMDB[V, H, A]) GetWALMsgs(height height) ([]IsWALMsg, error) {
	rawEntries, err := s.scanWALRaw(height)
	if err != nil {
		return nil, fmt.Errorf("GetWALMsgs: failed to scan raw WAL entries for height %d: %w", height, err)
	}

	walMsgs := make([]IsWALMsg, 0, len(rawEntries))

	for i, value := range rawEntries {
		var wrapper wrappedMsg
		if err := cbor.Unmarshal(value, &wrapper); err != nil {
			return nil, fmt.Errorf("GetWALMsgs: failed to decode CBOR wrapper for entry %d at height %d: %w", i, height, err)
		}
		switch wrapper.Type {
		case MessageTypeTimeout.String():
			var to timeout
			if err := cbor.Unmarshal(wrapper.Data, &to); err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to unmarshal timeout data for entry %d at height %d: %w", i, height, err)
			}
			// Append pointer to timeout directly
			walMsgs = append(walMsgs, &to)

		default:
			msg, err := decodeWALMessageData[V, H, A](wrapper.Type, wrapper.Data)
			if err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to decode message type %q for entry %d at height %d: %w", wrapper.Type, i, height, err)
			}

			walEntry, ok := msg.(IsWALMsg)
			if !ok {
				return nil, fmt.Errorf("GetWALMsgs: decoded message type %T does not implement IsWALMsg", msg)
			}
			// Append the message (which implements IsWALMsg) directly
			walMsgs = append(walMsgs, walEntry)
		}
	}
	return walMsgs, nil
}

// scanWALRaw iterates over raw WAL entries in the database for a given height.
func (s *TMDB[V, H, A]) scanWALRaw(height height) ([][]byte, error) {
	startKey := db.WALEntry.Key(encodeNumMsgsAtHeight(uint32(height)))
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
func decodeWALMessageData[V Hashable[H], H Hash, A Addr](msgTypeStr string, data cbor.RawMessage) (any, error) {
	switch msgTypeStr {
	case MessageTypeProposal.String():
		var proposal Proposal[V, H, A]
		if err := cbor.Unmarshal(data, &proposal); err != nil {
			return nil, fmt.Errorf("decodeWALMessageData: Proposal unmarshal failed: %w", err)
		}
		return proposal, nil

	case MessageTypePrevote.String():
		var vote Prevote[H, A]
		if err := cbor.Unmarshal(data, &vote); err != nil {
			return nil, fmt.Errorf("decodeWALMessageData: Prevote unmarshal failed: %w", err)
		}
		return vote, nil

	case MessageTypePrecommit.String():
		var vote Precommit[H, A]
		if err := cbor.Unmarshal(data, &vote); err != nil {
			return nil, fmt.Errorf("decodeWALMessageData: Precommit unmarshal failed: %w", err)
		}
		return vote, nil
	default:
		return nil, fmt.Errorf("decodeWALMessageData: unknown message type string %q", msgTypeStr)
	}
}

func heightToBytes(height height) []byte {
	heightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(heightBytes, uint32(height))
	return heightBytes
}

func encodeNumMsgsAtHeight(numMsgsAtHeight uint32) []byte {
	numMsgsAtHeightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(numMsgsAtHeightBytes, numMsgsAtHeight)
	return numMsgsAtHeightBytes
}
