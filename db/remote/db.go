package remote

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"google.golang.org/grpc"
)

var _ db.KeyValueStore = (*DB)(nil)

type DB struct {
	ctx        context.Context
	grpcClient *grpc.ClientConn
	kvClient   gen.KVClient
	log        utils.StructuredLogger
	listener   db.EventListener
}

func New(
	rawURL string, ctx context.Context, log utils.StructuredLogger, opts ...grpc.DialOption,
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
		log:        log,
		listener:   listener,
	}, nil
}

func (d *DB) NewTransaction(write bool) (*transaction, error) {
	defer d.listener.OnIO(write, time.Now())

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		return nil, err
	}

	return &transaction{client: txClient, log: d.log}, nil
}

func (d *DB) View(fn func(txn db.Snapshot) error) error {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return err
	}

	defer discardTxnOnPanic(txn)
	return utils.RunAndWrapOnError(txn.Discard, fn(txn))
}

func (d *DB) Update(fn func(txn db.IndexedBatch) error) error {
	defer d.listener.OnCommit(time.Now())

	txn, err := d.NewTransaction(true)
	if err != nil {
		return err
	}

	defer discardTxnOnPanic(txn)
	if err := fn(txn); err != nil {
		return utils.RunAndWrapOnError(txn.Discard, err)
	}

	return utils.RunAndWrapOnError(txn.Discard, txn.Commit())
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

	return txn.Get(key, cb)
}

func (d *DB) Has(key []byte) (bool, error) {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return false, err
	}

	return txn.Has(key)
}

func (d *DB) Put(key, val []byte) error {
	return errNotSupported
}

func (d *DB) NewBatch() db.Batch {
	defer d.listener.OnIO(false, time.Now())

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		panic(err)
	}

	return &transaction{client: txClient, log: d.log}
}

func (d *DB) NewBatchWithSize(size int) db.Batch {
	return d.NewBatch()
}

func (d *DB) NewIndexedBatch() db.IndexedBatch {
	defer d.listener.OnIO(true, time.Now())

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		panic(err)
	}

	return &transaction{client: txClient, log: d.log}
}

func (d *DB) NewIndexedBatchWithSize(size int) db.IndexedBatch {
	return d.NewIndexedBatch()
}

func (d *DB) NewIterator(start []byte, withUpperBound bool) (db.Iterator, error) {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return nil, err
	}

	return txn.NewIterator(start, withUpperBound)
}

func (d *DB) NewSnapshot() db.Snapshot {
	defer d.listener.OnIO(false, time.Now())

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		panic(err)
	}

	return &transaction{client: txClient, log: d.log}
}

func (d *DB) WithListener(listener db.EventListener) db.KeyValueStore {
	d.listener = listener
	return d
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
