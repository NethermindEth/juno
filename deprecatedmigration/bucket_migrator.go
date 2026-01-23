package deprecatedmigration

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var _ Migration = (*BucketMigrator)(nil)

type (
	BucketMigratorDoFunc    func(t db.KeyValueWriter, b1, b2 []byte, n *utils.Network) error
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
	return NewBucketMigrator(source, func(txn db.KeyValueWriter, key, value []byte, n *utils.Network) error {
		err := txn.Delete(key)
		if err != nil {
			return err
		}

		key[0] = byte(destination)
		return txn.Put(key, value)
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

func (m *BucketMigrator) Migrate(
	ctx context.Context,
	database db.KeyValueStore,
	network *utils.Network,
	log utils.SimpleLogger,
) ([]byte, error) {
	remainingInBatch := m.batchSize
	iterator, err := database.NewIterator(nil, false)
	if err != nil {
		return nil, err
	}

	var (
		firstInterrupt  = ctx.Done()
		secondInterrupt chan os.Signal // initially nil
	)
	for iterator.Seek(m.startFrom); iterator.Valid(); iterator.Next() {
		select {
		case <-firstInterrupt:
			if errors.Is(ctx.Err(), context.Canceled) {
				msg := "WARNING: Migration is in progress, but you tried to interrupt it.\n" +
					"Database may be in an inconsistent state.\n" +
					"To force cancellation and potentially corrupt data, send interrupt signal again.\n" +
					"Otherwise, please allow the migration to complete."
				log.Warnw(msg)

				// after context canceled on upper level there is no way to check how many interrupts were made from ctx.Done()
				// but we can Initialise additional channel to receive the signals, they will be copied by runtime and provided
				// to all callers (i.e. here and on upper level)
				secondInterrupt = make(chan os.Signal, 1)
				signal.Notify(secondInterrupt, os.Interrupt, syscall.SIGTERM)
				// if we don't set firstInterrupt to nil this case may be fired all the time because
				// execution order of cases in select is not guaranteed and selecting from nil channel is blocked operation
				firstInterrupt = nil
			}
		case <-secondInterrupt:
			err := errors.New("migration interrupt")
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		default:
			// keep going
		}

		key := iterator.Key()
		if !bytes.HasPrefix(key, m.target.Key()) {
			break
		}

		if pass, err := m.keyFilter(key); err != nil {
			return nil, utils.RunAndWrapOnError(iterator.Close, err)
		} else if pass {
			if remainingInBatch == 0 {
				m.startFrom = key
				return nil, utils.RunAndWrapOnError(iterator.Close, ErrCallWithNewTransaction)
			}

			remainingInBatch--
			value, err := iterator.Value()
			if err != nil {
				return nil, utils.RunAndWrapOnError(iterator.Close, err)
			}

			if err = m.do(database, key, value, network); err != nil {
				return nil, utils.RunAndWrapOnError(iterator.Close, err)
			}
		}
	}
	signal.Stop(secondInterrupt)

	return nil, iterator.Close()
}
