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

type storageElem struct {
	Txn      BroadcastedTransaction
	NextHash *felt.Felt   // persistent db
	Next     *storageElem // in-memory
}

type BroadcastedTransaction struct {
	Transaction   core.Transaction
	DeclaredClass core.Class
}

type txnList struct {
	head *storageElem
	tail *storageElem
	len  int
	mu   sync.Mutex
}

// Pool stores the transactions in a linked list for its inherent FCFS behaviour
type Pool struct {
	log         utils.SimpleLogger
	state       core.StateReader
	db          db.DB // persistent mempool
	txPushed    chan struct{}
	txnList     *txnList // in-memory
	maxNumTxns  int
	dbWriteChan chan *BroadcastedTransaction
	wg          sync.WaitGroup
}

// New initialises the Pool and starts the database writer goroutine.
// It is the responsibility of the user to call the cancel function if the context is cancelled
func New(persistentPool db.DB, state core.StateReader, maxNumTxns int, log utils.SimpleLogger) (*Pool, func() error, error) {
	pool := &Pool{
		log:         log,
		state:       state,
		db:          persistentPool, // todo: txns should be deleted everytime a new block is stored (builder responsibility)
		txPushed:    make(chan struct{}, 1),
		txnList:     &txnList{},
		maxNumTxns:  maxNumTxns,
		dbWriteChan: make(chan *BroadcastedTransaction, maxNumTxns),
	}

	if err := pool.loadFromDB(); err != nil {
		return nil, nil, fmt.Errorf("failed to load transactions from database into the in-memory transaction list: %v", err)
	}

	pool.wg.Add(1)
	go pool.dbWriter()
	closer := func() error {
		close(pool.dbWriteChan)
		pool.wg.Wait()
		if err := pool.db.Close(); err != nil {
			return fmt.Errorf("failed to close mempool database: %v", err)
		}
		return nil
	}
	return pool, closer, nil
}

func (p *Pool) dbWriter() {
	defer p.wg.Done()
	for txn := range p.dbWriteChan {
		err := p.handleTransaction(txn)
		p.log.Errorw("error in handling user transaction in persistent mempool", "err", err)
	}
}

// loadFromDB restores the in-memory transaction pool from the database
func (p *Pool) loadFromDB() error {
	return p.db.View(func(txn db.Transaction) error {
		headValue := new(felt.Felt)
		err := p.headHash(txn, headValue)
		if err != nil {
			if errors.Is(err, db.ErrKeyNotFound) {
				return nil
			}
			return err
		}

		currentHash := headValue
		for currentHash != nil {
			curElem, err := p.dbElem(txn, currentHash)
			if err != nil {
				return err
			}

			newNode := &storageElem{
				Txn: curElem.Txn,
			}

			if curElem.NextHash != nil {
				nxtElem, err := p.dbElem(txn, curElem.NextHash)
				if err != nil {
					return err
				}
				newNode.Next = &storageElem{
					Txn: nxtElem.Txn,
				}
			}

			p.txnList.mu.Lock()
			if p.txnList.tail != nil {
				p.txnList.tail.Next = newNode
				p.txnList.tail = newNode
			} else {
				p.txnList.head = newNode
				p.txnList.tail = newNode
			}
			p.txnList.len++
			p.txnList.mu.Unlock()

			currentHash = curElem.NextHash
		}

		return nil
	})
}

