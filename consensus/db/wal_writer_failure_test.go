package db

// These tests assert durability/error-handling behavior, but use package db
// so they can inject WAL writer failures that are not reachable through the
// public DB API.
//
// Invariants covered here:
//   - failed append/sync aborts to the previous committed offset and remains retryable
//   - a committed prune record stays applied even if later rotate/close fails
//   - ambiguous repair state blocks future WAL writers

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	consensuswal "github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/cockroachdb/pebble/v2/record"
	pebblewal "github.com/cockroachdb/pebble/v2/wal"
	"github.com/stretchr/testify/require"
)

func TestFlushKeepsExistingEntriesWhenPruneSyncFails(t *testing.T) {
	const walNum = pebblewal.NumWAL(1)

	walDir := t.TempDir()
	walPath := walPathFor(walDir, walNum)
	syncErr := errors.New("fsync failed")

	walStore := newTestWALStore(
		walDir,
		&fakeWALWriter{path: walPath, syncErr: syncErr},
		walNum,
		0,
	)
	existingEntry := seedExistingEntry(walStore, 1)
	require.NoError(t, walStore.DeleteWALEntries(1))

	err := walStore.Flush()
	require.ErrorIs(t, err, syncErr)
	requireEntries(t, walStore, existingEntry)
}

func TestAbortUncommittedRepairsWALTail(t *testing.T) {
	const walNum = pebblewal.NumWAL(2)

	walDir := t.TempDir()
	walPath := walPathFor(walDir, walNum)
	// Keep a valid committed prefix so repair must truncate only the bad tail.
	committedOffset := appendPhysicalRecord(t, walPath, emptyBatchPayload(t))
	appendPhysicalRecord(t, walPath, []byte("garbage-tail"))
	closeErr := errors.New("close failed")

	walStore := newTestWALStore(
		walDir,
		&fakeWALWriter{path: walPath, closeErr: closeErr},
		walNum,
		committedOffset,
	)

	err := walStore.wal.abortUncommitted()
	require.ErrorIs(t, err, closeErr)
	requireWALSize(t, walPath, committedOffset)
}

func TestFailedRepairBlocksFutureFlushes(t *testing.T) {
	const walNum = pebblewal.NumWAL(3)

	walDir := t.TempDir()
	walPath := walPathFor(walDir, walNum)
	// walPath normally points to a WAL file. Making it a directory forces repair to fail.
	require.NoError(t, os.Mkdir(walPath, 0o755))

	walStore := newTestWALStore(walDir, &fakeWALWriter{path: walPath}, walNum, 0)

	err := walStore.wal.abortUncommitted()
	require.Error(t, err)
	start := consensuswal.Start(1)
	require.NoError(t, walStore.SetWALEntry(&start))
	err = walStore.Flush()
	require.Error(t, err)
	requireEntries(t, walStore)
}

func TestFlushFailureDoesNotExposeEntriesAndCanRetry(t *testing.T) {
	tests := map[string]func(string, error) *fakeWALWriter{
		"write fails": func(walPath string, err error) *fakeWALWriter {
			return &fakeWALWriter{path: walPath, writeErr: err}
		},
		"sync fails": func(walPath string, err error) *fakeWALWriter {
			return &fakeWALWriter{path: walPath, syncErr: err}
		},
	}

	for name, failingWriter := range tests {
		t.Run(name, func(t *testing.T) {
			const walNum = pebblewal.NumWAL(4)

			walDir := t.TempDir()
			walPath := walPathFor(walDir, walNum)

			start := consensuswal.Start(2)
			injectedErr := errors.New(name)
			walStore := newTestWALStore(
				walDir,
				failingWriter(walPath, injectedErr),
				walNum,
				0,
			)
			require.NoError(t, walStore.SetWALEntry(&start))

			err := walStore.Flush()
			require.ErrorIs(t, err, injectedErr)
			requireEntries(t, walStore)

			// Failed flushes close the writer, so install a fresh fake writer for retry.
			walStore.wal.writer = &fakeWALWriter{path: walPath}
			walStore.wal.currentWALNum = walNum
			require.NoError(t, walStore.Flush())
			requireEntries(t, walStore, &start)
		})
	}
}

