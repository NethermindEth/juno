package remote

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils/log"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

var _ db.KeyValueStore = (*DB)(nil)

type DB struct {
	ctx        context.Context
	grpcClient *grpc.ClientConn
	kvClient   gen.KVClient
	logger     log.StructuredLogger
	listener   db.EventListener
}

func New(
	rawURL string, ctx context.Context, logger log.StructuredLogger, opts ...grpc.DialOption,
) (*DB, error) {
	grpcClient, err := grpc.NewClient(rawURL, opts...)
	if err != nil {
		return nil, err
	}

	listener := &db.SelectiveListener{
		OnIOCb:     func(write bool, duration time.Duration) {},
		OnCommitCb: func(duration time.Duration) {},
	}

	return &DB{
		ctx:        ctx,
		grpcClient: grpcClient,
		kvClient:   gen.NewKVClient(grpcClient),
		logger:     logger,
		listener:   listener,
	}, nil
}

func (d *DB) Path() string {
	// Remote DB has no local filesystem path.
	return ""
}

func (d *DB) NewTransaction(write bool) (*transaction, error) {
	defer d.listener.OnIO(write, time.Now())

	// Every transaction owns a stream, so it needs its own context to release it.
	ctx, cancel := context.WithCancel(d.ctx)
	txClient, err := d.kvClient.Tx(
		ctx,
		grpc.MaxCallSendMsgSize(math.MaxInt),
		grpc.MaxCallRecvMsgSize(math.MaxInt),
	)
	if err != nil {
		cancel()
		return nil, err
	}

	return &transaction{client: txClient, cancel: cancel, logger: d.logger}, nil
}

func (d *DB) Update(fn func(txn db.IndexedBatch) error) error {
	defer d.listener.OnCommit(time.Now())

	txn, err := d.NewTransaction(true)
	if err != nil {
		return err
	}

	defer discardTxnOnPanic(txn)
	if err := fn(txn); err != nil {
		return errors.Join(err, txn.Discard())
	}

	return errors.Join(txn.Commit(), txn.Discard())
}

func (d *DB) Write(fn func(w db.Batch) error) error {
	defer d.listener.OnCommit(time.Now())

	batch := d.NewBatch()
	if err := fn(batch); err != nil {
		return errors.Join(err, batch.Close())
	}

	return errors.Join(batch.Write(), batch.Close())
}

func (d *DB) Close() error {
	return d.grpcClient.Close()
}

func (d *DB) Impl() any {
	return d.kvClient
}

func (d *DB) Delete(key []byte) error {
	return errNotSupported
}

func (d *DB) DeleteRange(start, end []byte) error {
	return errNotSupported
}

func (d *DB) Get(key []byte, cb func(value []byte) error) error {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return err
	}
	defer d.discard(txn)

	return txn.Get(key, cb)
}

func (d *DB) Has(key []byte) (bool, error) {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return false, err
	}
	defer d.discard(txn)

	return txn.Has(key)
}

func (d *DB) Put(key, val []byte) error {
	return errNotSupported
}

func (d *DB) NewBatch() db.Batch {
	txn, err := d.NewTransaction(false)
	if err != nil {
		panic(err)
	}

	return txn
}

func (d *DB) NewBatchWithSize(size int) db.Batch {
	return d.NewBatch()
}

func (d *DB) NewIndexedBatch() db.IndexedBatch {
	txn, err := d.NewTransaction(true)
	if err != nil {
		panic(err)
	}

	return txn
}

func (d *DB) NewIndexedBatchWithSize(size int) db.IndexedBatch {
	return d.NewIndexedBatch()
}

func (d *DB) NewIterator(start []byte, withUpperBound bool) (db.Iterator, error) {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return nil, err
	}

	it, err := txn.NewIterator(start, withUpperBound)
	if err != nil {
		return nil, errors.Join(err, txn.Discard())
	}

	return &ownedIterator{Iterator: it, txn: txn}, nil
}

func (d *DB) NewSnapshot() db.Snapshot {
	txn, err := d.NewTransaction(false)
	if err != nil {
		panic(err)
	}

	return txn
}

func (d *DB) WithListener(listener db.EventListener) db.KeyValueStore {
	d.listener = listener
	return d
}

// discard releases a transaction the DB opened for a single call. A read has
// nothing to report on close, so the error only reaches the log.
func (d *DB) discard(txn *transaction) {
	if err := txn.Discard(); err != nil {
		d.logger.Debug("Discarding remote transaction", zap.Error(err))
	}
}

func discardTxnOnPanic(txn *transaction) {
	p := recover()
	if p != nil {
		if err := txn.Discard(); err != nil {
			fmt.Fprintf(os.Stderr, "failed discarding panicing txn err: %s", err)
		}
		panic(p)
	}
}
