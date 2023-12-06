package mempool

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

type ValidatorFunc func(*BroadcastedTransaction) error

type BroadcastedTransaction struct {
	Transaction   core.Transaction
	DeclaredClass core.Class
}

const (
	poolLengthKey = "poolLength"
	headKey       = "headKey"
	tailKey       = "tailKey"
)

// Pool stores the transactions in a linked list for its inherent FCFS behaviour
type storageElem struct {
	Txn      BroadcastedTransaction
	NextHash *felt.Felt
}

type Pool struct {
	db        db.DB
	validator ValidatorFunc
}

func New(poolDB db.DB) *Pool {
	return &Pool{
		db:        poolDB,
		validator: func(_ *BroadcastedTransaction) error { return nil },
	}
}

// WithValidator adds a validation step to be triggered before adding a
// BroadcastedTransaction to the pool
func (p *Pool) WithValidator(validator ValidatorFunc) *Pool {
	p.validator = validator
	return p
}

// Push queues a transaction to the pool
func (p *Pool) Push(userTxn *BroadcastedTransaction) error {
	err := p.validator(userTxn)
	if err != nil {
		return err
	}

	return p.db.Update(func(txn db.Transaction) error {
		tail, err := p.tailHash(txn)
		if err != nil && !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}

		if err = p.putElem(txn, userTxn.Transaction.Hash(), &storageElem{
			Txn: *userTxn,
		}); err != nil {
			return nil
		}

		if tail != nil {
			var oldTail storageElem
			oldTail, err = p.elem(txn, tail)
			if err != nil {
				return err
			}

			// update old tail to point to the new item
			oldTail.NextHash = userTxn.Transaction.Hash()
			if err = p.putElem(txn, tail, &oldTail); err != nil {
				return err
			}
		} else {
			// empty list, make new item both the head and the tail
			if err = p.updateHead(txn, userTxn.Transaction.Hash()); err != nil {
				return err
			}
		}

		if err = p.updateTail(txn, userTxn.Transaction.Hash()); err != nil {
			return err
		}

		pLen, err := p.len(txn)
		if err != nil {
			return err
		}
		return p.updateLen(txn, pLen+1) // don't worry about overflows, highly unlikely
	})
}

// Pop returns the transaction with the highest priority from the pool
func (p *Pool) Pop() (BroadcastedTransaction, error) {
	var nextTxn BroadcastedTransaction
	return nextTxn, p.db.Update(func(txn db.Transaction) error {
		headHash, err := p.headHash(txn)
		if err != nil {
			return err
		}

		headElem, err := p.elem(txn, headHash)
		if err != nil {
			return err
		}

		if err = txn.Delete(headHash.Marshal()); err != nil {
			return err
		}

		if headElem.NextHash == nil {
			// the list is empty now
			if err = txn.Delete([]byte(headKey)); err != nil {
				return err
			}
			if err = txn.Delete([]byte(tailKey)); err != nil {
				return err
			}
		} else {
			if err = p.updateHead(txn, headElem.NextHash); err != nil {
				return err
			}
		}

		pLen, err := p.len(txn)
		if err != nil {
			return err
		}

		if err = p.updateLen(txn, pLen-1); err != nil {
			return err
		}
		nextTxn = headElem.Txn
		return nil
	})
}

// Remove removes a set of transactions from the pool
func (p *Pool) Remove(hash ...*felt.Felt) error {
	return errors.New("not implemented")
}

// Len returns the number of transactions in the pool
func (p *Pool) Len() (uint64, error) {
	var l uint64
	return l, p.db.View(func(txn db.Transaction) error {
		var err error
		l, err = p.len(txn)
		return err
	})
}

func (p *Pool) len(txn db.Transaction) (uint64, error) {
	var l uint64
	err := txn.Get([]byte(poolLengthKey), func(b []byte) error {
		l = binary.BigEndian.Uint64(b)
		return nil
	})

	if err != nil && errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	return l, err
}

func (p *Pool) updateLen(txn db.Transaction, l uint64) error {
	return txn.Set([]byte(poolLengthKey), binary.BigEndian.AppendUint64(nil, l))
}

func (p *Pool) headHash(txn db.Transaction) (*felt.Felt, error) {
	var head *felt.Felt
	return head, txn.Get([]byte(headKey), func(b []byte) error {
		head = new(felt.Felt).SetBytes(b)
		return nil
	})
}

func (p *Pool) updateHead(txn db.Transaction, head *felt.Felt) error {
	return txn.Set([]byte(headKey), head.Marshal())
}

func (p *Pool) tailHash(txn db.Transaction) (*felt.Felt, error) {
	var tail *felt.Felt
	return tail, txn.Get([]byte(tailKey), func(b []byte) error {
		tail = new(felt.Felt).SetBytes(b)
		return nil
	})
}

func (p *Pool) updateTail(txn db.Transaction, tail *felt.Felt) error {
	return txn.Set([]byte(tailKey), tail.Marshal())
}

func (p *Pool) elem(txn db.Transaction, itemKey *felt.Felt) (storageElem, error) {
	var item storageElem
	err := txn.Get(itemKey.Marshal(), func(b []byte) error {
		return encoder.Unmarshal(b, &item)
	})
	return item, err
}

func (p *Pool) putElem(txn db.Transaction, itemKey *felt.Felt, item *storageElem) error {
	itemBytes, err := encoder.Marshal(item)
	if err != nil {
		return err
	}
	return txn.Set(itemKey.Marshal(), itemBytes)
}
