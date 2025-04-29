package tendermint

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
	"github.com/fxamacker/cbor/v2"
)

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
	db    db.DB
	batch *pebble.Batch
}

func NewState(db db.DB) TMDB {
	return TMDB{db: db}
}

func (s *TMDB) CommitBatch() error { // Todo: figure out when to call this to balance safety and performance
	return s.batch.Commit(pebble.Sync)
}

func (s *TMDB) GetNumMsgsAtHeight(height height) (uint32, error) {
	heightBytes := heightToBytes(height)
	key := db.NumMsgsAtHeight.Key(heightBytes)

	// 1. Try batch first
	val, closer, err := s.batch.Get(key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			// 2. Fallback to DB using View
			var value []byte
			err = s.db.View(func(txn db.Transaction) error {
				return txn.Get(key, func(v []byte) error {
					value = append([]byte(nil), v...)
					return nil
				})
			})
			if err != nil {
				if errors.Is(err, pebble.ErrNotFound) {
					return 0, nil
				}
				return 0, fmt.Errorf("GetNumMsgsAtHeight: db.View error: %w", err)
			}

			if len(value) != 4 {
				return 0, fmt.Errorf("GetNumMsgsAtHeight: unexpected value size %d", len(value))
			}
			return binary.BigEndian.Uint32(value), nil
		} else {
			return 0, fmt.Errorf("GetNumMsgsAtHeight: batch.Get error: %w", err)
		}
	}
	defer closer.Close()
	if len(val) != 4 {
		return 0, fmt.Errorf("GetNumMsgsAtHeight: unexpected value size %d", len(val))
	}
	return binary.BigEndian.Uint32(val), nil
}

// Todo:
func (s *TMDB) DeleteMsgsAtHeight(height height) error {
	return nil
}

func (s *TMDB) SetNumMsgsAtHeight(height height, walIter uint32) error {
	heightBytes := heightToBytes(height)
	key := db.NumMsgsAtHeight.Key(heightBytes)
	val := encodeNumMsgsAtHeight(walIter)
	return s.batch.Set(key, val, nil)
}

func SetWAL[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *TMDB, msg M, to *timeout, height height) error {
	var (
		dataType string
		data     []byte
		err      error
	)

	if to != nil {
		data, err = to.MarshalCBOR()
		if err != nil {
			return fmt.Errorf("SetWAL: marshal inner timeout failed: %w", err)
		}
		dataType = "timeout"
	} else {
		dataType, err = msgType[V, H, A](msg)
		if err != nil {
			return fmt.Errorf("SetWAL: failed to get msg type: %w", err)
		}
		data, err = MarshalMsg[V, H, A](msg)
		if err != nil {
			return fmt.Errorf("SetWAL: marshal message failed: %w", err)
		}
	}

	// Wrap type + data
	wrapper := struct {
		Type string `cbor:"type"`
		Data []byte `cbor:"data"`
	}{
		Type: dataType,
		Data: data,
	}

	msgData, err := cbor.Marshal(wrapper)
	if err != nil {
		return fmt.Errorf("SetWAL: marshal wrapper failed: %w", err)
	}

	numMsgsAtHeight, err := s.GetNumMsgsAtHeight(height)
	if err != nil {
		return fmt.Errorf("SetWAL: failed to get numMsgsAtHeight: %w", err)
	}
	numMsgsAtHeight++
	numMsgsAtHeightInc := encodeNumMsgsAtHeight(numMsgsAtHeight)

	key := db.MsgsAtHeight.Key(heightToBytes(height), numMsgsAtHeightInc)

	return s.batch.Set(key, msgData, pebble.Sync)
}