func TestFlushKeepsEntriesPrunedAfterCloseFailure(t *testing.T) {
	const walNum = pebblewal.NumWAL(5)

	walDir := t.TempDir()
	walPath := walPathFor(walDir, walNum)
	closeErr := errors.New("close failed")
	walStore := newTestWALStore(
		walDir,
		&fakeWALWriter{path: walPath, closeErr: closeErr},
		walNum,
		0,
	)
	seedExistingEntry(walStore, 1)
	// Force cleanup, which closes the writer after the prune record is committed.
	walStore.pruneRecordsSinceCleanup = cleanupPruneRecordInterval - 1
	require.NoError(t, walStore.DeleteWALEntries(1))

	err := walStore.Flush()
	require.ErrorIs(t, err, closeErr)
	requireEntries(t, walStore)

	require.NoError(t, walStore.Flush())
	requireEntries(t, walStore)
}

type (
	testWALStore = tendermintWALStore[starknet.Value, starknet.Hash, starknet.Address]
	testWALEntry = consensuswal.Entry[starknet.Value, starknet.Hash, starknet.Address]
)

type fakeWALWriter struct {
	path     string
	writeErr error
	syncErr  error
	closeErr error
}

var _ pebblewal.Writer = (*fakeWALWriter)(nil)

func newTestWALStore(
	walDir string,
	writer pebblewal.Writer,
	walNum pebblewal.NumWAL,
	syncedOffset int64,
) *testWALStore {
	wal := newWALWriter(nil, walDir, walNum+1)
	wal.writer = writer
	wal.currentWALNum = walNum
	wal.currentWALSyncedOffset = syncedOffset

	return &tendermintWALStore[starknet.Value, starknet.Hash, starknet.Address]{
		wal:              wal,
		nextBatchSeqNum:  initialSeqNum,
		entriesByHeight:  make(map[types.Height][]testWALEntry),
		walFilesByHeight: make(map[types.Height]walNumSet),
		walHeightRefs:    make(map[pebblewal.NumWAL]int),
	}
}

func (w *fakeWALWriter) WriteRecord(
	p []byte,
	opts pebblewal.SyncOptions,
	_ pebblewal.RefCount,
) (int64, error) {
	if w.writeErr != nil {
		return 0, w.writeErr
	}

	offset, err := appendPhysicalWALRecord(w.path, p)
	if err != nil {
		return 0, err
	}

	if opts.Err != nil {
		*opts.Err = w.syncErr
	}

	if opts.Done != nil {
		opts.Done.Done()
	}
	return offset, nil
}

func (w *fakeWALWriter) Close() (int64, error) {
	return 0, w.closeErr
}

func (w *fakeWALWriter) Metrics() record.LogWriterMetrics {
	return record.LogWriterMetrics{}
}

func walPathFor(walDir string, walNum pebblewal.NumWAL) string {
	return filepath.Join(walDir, walNum.String()+".log")
}

func appendPhysicalWALRecord(path string, payload []byte) (int64, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return 0, err
	}

	writer := record.NewWriter(file)
	offset, err := writer.WriteRecord(payload)
	if closeErr := writer.Close(); closeErr != nil {
		err = errors.Join(err, closeErr)
	}
	if syncErr := file.Sync(); syncErr != nil {
		err = errors.Join(err, syncErr)
	}
	return offset, err
}

func appendPhysicalRecord(t *testing.T, path string, payload []byte) int64 {
	t.Helper()

	offset, err := appendPhysicalWALRecord(path, payload)
	require.NoError(t, err)
	return offset
}

func emptyBatchPayload(t *testing.T) []byte {
	t.Helper()

	payload, err := encodeBatch[starknet.Value, starknet.Hash, starknet.Address](
		nil, initialSeqNum, nil,
	)
	require.NoError(t, err)
	return payload
}

func requireWALSize(t *testing.T, walPath string, size int64) {
	t.Helper()

	info, err := os.Stat(walPath)
	require.NoError(t, err)
	require.Equal(t, size, info.Size())
}

func loadAllEntries(
	t *testing.T,
	db *testWALStore,
) []testWALEntry {
	t.Helper()

	var entries []testWALEntry
	for entry, err := range db.LoadAllEntries() {
		require.NoError(t, err)
		entries = append(entries, entry)
	}
	return entries
}

func requireEntries(
	t *testing.T,
	db *testWALStore,
	want ...testWALEntry,
) {
	t.Helper()
	require.Equal(t, want, loadAllEntries(t, db))
}

func seedExistingEntry(
	db *testWALStore,
	height types.Height,
) testWALEntry {
	start := consensuswal.Start(height)
	db.entriesByHeight[height] = []testWALEntry{&start}
	return &start
}
