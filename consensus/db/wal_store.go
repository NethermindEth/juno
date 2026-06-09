package db

import (
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	kvdb "github.com/NethermindEth/juno/db"
	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/vfs"
	pebblewal "github.com/cockroachdb/pebble/v2/wal"
)

const (
	initialWALNum              = 1
	initialSeqNum              = 1
	cleanupPruneRecordInterval = 256 // Amortizes crash-safe cleanup fsyncs.
)

// TendermintWALStore persists consensus WAL entries so the node can recover after a crash.
type TendermintWALStore[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Flush writes pending records to the WAL.
	Flush() error
	// LoadAllEntries returns stored WAL entries ordered by height.
	LoadAllEntries() iter.Seq2[wal.Entry[V, H, A], error]
	// SetWALEntry buffers an entry until Flush or Close.
	SetWALEntry(entry wal.Entry[V, H, A]) error
	// DeleteWALEntries buffers a prune for all entries up to and including height.
	DeleteWALEntries(height types.Height) error
	// Close flushes pending records and closes WAL resources.
	Close() error
}

type tendermintWALStore[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	mu     sync.Mutex
	closed bool
	wal    *walWriter

	nextBatchSeqNum uint64
	entriesByHeight map[types.Height][]wal.Entry[V, H, A]
	// Height deletes need to find all WAL files that contain the deleted height.
	walFilesByHeight map[types.Height]walNumSet
	// WAL cleanup needs to know whether any remaining height still references the file.
	walHeightRefs map[pebblewal.NumWAL]int

	// prunedUpToHeight is the inclusive watermark for discarded WAL recovery data.
	prunedUpToHeight types.Height

	// pendingRecords are only visible through LoadAllEntries after a successful flush.
	pendingRecords           []walRecordEnvelope[V, H, A]
	encodedBatch             []byte
	pruneRecordsSinceCleanup uint64
}

func DefaultWALDir(dbPath string) string {
	const walDirName = "consensus-wal"

	return filepath.Join(dbPath, walDirName)
}

// NewTendermintWALStore creates a new TendermintWALStore
// backed by Pebble's standalone WAL implementation.
func NewTendermintWALStore[V types.Hashable[H], H types.Hash, A types.Addr](
	database kvdb.KeyValueStore,
) (TendermintWALStore[V, H, A], error) {
	const walDirPerm = 0o755

	// Store the standalone consensus WAL next to the backing DB to keep all local
	// consensus persistence under the same data path.
	dbPath := database.Path()
	if dbPath == "" {
		return nil, errors.New(
			"NewTendermintWALStore: consensus WAL requires a local path; remote DBs are unsupported",
		)
	}
	walDir := DefaultWALDir(dbPath)
	if err := vfs.Default.MkdirAll(walDir, walDirPerm); err != nil {
		return nil, fmt.Errorf("NewTendermintWALStore: create WAL directory: %w", err)
	}

	dir := pebblewal.Dir{
		FS:      vfs.Default,
		Dirname: walDir,
	}

	logs, err := pebblewal.Scan(dir)
	if err != nil {
		return nil, fmt.Errorf("NewTendermintWALStore: scan WAL directory: %w", err)
	}
	if err := recoverLatestWALTail(logs); err != nil {
		return nil, fmt.Errorf("NewTendermintWALStore: recover latest WAL tail: %w", err)
	}
	prunedUpToHeight, err := loadPruneWatermark(walDir)
	if err != nil {
		return nil, fmt.Errorf("NewTendermintWALStore: load prune watermark: %w", err)
	}

	manager, err := pebblewal.Init(pebblewal.Options{
		Primary:            dir,
		MinUnflushedWALNum: initialWALNum,
		Logger:             pebble.DefaultLogger,
		EventListener:      noopEventListener{},
		// No prealloc: batches are small and short-lived.
		PreallocateSize: func() int { return 0 },
		// Consensus requires durability per batch.
		MinSyncInterval: func() time.Duration { return 0 },
		// We do not use Pebble's flush pipeline.
		WriteWALSyncOffsets: func() bool { return false },
	}, logs)
	if err != nil {
		return nil, fmt.Errorf("NewTendermintWALStore: initialize WAL manager: %w", err)
	}

	walStore := &tendermintWALStore[V, H, A]{
		wal:              newWALWriter(manager, walDir, nextWALNum(logs)),
		nextBatchSeqNum:  initialSeqNum,
		entriesByHeight:  make(map[types.Height][]wal.Entry[V, H, A]),
		walFilesByHeight: make(map[types.Height]walNumSet),
		walHeightRefs:    make(map[pebblewal.NumWAL]int),
		prunedUpToHeight: prunedUpToHeight,
	}

	if err := walStore.loadExistingEntries(logs); err != nil {
		return nil, errors.Join(err, manager.Close())
	}

	return walStore, nil
}

func (s *tendermintWALStore[V, H, A]) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.flushLocked()
}

