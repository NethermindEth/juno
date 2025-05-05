package tendermint

import (
	"encoding/binary"
	"errors"
	"fmt"

	db "github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
	"github.com/fxamacker/cbor/v2"
)

// Example DB layout for storing Tendermint WAL messages per height:
//
// We store a counter for the number of messages (including proposals, votes, and timeouts)
// written to the DB at a given height. This counter (`NumMsgsAtHeight`) acts as a 1-based index
// for the individual message keys.
//
// - Height is encoded as a 4-byte big-endian uint32.
// - The message index (iterator) is also encoded as a 4-byte big-endian uint32.
//
// | Key Prefix            | Key Suffixes                         | Value                          	| Meaning                                     |
// |-----------------------|--------------------------------------|---------------------------------|---------------------------------------------|
// | db.NumMsgsAtHeight    | <heightBytes>                        | <countBytes>                    | Stores the total count (e.g., 43) for height |
// | db.MsgsAtHeight       | <heightBytes>/<iterBytes=1>          | (CBOR-encoded wrapped Msg/T.O.) | First message at height                     |
// | db.MsgsAtHeight       | <heightBytes>/<iterBytes=2>          | (CBOR-encoded wrapped Msg/T.O.) | Second message at height                    |
// | ...                   | ...                                  | ...                             | ...                                         |
// | db.MsgsAtHeight       | <heightBytes>/<iterBytes=countBytes> | (CBOR-encoded wrapped Msg/T.O.) | Last message at height                      |
//
// We use a Batch to accumulate writes before committing them to the DB.
// This reduces expensive disk I/O by batching multiple writes together.
//
// However, because the Batch contents are only stored in memory until Commit() is called,
// if the process crashes before committing, the buffered messages are lost.
// This loss risks serious issues like equivocation (e.g., missing votes, double-signing, etc.).

type wrappedMsg struct {
	Type string          `cbor:"type"`
	Data cbor.RawMessage `cbor:"data"` // cbor serialised Msg or timeout
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

type WALMsg[V Hashable[H], H Hash, A Addr] struct {
	Msg     any
	Timeout *timeout
}

type TMDB struct { // Todo: move this elsewhere. Currently just a placeholder
	db    db.KeyValueStore
	batch db.IndexedBatch
}

func NewTMState(db db.KeyValueStore) TMDB {
	return TMDB{db: db, batch: db.NewIndexedBatch()}
}

func (s *TMDB) CommitBatch() error {
	return s.batch.Write()
}

// GetNumMsgsAtHeight reads the number of messages stored at a given height.
// It reads from the batch first, then the main db if not found in batch.
func (s *TMDB) GetNumMsgsAtHeight(height height) (uint32, error) {
	heightBytes := heightToBytes(height)
	key := db.NumMsgsAtHeight.Key(heightBytes)

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
		return 0, fmt.Errorf("GetNumMsgsAtHeight: batch.Get error: %w", err)
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
			return 0, fmt.Errorf("GetNumMsgsAtHeight: db.Get error: %w", err)
		}
	}

	// At this point, 'value' holds the data either from batch or db
	if len(value) != 4 {
		return 0, fmt.Errorf("GetNumMsgsAtHeight: unexpected value size %d", len(value))
	}

	return binary.BigEndian.Uint32(value), nil
}

// DeleteMsgsAtHeight removes all WAL messages and the message count associated with a specific height from the batch.
// It iterates through the expected message keys based on the stored count.
// Note: This operates on the batch. Changes are only persisted after CommitBatch() is called.
func (s *TMDB) DeleteMsgsAtHeight(height height) error {

	numMsgs, err := s.GetNumMsgsAtHeight(height)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return fmt.Errorf("DeleteMsgsAtHeight: failed to get message count for height %d: %w", height, err)
	}

	heightBytes := heightToBytes(height)

	// Delete individual message keys. Message iterators are 1-based.
	for i := uint32(1); i <= numMsgs; i++ {
		iterBytes := encodeNumMsgsAtHeight(i) // Get the bytes for the iterator number
		msgKey := db.MsgsAtHeight.Key(heightBytes, iterBytes)
		if err := s.batch.Delete(msgKey); err != nil {
			return fmt.Errorf("DeleteMsgsAtHeight: failed to add deletion for message key %d/%d to batch: %w", height, i, err)
		}
	}

	// Delete the count key itself
	numMsgsKey := db.NumMsgsAtHeight.Key(heightBytes)
	if err := s.batch.Delete(numMsgsKey); err != nil {
		// If adding the count key deletion fails, return the error.
		return fmt.Errorf("DeleteMsgsAtHeight: failed to add count key deletion for height %d to batch: %w", height, err)
	}

	// All delete operations successfully added to the batch.
	return nil
}

