package remote

import (
	"context"
	"math"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"google.golang.org/grpc"
)

var (
	_ db.DB            = (*DB)(nil)
	_ db.KeyValueStore = (*DB)(nil)
)

type DB struct {
	ctx        context.Context
	grpcClient *grpc.ClientConn
	kvClient   gen.KVClient
	log        utils.SimpleLogger
	listener   db.EventListener
}

func New(rawURL string, ctx context.Context, log utils.SimpleLogger, opts ...grpc.DialOption) (*DB, error) {
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

func (d *DB) NewTransaction(write bool) (db.Transaction, error) {
	start := time.Now()

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		return nil, err
	}

	d.listener.OnIO(write, time.Since(start))

	return &transaction{client: txClient, log: d.log}, nil
}

func (d *DB) View(fn func(txn db.Transaction) error) error {
	return db.View(d, fn)
}

func (d *DB) Update(fn func(txn db.Transaction) error) error {
	start := time.Now()

	defer func() {
		d.listener.OnCommit(time.Since(start))
	}()

	return db.Update(d, fn)
}

func (d *DB) WithListener(listener db.EventListener) db.DB {
	d.listener = listener
	return d
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

func (d *DB) DeleteRange(start []byte, end []byte) error {
	return errNotSupported
}

func (d *DB) Get2(key []byte) ([]byte, error) {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return nil, err
	}

	var val []byte
	err = txn.Get(key, func(v []byte) error {
		val = v
		return nil
	})
	return val, err
}

func (d *DB) Has(key []byte) (bool, error) {
	txn, err := d.NewTransaction(false)
	if err != nil {
		return false, err
	}

	err = txn.Get(key, func(v []byte) error {
		return nil
	})
	return err == nil, err
}

func (d *DB) Put(key, val []byte) error {
	return errNotSupported
}

func (d *DB) NewBatch() db.Batch {
	start := time.Now()

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		panic(err)
	}

	d.listener.OnIO(false, time.Since(start))

	return &transaction{client: txClient, log: d.log}
}

func (d *DB) NewBatchWithSize(size int) db.Batch {
	return d.NewBatch()
}

func (d *DB) NewIndexedBatch() db.IndexedBatch {
	start := time.Now()

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		panic(err)
	}

	d.listener.OnIO(true, time.Since(start))

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
	start := time.Now()

	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		panic(err)
	}

	d.listener.OnIO(false, time.Since(start))

	return &transaction{client: txClient, log: d.log}
}

func (d *DB) Update2(fn func(txn db.IndexedBatch) error) error {
	return errNotSupported
}

func (d *DB) View2(fn func(txn db.Snapshot) error) error {
	return errNotSupported
}

func (d *DB) WithListener2(listener db.EventListener) db.KeyValueStore {
	d.listener = listener
	return d
}