func (s *tendermintWALStore[V, H, A]) flushLocked() error {
	if s.closed {
		return errors.New("Flush: WAL is closed")
	}

	if len(s.pendingRecords) == 0 {
		return nil
	}

	encodedBatch, err := encodeBatch(s.pendingRecords, s.nextBatchSeqNum, s.encodedBatch)
	if err != nil {
		return err
	}
	recordCount := uint32(len(s.pendingRecords))
	appendResult, err := s.wal.appendSync(encodedBatch)
	const maxReusableEncodedBatchCap = 512 << 10 // Sized for current mainnet encoded-batch workloads.
	if cap(encodedBatch) <= maxReusableEncodedBatchCap {
		s.encodedBatch = encodedBatch[:0]
	} else {
		s.encodedBatch = nil
	}
	if err != nil {
		if !appendResult.committed {
			return err
		}
	}
	walNum := appendResult.walNum

	committedErr := err
	records := s.pendingRecords

	s.nextBatchSeqNum += uint64(recordCount)
	s.updateIndexesFromCommittedRecords(walNum, records)

	pruneRecordCount := countPruneRecords(records)
	committedErr = errors.Join(committedErr, s.removeObsoleteWALFiles(pruneRecordCount))

	clear(records)
	s.pendingRecords = records[:0]
	return committedErr
}

func countPruneRecords[V types.Hashable[H], H types.Hash, A types.Addr](
	records []walRecordEnvelope[V, H, A],
) uint64 {
	var pruneRecordCount uint64
	for _, record := range records {
		if record.Kind == walRecordPruneUpToHeight {
			pruneRecordCount++
		}
	}
	return pruneRecordCount
}

func (s *tendermintWALStore[V, H, A]) removeObsoleteWALFiles(
	pruneRecordCount uint64,
) error {
	if pruneRecordCount == 0 {
		return nil
	}

	s.pruneRecordsSinceCleanup += pruneRecordCount
	if s.pruneRecordsSinceCleanup < cleanupPruneRecordInterval {
		return nil
	}

	if err := writePruneWatermark(s.wal.dir, s.prunedUpToHeight); err != nil {
		return err
	}

	// Future optimisation: run cleanup in a background worker, piggyback prune
	// durability on the next WAL flush instead of the driver's per-height Flush.
	rotateErr := s.wal.rotateAfterSynced()
	cleanupErr := s.cleanupObsoleteWALs()
	if rotateErr == nil && cleanupErr == nil {
		s.pruneRecordsSinceCleanup = 0
	}
	return errors.Join(rotateErr, cleanupErr)
}

func (s *tendermintWALStore[V, H, A]) cleanupObsoleteWALs() error {
	minLiveWALNum := s.wal.minLiveWALNum()
	updateMinLiveWALNum := func(walRefs map[pebblewal.NumWAL]int) {
		for walNum := range walRefs {
			if walNum < minLiveWALNum {
				minLiveWALNum = walNum
			}
		}
	}
	updateMinLiveWALNum(s.walHeightRefs)

	toDelete, err := s.wal.obsolete(minLiveWALNum)
	if err != nil {
		return fmt.Errorf("cleanupObsoleteWALs: mark obsolete WALs: %w", err)
	}

	for _, log := range toDelete {
		if err := log.FS.Remove(log.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cleanupObsoleteWALs: remove obsolete WAL %s: %w", log.Path, err)
		}
	}
	return nil
}

func (s *tendermintWALStore[V, H, A]) DeleteWALEntries(height types.Height) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("DeleteWALEntries: WAL is closed")
	}
	if height <= s.prunedUpToHeight {
		return nil
	}

	for i := range s.pendingRecords {
		record := &s.pendingRecords[i]
		if record.Kind == walRecordPruneUpToHeight {
			record.Height = max(record.Height, height)
			return nil
		}
	}

	s.pendingRecords = append(s.pendingRecords, walRecordEnvelope[V, H, A]{
		Kind:   walRecordPruneUpToHeight,
		Height: height,
	})
	return nil
}

func (s *tendermintWALStore[V, H, A]) SetWALEntry(entry wal.Entry[V, H, A]) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("SetWALEntry: WAL is closed")
	}

	record := walRecordEnvelope[V, H, A]{
		Kind: walRecordEntry,
	}
	if err := record.setEntry(entry); err != nil {
		return err
	}
	if entry.GetHeight() <= s.prunedUpToHeight {
		return nil
	}
	s.pendingRecords = append(s.pendingRecords, record)
	return nil
}

func (s *tendermintWALStore[V, H, A]) LoadAllEntries() iter.Seq2[wal.Entry[V, H, A], error] {
	s.mu.Lock()
	entrySnapshot := make(map[types.Height][]wal.Entry[V, H, A], len(s.entriesByHeight))
	heights := make([]types.Height, 0, len(s.entriesByHeight))
	for height, entries := range s.entriesByHeight {
		heights = append(heights, height)
		entrySnapshot[height] = append([]wal.Entry[V, H, A](nil), entries...)
	}
	s.mu.Unlock()
	slices.Sort(heights)

	return func(yield func(wal.Entry[V, H, A], error) bool) {
		for _, height := range heights {
			for _, entry := range entrySnapshot[height] {
				if !yield(entry, nil) {
					return
				}
			}
		}
	}
}

func (s *tendermintWALStore[V, H, A]) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}

	flushErr := s.flushLocked()
	s.closed = true
	closeErr := s.wal.close()
	return errors.Join(flushErr, closeErr, s.wal.closeManager())
}
