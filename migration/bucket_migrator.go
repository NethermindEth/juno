package migration

import (
	"bytes"

	"github.com/NethermindEth/juno/db"
)

var _ Migration = (*BucketMigrator)(nil)

type (
	BucketMigratorDoFunc    func(t db.Transaction, b1, b2 []byte) error
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
	return NewBucketMigrator(source, func(txn db.Transaction, key, value []byte) error {
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

func (m *BucketMigrator) Before() {
	m.before()
}

func (m *BucketMigrator) Migrate(txn db.Transaction) error {
	remainingInBatch := m.batchSize
	iterator, err := txn.NewIterator()
	if err != nil {
		return err
	}

	for iterator.Seek(m.startFrom); iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if !bytes.HasPrefix(key, m.target.Key()) {
			break
		}

		if pass, err := m.keyFilter(key); err != nil {
			return db.CloseAndWrapOnError(iterator.Close, err)
		} else if pass {
			if remainingInBatch == 0 {
				m.startFrom = key
				return db.CloseAndWrapOnError(iterator.Close, ErrCallWithNewTransaction)
			}

			remainingInBatch--
			value, err := iterator.Value()
			if err != nil {
				return db.CloseAndWrapOnError(iterator.Close, err)
			}

			if err = m.do(txn, key, value); err != nil {
				return db.CloseAndWrapOnError(iterator.Close, err)
			}
		}
	}

	return iterator.Close()
}