func (s *TMDB) setNumMsgsAtHeight(height height, walIter uint32) error {
	heightBytes := heightToBytes(height)
	key := db.NumMsgsAtHeight.Key(heightBytes)
	val := encodeNumMsgsAtHeight(walIter)
	return s.batch.Put(key, val)
}

// SetWALMsg stores a consensus message (Proposal, Prevote, Precommit) in the WAL batch.
func SetWALMsg[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *TMDB, msg M, height height) error {

	var msgType MessageType
	switch any(msg).(type) {
	case Proposal[V, H, A]:
		msgType = MessageTypeProposal
	case Prevote[H, A]:
		msgType = MessageTypePrevote
	case Precommit[H, A]:
		msgType = MessageTypePrecommit
	default:
		return fmt.Errorf("SetWALMsg: unknown message type %T", msg)
	}

	msgDataInner, err := cbor.Marshal(msg)
	if err != nil {
		return fmt.Errorf("SetWALMsg: marshal message failed: %w", err)
	}

	return setWALEntry(s, height, msgType, msgDataInner)
}

// SetWALTimeout stores a timeout event in the WAL batch.
func SetWALTimeout(s *TMDB, to *timeout, height height) error {
	msgType := MessageTypeTimeout

	msgDataInner, err := cbor.Marshal(to)
	if err != nil {
		return fmt.Errorf("SetWALTimeout: marshal timeout failed: %w", err)
	}
	return setWALEntry(s, height, msgType, msgDataInner)
}

// internal helper to set a WAL entry in the batch
func setWALEntry(s *TMDB, height height, msgType MessageType, innerData cbor.RawMessage) error {
	wrapper := wrappedMsg{
		Type: msgType.String(),
		Data: innerData,
	}

	msgData, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("setWALEntry: marshal wrapper failed: %w", err)
	}

	// Get NumMsgsAtHeight and increment
	numMsgsAtHeight, err := s.GetNumMsgsAtHeight(height)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			numMsgsAtHeight = 0 // first time storing a msg
		} else {
			return fmt.Errorf("setWALEntry: failed to get numMsgsAtHeight: %w", err)
		}
	}
	numMsgsAtHeight++
	numMsgsAtHeightBytes := encodeNumMsgsAtHeight(numMsgsAtHeight)

	// Set the WAL msg/timeout, and the new iterator
	msgKey := db.MsgsAtHeight.Key(heightToBytes(height), numMsgsAtHeightBytes)
	if err := s.batch.Put(msgKey, msgData); err != nil {
		return fmt.Errorf("setWALEntry: failed to set MsgsAtHeight: %w", err)
	}
	if err := s.setNumMsgsAtHeight(height, numMsgsAtHeight); err != nil {
		return fmt.Errorf("setWALEntry: failed to set NumMsgsAtHeight: %w", err)
	}
	return nil
}

func GetWALMsgs[V Hashable[H], H Hash, A Addr](s *TMDB, height height) ([]WALMsg[V, H, A], error) {
	rawEntries, err := scanWALRaw(s, height)
	if err != nil {
		return nil, fmt.Errorf("GetWALMsgs: failed to scan raw WAL entries for height %d: %w", height, err)
	}

	walMsgs := make([]WALMsg[V, H, A], 0, len(rawEntries))

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

			walMsgs = append(walMsgs, WALMsg[V, H, A]{Timeout: &to})

		default:
			msg, err := decodeWALMessageData[V, H, A](wrapper.Type, wrapper.Data)
			if err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to decode message type %q for entry %d at height %d: %w", wrapper.Type, i, height, err)
			}
			walMsgs = append(walMsgs, WALMsg[V, H, A]{Msg: msg})
		}
	}
	return walMsgs, nil
}

func scanWALRaw(s *TMDB, height height) ([][]byte, error) {
	startKey := db.MsgsAtHeight.Key(encodeNumMsgsAtHeight(uint32(height)))
	rawEntries := [][]byte{}

	err := s.db.View(func(snap db.Snapshot) error {
		iter, err := snap.NewIterator(startKey, false)
		if err != nil {
			return err
		}
		defer iter.Close()

		if !iter.Seek(startKey) {
			return fmt.Errorf("failed to set the iterator when scanning for WAL msgs")
		}
		if !iter.Valid() {
			return fmt.Errorf("invalid iterator when scanning for WAL msgs")
		}

		for ; iter.Valid(); iter.Next() {
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
	heightBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(heightBytes, uint32(height))
	return heightBytes
}

func encodeNumMsgsAtHeight(numMsgsAtHeight uint32) []byte {
	numMsgsAtHeightBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(numMsgsAtHeightBytes, numMsgsAtHeight)
	return numMsgsAtHeightBytes
}
