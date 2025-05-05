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
type WALMsg[V Hashable[H], H Hash, A Addr, M Message[V, H, A]] struct {
	Msg     *M
	Timeout *timeout
}

type TMDB struct { // Todo: move this elsewhere. Currently just a placeholder
	db    db.KeyValueStore
	batch db.Batch // Tendermint never needs to read wal messages from the batch
}

func NewTMState(db db.KeyValueStore) TMDB {
	return TMDB{db: db, batch: db.NewBatch()}
}

func (s *TMDB) CommitBatch() error { // Todo: figure out when to call this to balance safety and performance
	return s.batch.Write()
}

// GetNumMsgsAtHeight can onl read from the db. This is because we use batch transactions
// which don't allow reading from the batch. This is okay because we will only call this function
// once we (re)start the node, to get the WAL msgs and replay them.
func (s *TMDB) GetNumMsgsAtHeight(height height) (uint32, error) {
	heightBytes := heightToBytes(height)
	key := db.NumMsgsAtHeight.Key(heightBytes)

	var value []byte
	err := s.db.Get(key, func(val []byte) error {
		value = append([]byte(nil), val...)
		return nil
	})
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return 0, db.ErrKeyNotFound
		}
		return 0, fmt.Errorf("GetNumMsgsAtHeight: db.View error: %w", err)
	}

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

func SetWAL[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *TMDB, msg M, to *timeout, height height) error {
	var (
		dataType MessageType
		data     []byte
		err      error
	)

	if to != nil {
		data, err = cbor.Marshal(to)
		if err != nil {
			return fmt.Errorf("SetWAL: marshal inner timeout failed: %w", err)
		}
		dataType = MessageTypeTimeout
	} else {
		dataType, err = msgType[V, H, A](msg)
		if err != nil {
			return fmt.Errorf("SetWAL: failed to get msg type: %w", err)
		}
		data, err = cbor.Marshal(msg)
		if err != nil {
			return fmt.Errorf("SetWAL: marshal message failed: %w", err)
		}
	}

	// Wrap type + data
	wrapper := struct {
		Type string `cbor:"type"`
		Data []byte `cbor:"data"`
	}{
		Type: dataType.String(),
		Data: data,
	}

	msgData, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("SetWAL: marshal wrapper failed: %w", err)
	}

	numMsgsAtHeight, err := s.GetNumMsgsAtHeight(height)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			numMsgsAtHeight = 0 // first time storing a msg at this height, count starts at 0
		} else {
			return fmt.Errorf("SetWAL: failed to get numMsgsAtHeight: %w", err)
		}
	}
	// numMsgsAtHeight now holds the current count of messages (0 if none exist)

	// Use the *current* count as the iterator index for the new message key
	currentIterBytes := encodeNumMsgsAtHeight(numMsgsAtHeight)
	key := db.MsgsAtHeight.Key(heightToBytes(height), currentIterBytes)
	fmt.Println("MsgsAtHeight key", key) // Keep for debugging if needed
	err = s.batch.Put(key, msgData)
	if err != nil {
		return fmt.Errorf("SetWAL: failed to set MsgsAtHeight with key %q: %w", key, err)
	}

	// Now, increment the count for the *next* message
	numMsgsAtHeight++
	// Update the counter in the DB batch with the new count
	err = s.setNumMsgsAtHeight(height, numMsgsAtHeight)
	if err != nil {
		return fmt.Errorf("SetWAL: failed to set NumMsgsAtHeight to %d: %w", numMsgsAtHeight, err)
	}
	return nil
}

func GetWALMsgs[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *TMDB, height height) ([]WALMsg[V, H, A, M], error) {
	rawEntries, err := scanWALRaw(s, height)
	if err != nil {
		return nil, fmt.Errorf("GetWALMsgs: failed to scan raw entries: %w", err)
	}
	if len(rawEntries) == 0 {
		return nil, nil
	}

	walMsgs := make([]WALMsg[V, H, A, M], 0, len(rawEntries))

	for i, value := range rawEntries {
		var wrapper struct {
			Type string          `cbor:"type"`
			Data cbor.RawMessage `cbor:"data"`
		}
		if err := cbor.Unmarshal(value, &wrapper); err != nil {
			var simplerWrapper struct {
				Type string `cbor:"type"`
			}
			if errSimple := cbor.Unmarshal(value, &simplerWrapper); errSimple == nil {
				return nil, fmt.Errorf("GetWALMsgs: entry %d: failed to decode wrapper data for type %q: %w", i, simplerWrapper.Type, err)
			}
			return nil, fmt.Errorf("GetWALMsgs: entry %d: failed to decode wrapper: %w", i, err)
		}

		switch wrapper.Type {
		case MessageTypeTimeout.String():
			var to timeout
			if err := cbor.Unmarshal(wrapper.Data, &to); err != nil {
				return nil, fmt.Errorf("GetWALMsgs: entry %d: failed to unmarshal timeout data: %w", i, err)
			}
			walMsgs = append(walMsgs, WALMsg[V, H, A, M]{Timeout: &to})

		case MessageTypeProposal.String():
			var proposal Proposal[V, H, A]
			if err := cbor.Unmarshal(wrapper.Data, &proposal); err != nil {
				return nil, fmt.Errorf("GetWALMsgs: entry %d: Proposal unmarshal failed: %w", i, err)
			}
			msg := any(proposal).(M)
			walMsgs = append(walMsgs, WALMsg[V, H, A, M]{Msg: &msg})

		case MessageTypePrevote.String(), MessageTypePrecommit.String():
			var vote Vote[H, A]
			if err := cbor.Unmarshal(wrapper.Data, &vote); err != nil {
				return nil, fmt.Errorf("GetWALMsgs: entry %d: Vote unmarshal failed for type %q: %w", i, wrapper.Type, err)
			}
			msg := any(vote).(M)
			walMsgs = append(walMsgs, WALMsg[V, H, A, M]{Msg: &msg})

		default:
			return nil, fmt.Errorf("GetWALMsgs: entry %d: encountered unknown message type string %q", i, wrapper.Type)
		}
	}
	return walMsgs, nil
}

func scanWALRaw(s *TMDB, height height) ([][]byte, error) {
	prefix := db.MsgsAtHeight.Key(heightToBytes(height))
	rawEntries := [][]byte{}

	err := s.db.View(func(snap db.Snapshot) error {
		iter, err := snap.NewIterator(prefix, false)
		if err != nil {
			return fmt.Errorf("failed to create iterator with prefix %q: %w", prefix, err)
		}
		defer iter.Close()

		if !iter.Seek(prefix) {
			if !iter.Valid() {
				return nil
			}
			if !bytes.HasPrefix(iter.Key(), prefix) {
				return nil
			}
		}

		for ; iter.Valid() && bytes.HasPrefix(iter.Key(), prefix); iter.Next() {
			v, err := iter.Value()
			if err != nil {
				return fmt.Errorf("failed to get value for key %q: %w", iter.Key(), err)
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
func msgType[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](msg M) (MessageType, error) {
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