func GetWALMsgs[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *TMDB, height height) ([]WALMsg[V, H, A, M], error) {
	rawEntries, err := scanWALRaw(s, height)
	if err != nil {
		return nil, err
	}
	if len(rawEntries) == 0 {
		return nil, nil
	}

	walMsgs := make([]WALMsg[V, H, A, M], 0, len(rawEntries))

	for _, value := range rawEntries {
		var wrapper struct {
			Type string `cbor:"type"`
			Data []byte `cbor:"data"`
		}
		if err := cbor.Unmarshal(value, &wrapper); err != nil {
			return nil, fmt.Errorf("GetWALMsgs: failed to decode wrapper: %w", err)
		}

		switch wrapper.Type {
		case "timeout":
			var to timeout
			if err := to.UnmarshalCBOR(wrapper.Data); err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to unmarshal timeout: %w", err)
			}
			walMsgs = append(walMsgs, WALMsg[V, H, A, M]{Timeout: &to})

		default:
			msg, err := UnmarshalMsg[V, H, A, M](wrapper.Data)
			if err != nil {
				return nil, fmt.Errorf("GetWALMsgs: failed to unmarshal message: %w", err)
			}
			walMsgs = append(walMsgs, WALMsg[V, H, A, M]{Msg: &msg})
		}
	}

	return walMsgs, nil
}

func scanWALRaw(s *TMDB, height height) ([][]byte, error) {
	prefix := db.MsgsAtHeight.Key(encodeNumMsgsAtHeight(uint32(height)))
	startKey := db.MsgsAtHeight.Key(encodeNumMsgsAtHeight(uint32(height)), encodeNumMsgsAtHeight(0))

	rawEntries := [][]byte{}

	err := s.db.View(func(txn db.Transaction) error {
		iter, err := txn.NewIterator(nil, false)
		if err != nil {
			return fmt.Errorf("scanWALRaw: failed to create iterator: %w", err)
		}
		defer iter.Close()

		if !iter.Seek(startKey) {
			// No entries at or after this key
			return nil
		}

		for ; iter.Valid(); iter.Next() {
			k := iter.Key()
			if !bytes.HasPrefix(k, prefix) {
				break // Done scanning this height
			}

			v, err := iter.Value()
			if err != nil {
				return fmt.Errorf("scanWALRaw: failed to get value: %w", err)
			}

			rawEntries = append(rawEntries, append([]byte(nil), v...))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return rawEntries, nil
}

func UnmarshalMsg[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](value []byte) (M, error) {
	var wrapper struct {
		Type string          `json:"type"`
		Data cbor.RawMessage `json:"data"`
	}

	var zero M

	if err := cbor.Unmarshal(value, &wrapper); err != nil {
		return zero, fmt.Errorf("UnmarshalMsg: failed to unmarshal wrapper: %w", err)
	}

	switch wrapper.Type {
	case "proposal":
		var proposal Proposal[V, H, A]
		if err := proposal.UnmarshalCBOR(wrapper.Data); err != nil {
			return zero, fmt.Errorf("UnmarshalMsg: Proposal.UnmarshalCBOR failed: %w", err)
		}
		return any(proposal).(M), nil

	case "prevote", "precommit": // Todo: we treat both identically here..
		var vote Vote[H, A]
		if err := vote.UnmarshalCBOR(wrapper.Data); err != nil {
			return zero, fmt.Errorf("UnmarshalMsg: Vote.UnmarshalCBOR failed for %q: %w", wrapper.Type, err)
		}
		return any(vote).(M), nil

	default:
		return zero, fmt.Errorf("UnmarshalMsg: unknown type %q", wrapper.Type)
	}
}

func MarshalMsg[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](msg M) (cbor.RawMessage, error) {
	var (
		data []byte
		err  error
	)

	switch m := any(msg).(type) {
	case Proposal[V, H, A]:
		data, err = m.MarshalCBOR()

	case Prevote[H, A]:
		vote := Vote[H, A](m)
		data, err = vote.MarshalCBOR()

	case Precommit[H, A]:
		vote := Vote[H, A](m)
		data, err = vote.MarshalCBOR()
	default:
		return nil, fmt.Errorf("MarshalMsg: unknown message type")
	}
	if err != nil {
		return nil, fmt.Errorf("MarshalMsg: marshal inner data failed: %w", err)
	}

	return data, nil
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

// Todo: consider enum
func msgType[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](msg M) (string, error) {
	switch any(msg).(type) {
	case Proposal[V, H, A]:
		return "proposal", nil

	case Prevote[H, A]:
		return "prevote", nil

	case Precommit[H, A]:
		return "precommit", nil

	}
	return "", fmt.Errorf("MarshalMsg: unknown message type")
}
