package remote

import (
	"context"
	"math"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
	"google.golang.org/grpc"
)

var _ db.DB = (*DB)(nil)

type DB struct {
	ctx context.Context

	grpcClient *grpc.ClientConn
	kvClient   gen.KVClient
	log        utils.SimpleLogger
}

func New(rawURL string, ctx context.Context, log utils.SimpleLogger, opts ...grpc.DialOption) (*DB, error) {
	grpcClient, err := grpc.Dial(rawURL, opts...)
	if err != nil {
		return nil, err
	}

	return &DB{
		ctx:        ctx,
		grpcClient: grpcClient,
		kvClient:   gen.NewKVClient(grpcClient),
		log:        log,
	}, nil
}

func (d *DB) NewTransaction(write bool) (db.Transaction, error) {
	txClient, err := d.kvClient.Tx(d.ctx, grpc.MaxCallSendMsgSize(math.MaxInt), grpc.MaxCallRecvMsgSize(math.MaxInt))
	if err != nil {
		return nil, err
	}

	return &transaction{client: txClient, log: d.log}, nil
}

func (d *DB) View(fn func(txn db.Transaction) error) error {
	return db.View(d, fn)
}

func (d *DB) Update(fn func(txn db.Transaction) error) error {
	return db.Update(d, fn)
}

func (d *DB) WithListener(listener db.EventListener) db.DB {
	return d
}

func (d *DB) Close() error {
	return d.grpcClient.Close()
}

func (d *DB) Impl() any {
	return d.kvClient
}
