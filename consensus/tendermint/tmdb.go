package tendermint

import (
	"bytes"
	"encoding/binary"
	"fmt"

	db "github.com/NethermindEth/juno/db"
	tmdb "github.com/NethermindEth/juno/db/consensus"
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
	// GetWALMsgs retrieves all WAL messages (consensus messages and timeouts) stored for a given height from the database.
	GetWALMsgs(height height) ([]IsWALMsg, error)
	// SetWALEntry schedules the storage of a WAL message in the batch.
	SetWALEntry(entry IsWALMsg, height height) error
	// DeleteWALMsgs schedules the deletion of all WAL messages for a specific height in the batch.
	DeleteWALMsgs(height height) error
}

// TMDB provides database access for Tendermint consensus state.
type TMDB[V Hashable[H], H Hash, A Addr] struct {
	db       db.KeyValueStore
	batch    db.Batch
	walCount map[height]walIter
}

// NewTMDB creates a new TMDB instance implementing the TMDBInterface.
func NewTMDB[V Hashable[H], H Hash, A Addr](db db.KeyValueStore, h height) TMDBInterface[V, H, A] {
	tmdb := TMDB[V, H, A]{db: db, batch: db.NewBatch()}

	walCount := make(map[height]walIter)
	walCount[h] = tmdb.getWALCount(h)
	tmdb.walCount = walCount

	return &tmdb
}

// CommitBatch implements TMDBInterface.
func (s *TMDB[V, H, A]) CommitBatch() error {
	return s.batch.Write()
}

// getWALCount scans the DB for the number of WAL messages at a given height.
// It panics if the DB scan fails.
func (s *TMDB[V, H, A]) getWALCount(height height) walIter {
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
		// Failing to retrieve the WAL msgs can result in slashing
		panic(fmt.Sprintf("getWALCount: failed to scan WAL entries for height %d: %v", height, err))
	}
	return count
}

// DeleteWALMsgs iterates through the expected message keys based on the stored count.
// Note: This operates on the batch. Changes are only persisted after CommitBatch() is called.
func (s *TMDB[V, H, A]) DeleteWALMsgs(height height) error {
	numMsgs, ok := s.walCount[height]
	if !ok {
		return nil
	}

	heightBytes := heightToBytes(height)

	// Delete individual message keys. Message iterators are 1-based.
	for i := uint32(1); i <= uint32(numMsgs); i++ {
		iterBytes := encodeNumMsgsAtHeight(walIter(i)) // Get the bytes for the iterator number
		msgKey := tmdb.WALEntry.Key(heightBytes, iterBytes)
		if err := s.batch.Delete(msgKey); err != nil {
			return fmt.Errorf("DeleteWALMsgs: failed to add deletion for message key %d/%d to batch: %w", height, i, err)
		}
	}

	// Delete the count entry
	delete(s.walCount, height)

	return nil
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
	numMsgsAtHeight, ok := s.walCount[height]
	if !ok {
		numMsgsAtHeight = 0 // First time storing a message
	}
	nextNumMsgsAtHeight := numMsgsAtHeight + 1
	nextNumMsgsAtHeightBytes := encodeNumMsgsAtHeight(nextNumMsgsAtHeight)

	// Set the WAL msg/timeout, and the new iterator
	msgKey := tmdb.WALEntry.Key(heightToBytes(height), nextNumMsgsAtHeightBytes)
	if err := s.batch.Put(msgKey, msgData); err != nil {
		return fmt.Errorf("writeWALEntryToBatch: failed to set MsgsAtHeight: %w", err)
	}

	// Update count
	s.walCount[height] = nextNumMsgsAtHeight
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

func encodeNumMsgsAtHeight(numMsgsAtHeight walIter) []byte {
	numMsgsAtHeightBytes := make([]byte, NumBytesForHeight)
	binary.BigEndian.PutUint32(numMsgsAtHeightBytes, uint32(numMsgsAtHeight))
	return numMsgsAtHeightBytes
}
