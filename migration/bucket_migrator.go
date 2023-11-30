package migration

import (
	"bytes"
	"context"
	"fmt"
	"runtime"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
	"github.com/sourcegraph/conc/pool"
)

var _ Migration = (*BucketMigrator)(nil)

type (
	BucketMigratorDoFunc    func(t db.Transaction, b1, b2 []byte, n utils.Network) error
	BucketMigratorKeyFilter func([]byte) (bool, error)
)

type BucketMigrator struct {
	target db.Bucket

	// number of entries to update before returning migration.ErrCallWithNewTransaction
	batchSize uint
	before    func()
	keyFilter BucketMigratorKeyFilter
	do        BucketMigratorDoFunc

	// key to seek to when starting the migration
	startFrom []byte
}

func NewBucketMigrator(target db.Bucket, do BucketMigratorDoFunc) *BucketMigrator {
	return &BucketMigrator{
		target:    target,
		startFrom: target.Key(),
		batchSize: 1_000_000,

		before:    func() {},
		keyFilter: func(b []byte) (bool, error) { return true, nil },
		do:        do,
	}
}

func NewBucketMover(source, destination db.Bucket) *BucketMigrator {
	return NewBucketMigrator(source, func(txn db.Transaction, key, value []byte, n utils.Network) error {
		err := txn.Delete(key)
		if err != nil {
			return err
		}

		key[0] = byte(destination)
		return txn.Set(key, value)
	})
}

func (m *BucketMigrator) WithBatchSize(batchSize uint) *BucketMigrator {
	m.batchSize = batchSize
	return m
}

func (m *BucketMigrator) WithBefore(before func()) *BucketMigrator {
	m.before = before
	return m
}

func (m *BucketMigrator) WithKeyFilter(keyFilter BucketMigratorKeyFilter) *BucketMigrator {
	m.keyFilter = keyFilter
	return m
}

func (m *BucketMigrator) Before(_ []byte) error {
	m.before()
	return nil
}

func (m *BucketMigrator) Migrate(_ context.Context, txn db.Transaction, network utils.Network) ([]byte, error) {
	remainingInBatch := m.batchSize
	iterator, err := txn.NewIterator()
	if err != nil {
		return nil, fmt.Errorf("new iterator: %v", err)
	}

	keyValue := make([][2][]byte, 0)
	callWithNewTransaction := false
	for iterator.Seek(m.startFrom); iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if !bytes.HasPrefix(key, m.target.Key()) {
			break
		}
		if do, filterErr := m.keyFilter(key); filterErr != nil {
			return nil, fmt.Errorf("filter: %v", filterErr)
		} else if !do {
			continue
		}
		if remainingInBatch == 0 {
			m.startFrom = key
			callWithNewTransaction = true
			break
		}
		remainingInBatch--

		var value []byte
		value, err = iterator.Value()
		if err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, fmt.Errorf("get value from iterator: %v", err))
		}
		keyValue = append(keyValue, [2][]byte{key, value})
	}
	if err = iterator.Close(); err != nil {
		return nil, fmt.Errorf("close iterator: %v", err)
	}

	syncTxn := db.NewSyncTransaction(txn)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := pool.New().WithErrors().WithFirstError().WithMaxGoroutines(3 * runtime.GOMAXPROCS(0))
	for _, kv := range keyValue {
		if ctx.Err() != nil {
			break
		}
		kv := kv
		p.Go(func() error {
			if doErr := m.do(syncTxn, kv[0], kv[1], network); doErr != nil {
				cancel()
				return fmt.Errorf("do: %v", doErr)
			}
			return nil
		})
	}
	if err = p.Wait(); err != nil {
		return nil, fmt.Errorf("worker: %v", err)
	}

	if callWithNewTransaction {
		return nil, ErrCallWithNewTransaction
	}
	return nil, nil
}
