package tendermint

import (
	"encoding/binary"
	"fmt"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
)

// Todo: just placing the DB logic here. Will likely restructure

// Todo: remove this description?
// Example db layout for the NumMsgsAtHeight and Msgs format.
// Basically we store a sequence number (`NumMsgsAtHeight`) for the number of messages written to the db at a given height.
// The NumMsgsAtHeight is used to divide and locate the message keys efficiently.
//
// | Key                                  | Value                         | Meaning                                |
// |--------------------------------------|-------------------------------|----------------------------------------|
// | db/WalIter/000003E8                  | 0000002A                      | Last wal_iter=42 at height=1000        |
// | db/Msg/000003E8/00000000             | (CBOR-encoded Proposal)       | Proposal at height=1000, iter=0        |
// | db/Msg/000003E8/00000001             | (CBOR-encoded Prevote)        | Prevote at height=1000, iter=1         |
// | db/Msg/000003E8/00000002             | (CBOR-encoded Precommit)      | Precommit at height=1000, iter=2       |
// ....
// | db/Msg/000003E8/0000002A             | (CBOR-encoded Proposal)       | Proposal at height=1000, iter=42       |
// | db/WalIter/000003E9                  | 00000010                      | Last wal_iter=16 at height=1001        |
// | db/Msg/000003E9/00000000             | (CBOR-encoded Proposal)       | Proposal at height=1001, iter=0        |
// | db/Msg/000003E9/00000001             | (CBOR-encoded Prevote)        | Prevote at height=1001, iter=1         |
// ....
// | db/Msg/000003E9/00000010             | (CBOR-encoded Precommit)      | Precommit at height=1001, iter=16      |
//
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

type State struct { // Todo: move this elsewhere. Currently just a placeholder
	db    *db.DB
	batch *pebble.Batch
}

func NewState(db *db.DB) State {
	return State{db: db}
}

func (s *State) CommitBatch() error { // Todo: figure out when to call this to balance safety and performance
	return s.batch.Commit(pebble.Sync)
}

// Todo
func (s *State) SetWalSeqNum(height height) []byte {
	return []byte{}
}

// Todo: after we commit a block, we can delete old wal-msgs.
func (s *State) DeleteMsgsAtHeight(height height) error {
	// Delete both `MsgsAtHeight` and `NumMsgsAtHeight`
	return nil
}

// Todo
func (s *State) GetNumMsgsAtHeight(h height) []byte {
	return []byte{}
}

// Todo: methods + generics don't play well together
func SetWALMsg[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *State, msg M) error {
	var (
		height   height
		valBytes []byte
		err      error
	)
	switch m := any(msg).(type) {
	case Proposal[V, H, A]:
		height = m.Height
		valBytes, err = m.MarshalCBOR()

	case Prevote[H, A]:
		height = m.Height
		vote := Vote[H, A](m) // Free cast
		valBytes, err = vote.MarshalCBOR()

	case Precommit[H, A]:
		height = m.Height
		vote := Vote[H, A](m) // Free cast
		valBytes, err = vote.MarshalCBOR()
	default:
		panic("unexpected type in StoreWALMsg") // Todo: handle
	}

	if err != nil {
		return fmt.Errorf("StoreWALMsg: marshal error: %w", err)
	}

	key_bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(key_bytes, uint32(height))
	numMsgsAtHeight := s.GetNumMsgsAtHeight(height)

	key := db.MsgsAtHeight.Key(key_bytes, numMsgsAtHeight)

	return s.batch.Set(key, valBytes, pebble.Sync) // Note: Set() doesn't actually use `pebble.Sync` here
}

// Todo
func GetWALMsgs[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](s *State, height height) error {
	return nil
}
