package pebble

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

const (
	// minCache is the minimum amount of memory in megabytes to allocate to pebble read and write caching.
	minCache = 8

	megabyte = 1 << 20
)

var _ db.DB = (*DB)(nil)

type DB struct {
	pebble *pebble.DB
	wMutex *sync.Mutex

	listener db.EventListener

	// event-derrived stats
	activeComp          int           // Current number of active compactions
	compStartTime       time.Time     // The start time of the earliest currently-active compaction
	compTime            atomic.Int64  // Total time spent in compaction
	level0Comp          atomic.Uint32 // Total number of level-zero compactions
	nonLevel0Comp       atomic.Uint32 // Total number of non level-zero compactions
	writeDelayStartTime time.Time     // The start time of the latest write stall
	writeDelayCount     atomic.Uint64 // Total number of write stall counts
	writeDelayTime      atomic.Int64  // Total time spent in write stalls
}

// New opens a new database at the given path
func New(path string, cache uint, maxOpenFiles int, logger pebble.Logger) (db.DB, error) {
	// Ensure that the specified cache size meets a minimum threshold.
	if cache < minCache {
		cache = minCache
	}
	pDB, err := newPebble(path, &pebble.Options{
		Logger:       logger,
		Cache:        pebble.NewCache(int64(cache * megabyte)),
		MaxOpenFiles: maxOpenFiles,
	})
	if err != nil {
		return nil, err
	}
	return pDB, nil
}

// NewMem opens a new in-memory database
func NewMem() (db.DB, error) {
	return newPebble("", &pebble.Options{
		FS: vfs.NewMem(),
	})
}

// NewMemTest opens a new in-memory database, panics on error
func NewMemTest(t *testing.T) db.DB {
	memDB, err := NewMem()
	if err != nil {
		t.Fatalf("create in-memory db: %v", err)
	}
	t.Cleanup(func() {
		if err := memDB.Close(); err != nil {
			t.Errorf("close in-memory db: %v", err)
		}
	})
	return memDB
}

func newPebble(path string, options *pebble.Options) (*DB, error) {
	var (
		database = &DB{
			wMutex:   new(sync.Mutex),
			listener: &db.SelectiveListener{},
		}
		err error
	)
	// hookup into events
	if options.EventListener == nil {
		options.EventListener = &pebble.EventListener{}
	}
	options.EventListener.CompactionBegin = database.onCompactionBegin
	options.EventListener.CompactionEnd = database.onCompactionEnd
	options.EventListener.WriteStallBegin = database.onWriteStallBegin
	options.EventListener.WriteStallEnd = database.onWriteStallEnd

	database.pebble, err = pebble.Open(path, options)
	if err != nil {
		return nil, err
	}
	return database, nil
}

// WithListener registers an EventListener
func (d *DB) WithListener(listener db.EventListener) db.DB {
	d.listener = listener
	return d
}

// Meter: see db.DB.Meter
func (d *DB) Meter(interval time.Duration) {
	if d.listener == nil {
		return
	}
	go d.meter(interval)
}

// NewTransaction : see db.DB.NewTransaction
func (d *DB) NewTransaction(update bool) (db.Transaction, error) {
	txn := &Transaction{
		listener: d.listener,
	}
	if update {
		d.wMutex.Lock()
		txn.lock = d.wMutex
		txn.batch = d.pebble.NewIndexedBatch()
	} else {
		txn.snapshot = d.pebble.NewSnapshot()
	}

	return txn, nil
}

// Close : see io.Closer.Close
func (d *DB) Close() error {
	return d.pebble.Close()
}

// View : see db.DB.View
func (d *DB) View(fn func(txn db.Transaction) error) error {
	return db.View(d, fn)
}

// Update : see db.DB.Update
func (d *DB) Update(fn func(txn db.Transaction) error) error {
	return db.Update(d, fn)
}

// Impl : see db.DB.Impl
func (d *DB) Impl() any {
	return d.pebble
}