func (p *Pool) handleTransaction(userTxn *BroadcastedTransaction) error {
	return p.db.Update(func(dbTxn db.Transaction) error {
		tailValue := new(felt.Felt)
		if err := p.tailValue(dbTxn, tailValue); err != nil {
			if !errors.Is(err, db.ErrKeyNotFound) {
				return err
			}
			tailValue = nil
		}
		if err := p.putdbElem(dbTxn, userTxn.Transaction.Hash(), &storageElem{
			Txn: *userTxn,
		}); err != nil {
			return err
		}
		if tailValue != nil {
			// Update old tail to point to the new item
			var oldTailElem storageElem
			oldTailElem, err := p.dbElem(dbTxn, tailValue)
			if err != nil {
				return err
			}
			oldTailElem.NextHash = userTxn.Transaction.Hash()
			if err = p.putdbElem(dbTxn, tailValue, &oldTailElem); err != nil {
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

// Push queues a transaction to the pool and adds it to both the in-memory list and DB
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

	p.txnList.mu.Lock()
	newNode := &storageElem{Txn: *userTxn, Next: nil}
	if p.txnList.tail != nil {
		p.txnList.tail.Next = newNode
		p.txnList.tail = newNode
	} else {
		p.txnList.head = newNode
		p.txnList.tail = newNode
	}
	p.txnList.len++
	p.txnList.mu.Unlock()

	select {
	case p.txPushed <- struct{}{}:
	default:
	}

	return nil
}

func (p *Pool) validate(userTxn *BroadcastedTransaction) error {
	if p.txnList.len+1 >= p.maxNumTxns {
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
	p.txnList.mu.Lock()
	defer p.txnList.mu.Unlock()

	if p.txnList.head == nil {
		return BroadcastedTransaction{}, errors.New("transaction pool is empty")
	}

	headNode := p.txnList.head
	p.txnList.head = headNode.Next
	if p.txnList.head == nil {
		p.txnList.tail = nil
	}
	p.txnList.len--

	return headNode.Txn, nil
}

// Remove removes a set of transactions from the pool
// todo: should be called by the builder to remove txns from the db everytime a new block is stored.
// todo: in the consensus+p2p world, the txns should also be removed from the in-memory pool.
func (p *Pool) Remove(hash ...*felt.Felt) error {
	return errors.New("not implemented")
}

// Len returns the number of transactions in the in-memory pool
func (p *Pool) Len() int {
	return p.txnList.len
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
	err := txn.Get(Length.Key(), func(b []byte) error {
		l = int(new(big.Int).SetBytes(b).Int64())
		return nil
	})

	if err != nil && errors.Is(err, db.ErrKeyNotFound) {
		return 0, nil
	}
	return l, err
}

func (p *Pool) updateLen(txn db.Transaction, l int) error {
	return txn.Set(Length.Key(), new(big.Int).SetInt64(int64(l)).Bytes())
}

func (p *Pool) Wait() <-chan struct{} {
	return p.txPushed
}

func (p *Pool) headHash(txn db.Transaction, head *felt.Felt) error {
	return txn.Get(Head.Key(), func(b []byte) error {
		head.SetBytes(b)
		return nil
	})
}

func (p *Pool) updateHead(txn db.Transaction, head *felt.Felt) error {
	return txn.Set(Head.Key(), head.Marshal())
}

func (p *Pool) tailValue(txn db.Transaction, tail *felt.Felt) error {
	return txn.Get(Tail.Key(), func(b []byte) error {
		tail.SetBytes(b)
		return nil
	})
}

func (p *Pool) updateTail(txn db.Transaction, tail *felt.Felt) error {
	return txn.Set(Tail.Key(), tail.Marshal())
}

// todo : error when unmarshalling the core.Transasction...
// but unmarshalling core.Transaction works fine in TransactionsByBlockNumber...
func (p *Pool) dbElem(txn db.Transaction, itemKey *felt.Felt) (storageElem, error) {
	var item storageElem
	keyBytes := itemKey.Bytes()
	err := txn.Get(Node.Key(keyBytes[:]), func(b []byte) error {
		return encoder.Unmarshal(b, &item)
	})
	return item, err
}

func (p *Pool) putdbElem(txn db.Transaction, itemKey *felt.Felt, item *storageElem) error {
	itemBytes, err := encoder.Marshal(item)
	if err != nil {
		return err
	}
	keyBytes := itemKey.Bytes()
	return txn.Set(Node.Key(keyBytes[:]), itemBytes)
}
