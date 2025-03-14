package remote

import (
	"bytes"
	"errors"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
)

var (
	errNotSupported = errors.New("not supported")
	errReadOnly     = errors.New("read only DB")
)

var (
	_ db.Batch        = (*transaction)(nil)
	_ db.IndexedBatch = (*transaction)(nil)
)

type transaction struct {
	client gen.KV_TxClient
	log    utils.SimpleLogger
}

func (t *transaction) NewIterator(_ []byte, _ bool) (db.Iterator, error) {
	err := t.client.Send(&gen.Cursor{
		Op: gen.Op_OPEN,
	})
	if err != nil {
		return nil, err
	}

	pair, err := t.client.Recv()
	if err != nil {
		return nil, err
	}

	return &iterator{
		client:   t.client,
		cursorID: pair.CursorId,
		log:      t.log,
	}, nil
}

func (t *transaction) Discard() error {
	return t.client.CloseSend()
}

func (t *transaction) Commit() error {
	return errReadOnly
}

func (t *transaction) Set(key, val []byte) error {
	return errReadOnly
}

func (t *transaction) Delete(key []byte) error {
	return errReadOnly
}

func (t *transaction) Get(key []byte) ([]byte, error) {
	err := t.client.Send(&gen.Cursor{
		Op: gen.Op_GET,
		K:  key,
	})
	if err != nil {
		return nil, err
	}

	pair, err := t.client.Recv()
	if err != nil {
		return nil, err
	}

	if !bytes.Equal(key, pair.K) {
		return nil, db.ErrKeyNotFound
	}
	return pair.V, nil
}

func (t *transaction) Has(key []byte) (bool, error) {
	_, err := t.Get(key)
	return err == nil, err
}

func (t *transaction) Impl() any {
	return t.client
}

func (t *transaction) Put(key, val []byte) error {
	return errReadOnly
}

func (t *transaction) Size() int    { return 0 }
func (t *transaction) Reset()       {}
func (t *transaction) Write() error { return nil }
