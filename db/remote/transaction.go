package remote

import (
	"bytes"
	"errors"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/grpc/gen"
	"github.com/NethermindEth/juno/utils/log"
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
	logger log.StructuredLogger
}

func (t *transaction) NewIterator(prefix []byte, withUpperBound bool) (db.Iterator, error) {
	// The remote iterator has to be created with the same bounds as a local one,
	// otherwise it scans the whole database and First returns a foreign key.
	// BucketName carries the prefix and a non-empty V asks for the upper bound.
	cursor := &gen.Cursor{
		Op:         gen.Op_OPEN,
		BucketName: prefix,
	}
	if withUpperBound {
		cursor.V = []byte{1}
	}

	err := t.client.Send(cursor)
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
		logger:   t.logger,
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

func (t *transaction) DeleteRange(start, end []byte) error {
	return errReadOnly
}

func (t *transaction) Get(key []byte, cb func(value []byte) error) error {
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

func (t *transaction) Has(key []byte) (bool, error) {
	err := t.Get(key, func(_ []byte) error { return nil })
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
func (t *transaction) Close() error { return t.client.CloseSend() }
