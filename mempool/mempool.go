package mempool

import (
	"errors"
	"fmt"
	"math/big"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/utils"
)

var ErrTxnPoolFull = errors.New("transaction pool is full")

type BroadcastedTransaction struct {
	Transaction   core.Transaction
	DeclaredClass core.Class
}

// runtime mempool txn
type memPoolTxn struct {
	Txn  BroadcastedTransaction
	Next *memPoolTxn
}

// persistent db txn value
type dbPoolTxn struct {
	Txn      BroadcastedTransaction
	NextHash *felt.Felt
}

// memTxnList represents a linked list of user transactions at runtime
type memTxnList struct {
	head *memPoolTxn
	tail *memPoolTxn
	len  int
	mu   sync.Mutex
}

func (t *memTxnList) push(newNode *memPoolTxn) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tail != nil {
		t.tail.Next = newNode
		t.tail = newNode
	} else {
		t.head = newNode
		t.tail = newNode
	}
	t.len++
}

func (t *memTxnList) pop() (BroadcastedTransaction, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.head == nil {
		return BroadcastedTransaction{}, errors.New("transaction pool is empty")
	}

	headNode := t.head
	t.head = headNode.Next
	if t.head == nil {
		t.tail = nil
	}
	t.len--
	return headNode.Txn, nil
}

// Pool represents a blockchain mempool, managing transactions using both an
// in-memory and persistent database.
type Pool struct {
	log         utils.SimpleLogger
	state       core.StateReader
	db          db.DB // to store the persistent mempool
	txPushed    chan struct{}
	memTxnList  *memTxnList
	maxNumTxns  int
	dbWriteChan chan *BroadcastedTransaction
	wg          sync.WaitGroup
}

// New initialises the Pool and starts the database writer goroutine.
// It is the responsibility of the caller to execute the closer function.
func New(mainDB db.DB, state core.StateReader, maxNumTxns int, log utils.SimpleLogger) (*Pool, func() error) {
	pool := &Pool{
		log:         log,
		state:       state,
		db:          mainDB, // todo: txns should be deleted everytime a new block is stored (builder responsibility)
		txPushed:    make(chan struct{}, 1),
		memTxnList:  &memTxnList{},
		maxNumTxns:  maxNumTxns,
		dbWriteChan: make(chan *BroadcastedTransaction, maxNumTxns),
	}
	closer := func() error {
		close(pool.dbWriteChan)
		pool.wg.Wait()
		if err := pool.db.Close(); err != nil {
			return fmt.Errorf("failed to close mempool database: %v", err)
		}
		return nil
	}
	pool.dbWriter()
	return pool, closer
}

func (p *Pool) dbWriter() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for txn := range p.dbWriteChan {
			err := p.writeToDB(txn)
			if err != nil {
				p.log.Errorw("error in handling user transaction in persistent mempool", "err", err)
			}
		}
	}()
}

// LoadFromDB restores the in-memory transaction pool from the database
func (p *Pool) LoadFromDB() error {
	return p.db.View(func(txn db.Transaction) error {
		headValue := new(felt.Felt)
		err := p.headHash(txn, headValue)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return nil
			}
			return err
		}
		// loop through the persistent pool and push nodes to the in-memory pool
		currentHash := headValue
		for currentHash != nil {
			curDBElem, err := p.readDBElem(txn, currentHash)
			if err != nil {
				return err
			}
			newMemPoolTxn := &memPoolTxn{
				Txn: curDBElem.Txn,
			}
			if curDBElem.NextHash != nil {
				nextDBTxn, err := p.readDBElem(txn, curDBElem.NextHash)
				if err != nil {
					return err
				}
				newMemPoolTxn.Next = &memPoolTxn{
					Txn: nextDBTxn.Txn,
				}
			}
			p.memTxnList.push(newMemPoolTxn)
			currentHash = curDBElem.NextHash
		}
		return nil
	})
}

// writeToDB adds the transaction to the persistent pool db
func (p *Pool) writeToDB(userTxn *BroadcastedTransaction) error {
	return p.db.Update(func(dbTxn db.Transaction) error {
		tailValue := new(felt.Felt)
		if err := p.tailValue(dbTxn, tailValue); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}
			tailValue = nil
		}
		if err := p.setDBElem(dbTxn, &dbPoolTxn{Txn: *userTxn}); err != nil {
			return err
		}
		if tailValue != nil {
			// Update old tail to point to the new item
			var oldTailElem dbPoolTxn
			oldTailElem, err := p.readDBElem(dbTxn, tailValue)
			if err != nil {
				return err
			}
			oldTailElem.NextHash = userTxn.Transaction.Hash()
			if err = p.setDBElem(dbTxn, &oldTailElem); err != nil {
				return err
			}
		} else {
			// Empty list, make new item both the head and the tail
			if err := p.updateHead(dbTxn, userTxn.Transaction.Hash()); err != nil {
				return err
			}
		}
		if err := p.updateTail(dbTxn, userTxn.Transaction.Hash()); err != nil {
			return err
		}
		pLen, err := p.lenDB(dbTxn)
		if err != nil {
			return err
		}
		return p.updateLen(dbTxn, pLen+1)
	})
}

