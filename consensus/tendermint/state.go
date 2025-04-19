package tendermint

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

// Todo: just placing the DB logic here. Will likely be moved into another type

type State struct {
	db *db.DB
}

func NewState(db *db.DB) State {
	return State{db: db}
}

// Todo: key is WAL_prefix + height + iter
func StoreWALMsg[V Hashable[H], H Hash, A Addr, M Message[V, H, A]](txn db.Transaction, msg M) error {
	var height height
	var valBytes []byte
	var err error
	switch m := any(msg).(type) {
	case Proposal[V, H, A]:
		height = m.Height
		valBytes, err = encoder.Marshal(m) // Todo: register / handle encoding
		if err != nil {
			return err
		}
	case Prevote[H, A]:
		height = m.Height
		valBytes, err = encoder.Marshal(m) // Todo: register / handle encoding
		if err != nil {
			return err
		}
	case Precommit[H, A]:
		height = m.Height
		valBytes, err = encoder.Marshal(m) // Todo: register / handle encoding
		if err != nil {
			return err
		}
	default:
		panic("unexpected type in StoreWALMsg") // Todo: handle
	}

	key_bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(key_bytes, uint32(height))
	walIter := GetWAlIter(txn)

	key := db.WAL.Key(key_bytes, walIter)

	return txn.Set(key, valBytes)
}

func GetWAlIter(txn db.Transaction) []byte { // Todo: use fixed bytes?
	// Todo
	return []byte{}
}
