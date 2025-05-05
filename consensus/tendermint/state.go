package tendermint

import (
	"encoding/binary"
	"errors"
	"fmt"

	db "github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
	"github.com/fxamacker/cbor/v2"
)

type wrappedMsg struct {
	Type string          `cbor:"type"`
	Data cbor.RawMessage `cbor:"data"`
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

// Todo: just placing the DB logic here. Will likely restructure

// Todo: remove this description?
// Example db layout for the NumMsgsAtHeight and Msgs format.
// Basically we store a sequence number (`NumMsgsAtHeight`) for the number of messages written to the db at a given height.
// The NumMsgsAtHeight is used to divide and locate the message keys efficiently.
//
// | Key                                  | Value                         | Meaning                                |
// |--------------------------------------|-------------------------------|----------------------------------------|
// | db/WalIter/000003E8                  | 0000002B                      | Last wal_iter=43 at height=1000        |
// | db/Msg/000003E8/00000000             | (CBOR-encoded Proposal)       | Proposal at height=1000, iter=0        |
// | db/Msg/000003E8/00000001             | (CBOR-encoded Prevote)        | Prevote at height=1000, iter=1         |
// | db/Msg/000003E8/00000002             | (CBOR-encoded Precommit)      | Precommit at height=1000, iter=2       |
// ...
// | db/Msg/000003E8/00000029             | (CBOR-encoded Timeout)        | Timeout event at height=1000, iter=41  |
// | db/Msg/000003E8/0000002A             | (CBOR-encoded Proposal)       | Proposal at height=1000, iter=42       |
// | db/WalIter/000003E9                  | 00000010                      | Last wal_iter=16 at height=1001        |
// | db/Msg/000003E9/00000000             | (CBOR-encoded Timeout)        | Timeout at height=1001, iter=0         |
// | db/Msg/000003E9/00000001             | (CBOR-encoded Proposal)       | Proposal at height=1001, iter=1        |
// ...

// We use a Batch to accumulate writes before committing them to the DB.
// This reduces expensive disk I/O by batching multiple writes together.
//
// However, because the Batch contents are only stored in memory until Commit() is called,
// if the process crashes before committing, the buffered messages are lost.
// This loss risks serious issues like equivocation (e.g., missing votes, double-signing, etc.).
//
// Therefore, it is critical to commit the Batch frequently and with Sync enabled (pebble.Sync)
// to ensure durability guarantees are met.

// Todo: write a set of getters and setters around this.
type WALMsg[V Hashable[H], H Hash, A Addr] struct {
	Msg     any
	Timeout *timeout
}

type TMDB struct { // Todo: move this elsewhere. Currently just a placeholder
	db    db.KeyValueStore
	batch db.IndexedBatch // Tendermint never needs to read wal messages from the batch
}

func NewTMState(db db.KeyValueStore) TMDB {
	return TMDB{db: db, batch: db.NewIndexedBatch()}
}

func (s *TMDB) CommitBatch() error { // Todo: figure out when to call this to balance safety and performance
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

// Todo:
func (s *TMDB) DeleteMsgsAtHeight(height height) error {
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
	msgType, err := getMsgType[V, H, A, M](msg)
	if err != nil {
		return fmt.Errorf("SetWALMsg: failed to get msg type: %w", err)
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
	fmt.Println("msgDataInner", msgDataInner, to)
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
		fmt.Println(" -- ", wrapper.Data)
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

// todo: push to msg method??
func getHeight[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](msg M) (height, error) {
	switch m := any(msg).(type) {
	case Proposal[V, H, A]:
		return m.Height, nil
	case Prevote[H, A]:
		return m.Height, nil
	case Precommit[H, A]:
		return m.Height, nil
	}
	return height(0), fmt.Errorf("failed to get message height")
}

// Todo: push to utils?
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

// msgType determines the MessageType enum for a given message.
func getMsgType[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](msg M) (MessageType, error) {
	switch any(msg).(type) {
	case Proposal[V, H, A]:
		return MessageTypeProposal, nil
	case Prevote[H, A]:
		return MessageTypePrevote, nil
	case Precommit[H, A]:
		return MessageTypePrecommit, nil
	}
	return MessageTypeUnknown, fmt.Errorf("msgType: unknown message type %T", msg)
}