// Push queues a transaction to the pool
func (p *Pool) Push(userTxn *BroadcastedTransaction) error {
	err := p.validate(userTxn)
	if err != nil {
		return err
	}

	select {
	case p.dbWriteChan <- userTxn:
	default:
		select {
		case _, ok := <-p.dbWriteChan:
			if !ok {
				p.log.Errorw("cannot store user transasction in persistent pool, database write channel is closed")
			}
			p.log.Errorw("cannot store user transasction in persistent pool, database is full")
		default:
			p.log.Errorw("cannot store user transasction in persistent pool, database is full")
		}
	}

	newNode := &memPoolTxn{Txn: *userTxn, Next: nil}
	p.memTxnList.push(newNode)

	select {
	case p.txPushed <- struct{}{}:
	default:
	}

	return nil
}

func (p *Pool) validate(userTxn *BroadcastedTransaction) error {
	if p.memTxnList.len+1 >= p.maxNumTxns {
		return ErrTxnPoolFull
	}

	switch t := userTxn.Transaction.(type) {
	case *core.DeployTransaction:
		return fmt.Errorf("deploy transactions are not supported")
	case *core.DeployAccountTransaction:
		if !t.Nonce.IsZero() {
			return fmt.Errorf("validation failed, received non-zero nonce %s", t.Nonce)
		}
	case *core.DeclareTransaction:
		nonce, err := p.state.ContractNonce(t.SenderAddress)
		if err != nil {
			return fmt.Errorf("validation failed, error when retrieving nonce, %v", err)
		}
		if nonce.Cmp(t.Nonce) > 0 {
			return fmt.Errorf("validation failed, existing nonce %s, but received nonce %s", nonce, t.Nonce)
		}
	case *core.InvokeTransaction:
		if t.TxVersion().Is(0) { // cant verify nonce since SenderAddress was only added in v1
			return fmt.Errorf("invoke v0 transactions not supported")
		}
		nonce, err := p.state.ContractNonce(t.SenderAddress)
		if err != nil {
			return fmt.Errorf("validation failed, error when retrieving nonce, %v", err)
		}
		if nonce.Cmp(t.Nonce) > 0 {
			return fmt.Errorf("validation failed, existing nonce %s, but received nonce %s", nonce, t.Nonce)
		}
	case *core.L1HandlerTransaction:
		// todo: verification of the L1 handler nonce requires checking the
		// message nonce on the L1 Core Contract.
	}
	return nil
}

// Pop returns the transaction with the highest priority from the in-memory pool
func (p *Pool) Pop() (BroadcastedTransaction, error) {
	return p.memTxnList.pop()
}

// Remove removes a set of transactions from the pool
// todo: should be called by the builder to remove txns from the db everytime a new block is stored.
// todo: in the consensus+p2p world, the txns should also be removed from the in-memory pool.
func (p *Pool) Remove(hash ...*felt.Felt) error {
	return errors.New("not implemented")
}

// Len returns the number of transactions in the in-memory pool
func (p *Pool) Len() int {
	return p.memTxnList.len
}

// Len returns the number of transactions in the persistent pool
func (p *Pool) LenDB() (int, error) {
	p.wg.Add(1)
	defer p.wg.Done()
	txn, err := p.db.NewTransaction(false)
	if err != nil {
		return 0, err
	}
	lenDB, err := p.lenDB(txn)
	if err != nil {
		return 0, err
	}
	return lenDB, txn.Discard()
}

func (p *Pool) lenDB(txn db.Transaction) (int, error) {
	var l int
	err := txn.Get(db.MempoolLength.Key(), func(b []byte) error {
		l = int(new(big.Int).SetBytes(b).Int64())
		return nil
	})

	if err != nil && errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	return l, err
}

func (p *Pool) updateLen(txn db.Transaction, l int) error {
	return txn.Set(db.MempoolLength.Key(), new(big.Int).SetInt64(int64(l)).Bytes())
}

func (p *Pool) Wait() <-chan struct{} {
	return p.txPushed
}

func (p *Pool) headHash(txn db.Transaction, head *felt.Felt) error {
	return txn.Get(db.MempoolHead.Key(), func(b []byte) error {
		head.SetBytes(b)
		return nil
	})
}

func (p *Pool) updateHead(txn db.Transaction, head *felt.Felt) error {
	return txn.Set(db.MempoolHead.Key(), head.Marshal())
}

func (p *Pool) tailValue(txn db.Transaction, tail *felt.Felt) error {
	return txn.Get(db.MempoolTail.Key(), func(b []byte) error {
		tail.SetBytes(b)
		return nil
	})
}

func (p *Pool) updateTail(txn db.Transaction, tail *felt.Felt) error {
	return txn.Set(db.MempoolTail.Key(), tail.Marshal())
}

func (p *Pool) readDBElem(txn db.Transaction, itemKey *felt.Felt) (dbPoolTxn, error) {
	var item dbPoolTxn
	keyBytes := itemKey.Bytes()
	err := txn.Get(db.MempoolNode.Key(keyBytes[:]), func(b []byte) error {
		return encoder.Unmarshal(b, &item)
	})
	return item, err
}

func (p *Pool) setDBElem(txn db.Transaction, item *dbPoolTxn) error {
	itemBytes, err := encoder.Marshal(item)
	if err != nil {
		return err
	}
	keyBytes := item.Txn.Transaction.Hash().Bytes()
	return txn.Set(db.MempoolNode.Key(keyBytes[:]), itemBytes)
}
