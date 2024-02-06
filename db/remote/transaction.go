package remote

import (
	"bytes"
	"errors"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils"
)

var _ db.Transaction = (*transaction)(nil)

type transaction struct {
	client gen.KV_TxClient
	log    utils.SimpleLogger
}

func (t *transaction) NewIterator() (db.Iterator, error) {
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
	return errors.New("read only DB")
}

func (t *transaction) Set(key, val []byte) error {
	return errors.New("read only DB")
}

func (t *transaction) Delete(key []byte) error {
	return errors.New("read only DB")
}

func (t *transaction) Get(key []byte, cb func([]byte) error) error {
	err := t.client.Send(&gen.Cursor{
		Op: gen.Op_GET,
		K:  key,
	})
	if err != nil {
		return err
	}

	pair, err := t.client.Recv()
	if err != nil {
		return err
	}

	if !bytes.Equal(key, pair.K) {
		return db.ErrKeyNotFound
	}
	return cb(pair.V)
}

func (t *transaction) Impl() any {
	return t.client
}
