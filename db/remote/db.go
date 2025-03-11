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

var _ db.DB = (*DB)(nil)

type DB struct {
	ctx        context.Context
	grpcClient *grpc.ClientConn
	kvClient   gen.KVClient
	log        utils.SimpleLogger
	listener   db.EventListener
}

// TODO(weiihann): handle this remotedb with new interfaces
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
