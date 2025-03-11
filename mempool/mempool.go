package mempool

import (
	"errors"
	"fmt"
	"sync"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
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
	db          db.KeyValueStore // to store the persistent mempool
	txPushed    chan struct{}
	memTxnList  *memTxnList
	maxNumTxns  int
	dbWriteChan chan *BroadcastedTransaction
	wg          sync.WaitGroup
}

// New initialises the Pool and starts the database writer goroutine.
// It is the responsibility of the caller to execute the closer function.
func New(mainDB db.KeyValueStore, state core.StateReader, maxNumTxns int, log utils.SimpleLogger) (*Pool, func() error) {
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
	headVal, err := GetHeadValue(p.db)
	if err != nil {
		if errors.Is(err, db.ErrKeyNotFound) {
			return nil
		}
		return err
	}

	currentHash := &headVal
	for currentHash != nil {
		curTxn, err := GetTxn(p.db, currentHash)
		if err != nil {
			return err
		}
		newMemPoolTxn := &memPoolTxn{Txn: curTxn.Txn}
		if curTxn.NextHash != nil {
			nextDBTxn, err := GetTxn(p.db, curTxn.NextHash)
			if err != nil {
				return err
			}
			newMemPoolTxn = &memPoolTxn{Txn: nextDBTxn.Txn}
		}
		p.memTxnList.push(newMemPoolTxn)
		currentHash = curTxn.NextHash
	}

	return nil
}

// writeToDB adds the transaction to the persistent pool db
func (p *Pool) writeToDB(userTxn *BroadcastedTransaction) error {
	batch := p.db.NewBatch()

	var tailVal *felt.Felt
	val, err := GetTailValue(p.db)
	if err != nil {
		if !errors.Is(err, db.ErrKeyNotFound) {
			return err
		}
		tailVal = nil
	} else {
		tailVal = &val
	}

	if err := WriteTxn(batch, &dbPoolTxn{Txn: *userTxn}); err != nil {
		return err
	}

	if tailVal != nil {
		// Update old tail to point to the new item
		var oldTailElem dbPoolTxn
		oldTailElem, err = GetTxn(p.db, tailVal)
		if err != nil {
			return err
		}
		oldTailElem.NextHash = userTxn.Transaction.Hash()
		if err := WriteTxn(batch, &oldTailElem); err != nil {
			return err
		}
	} else {
		// Empty list, make new item both the head and the tail
		if err := WriteHeadValue(batch, userTxn.Transaction.Hash()); err != nil {
			return err
		}
	}

	if err := WriteTailValue(batch, userTxn.Transaction.Hash()); err != nil {
		return err
	}

	pLen, err := GetLenDB(p.db)
	if err != nil {
		return err
	}

	if err := WriteLenDB(batch, pLen+1); err != nil {
		return err
	}

	return batch.Write()
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

func (p *Pool) Wait() <-chan struct{} {
	return p.txPushed
}

func (p *Pool) LenDB() (int, error) {
	return GetLenDB(p.db)
}
