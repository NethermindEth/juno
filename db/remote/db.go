package remote

import (
	"context"

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
	txClient, err := d.kvClient.Tx(d.ctx)
	if err != nil {
		return nil, err
	}

	if d.listener != nil {
		d.listener.OnNewTransaction(write)
	}

	return &transaction{client: txClient, log: d.log}, nil
}

func (d *DB) View(fn func(txn db.Transaction) error) error {
	if d.listener != nil {
		d.listener.OnView()
	}

	return db.View(d, fn)
}

func (d *DB) Update(fn func(txn db.Transaction) error) error {
	if d.listener != nil {
		d.listener.OnUpdate()
	}

	return db.Update(d, fn)
}

func (d *DB) WithListener(listener db.EventListener) db.DB {
	d.listener = listener
	return d
}

func (d *DB) Close() error {
	if d.listener != nil {
		d.listener.OnClose()
	}

	return d.grpcClient.Close()
}

func (d *DB) Impl() any {
	return d.kvClient
}
