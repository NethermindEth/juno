package walstore

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/cockroachdb/pebble/v2/record"
	pebblewal "github.com/cockroachdb/pebble/v2/wal"
)

type walAppendResult struct {
	walNum    pebblewal.NumWAL
	committed bool
}

type noopEventListener struct{}

func (noopEventListener) LogCreated(pebblewal.CreateInfo) {}

type walWriter struct {
	manager pebblewal.Manager
	dir     string

	nextWALNum pebblewal.NumWAL

	writer        pebblewal.Writer
	currentWALNum pebblewal.NumWAL

	currentWALSyncedOffset int64
	repairRequired         bool
}

func newWALWriter(manager pebblewal.Manager, dir string, nextWALNum pebblewal.NumWAL) *walWriter {
	return &walWriter{
		manager:    manager,
		dir:        dir,
		nextWALNum: nextWALNum,
	}
}

func nextWALNum(logs pebblewal.Logs) pebblewal.NumWAL {
	next := pebblewal.NumWAL(initialWALNum)
	for _, log := range logs {
		if candidate := log.Num + 1; candidate > next {
			next = candidate
		}
	}
	return next
}

func (w *walWriter) appendSync(encodedBatch []byte) (walAppendResult, error) {
	walNum, writer, err := w.ensureWriter()
	if err != nil {
		return walAppendResult{}, err
	}

	var (
		waitGroup sync.WaitGroup
		syncErr   error
	)
	waitGroup.Add(1)

	logicalOffset, err := writer.WriteRecord(encodedBatch, pebblewal.SyncOptions{
		Done: &waitGroup,
		Err:  &syncErr,
	}, nil)
	if err != nil {
		abortErr := w.abortUncommitted()
		return walAppendResult{}, errors.Join(fmt.Errorf("Flush: write WAL record: %w", err), abortErr)
	}

	waitGroup.Wait()
	if syncErr != nil {
		abortErr := w.abortUncommitted()
		return walAppendResult{}, errors.Join(fmt.Errorf("Flush: sync WAL record: %w", syncErr), abortErr)
	}

	w.currentWALSyncedOffset = logicalOffset
	return walAppendResult{walNum: walNum, committed: true}, nil
}

func (w *walWriter) rotateAfterSynced() error {
	return w.closeAndRepairCurrent(w.currentWALSyncedOffset, false)
}

func (w *walWriter) abortUncommitted() error {
	// Failed writes/syncs may leave trailing WAL bytes, so always repair.
	return w.closeAndRepairCurrent(w.currentWALSyncedOffset, true)
}

func (w *walWriter) close() error {
	return w.closeAndRepairCurrent(w.currentWALSyncedOffset, false)
}

func (w *walWriter) ensureWriter() (pebblewal.NumWAL, pebblewal.Writer, error) {
	if w.repairRequired {
		return 0, nil, errors.New(
			"Flush: previous WAL tail repair failed; refusing to create a new WAL writer",
		)
	}
	if w.writer != nil {
		return w.currentWALNum, w.writer, nil
	}

	writer, err := w.manager.Create(w.nextWALNum, 0)
	if err != nil {
		return 0, nil, fmt.Errorf("Flush: create WAL writer: %w", err)
	}

	w.currentWALNum = w.nextWALNum
	w.nextWALNum++
	w.writer = writer
	w.currentWALSyncedOffset = 0
	return w.currentWALNum, w.writer, nil
}

func (w *walWriter) closeAndRepairCurrent(repairOffset int64, forceRepair bool) error {
	if w.writer == nil {
		return nil
	}

	walNum := w.currentWALNum
	closeErr := w.closeCurrent()

	if closeErr == nil && !forceRepair {
		w.repairRequired = false
		return nil
	}

	walPath := filepath.Join(w.dir, walNum.String()+".log")
	repairErr := repairWALTail(walPath, repairOffset)
	if repairErr != nil {
		w.repairRequired = true
		return errors.Join(closeErr, repairErr)
	}
	w.repairRequired = false
	return closeErr
}

func (w *walWriter) closeCurrent() error {
	if w.writer == nil {
		return nil
	}

	_, err := w.writer.Close()
	w.writer = nil
	w.currentWALNum = 0
	w.currentWALSyncedOffset = 0
	if err != nil {
		return fmt.Errorf("Flush: close WAL writer: %w", err)
	}
	return nil
}

func (w *walWriter) minLiveWALNum() pebblewal.NumWAL {
	minLiveWALNum := w.nextWALNum
	if w.writer != nil && w.currentWALNum < minLiveWALNum {
		minLiveWALNum = w.currentWALNum
	}
	return minLiveWALNum
}

func (w *walWriter) obsolete(minLiveWALNum pebblewal.NumWAL) ([]pebblewal.DeletableLog, error) {
	if w.manager == nil {
		return nil, nil
	}
	return w.manager.Obsolete(minLiveWALNum, true)
}

func (w *walWriter) closeManager() error {
	if w.manager == nil {
		return nil
	}
	return w.manager.Close()
}

func repairWALTail(walPath string, syncedOffset int64) (err error) {
	file, openErr := os.OpenFile(walPath, os.O_RDWR, 0)
	if errors.Is(openErr, os.ErrNotExist) {
		return nil
	}
	if openErr != nil {
		return fmt.Errorf("Flush: open WAL for repair: %w", openErr)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("Flush: close WAL after repair: %w", closeErr))
		}
	}()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("Flush: stat WAL for repair: %w", err)
	}
	if syncedOffset > info.Size() {
		syncedOffset = info.Size()
	}

	if err := file.Truncate(syncedOffset); err != nil {
		return fmt.Errorf("Flush: truncate WAL to last synced offset: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("Flush: sync truncated WAL: %w", err)
	}
	return nil
}

// Scan the latest WAL and truncate any bytes after the last valid record.
func recoverLatestWALTail(logs pebblewal.Logs) error {
	if len(logs) == 0 {
		return nil
	}

	latest := logs[len(logs)-1]
	reader := latest.OpenForRead()
	defer reader.Close()

	for {
		_, offset, err := reader.NextRecord()
		switch {
		case err == nil:
			continue
		case errors.Is(err, os.ErrNotExist):
			return nil
		case errors.Is(err, io.EOF):
			return repairWALTailIfLonger(offset.PhysicalFile, offset.Physical)
		case record.IsInvalidRecord(err):
			return repairWALTail(offset.PhysicalFile, offset.Physical)
		default:
			return fmt.Errorf("recoverLatestWALTail: read WAL %s: %w", latest.Num, err)
		}
	}
}

// EOF can still leave trailing garbage beyond the last valid record offset.
func repairWALTailIfLonger(walPath string, syncedOffset int64) error {
	if walPath == "" {
		return nil
	}
	info, err := os.Stat(walPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("Flush: stat WAL for repair: %w", err)
	}
	if info.Size() <= syncedOffset {
		return nil
	}
	return repairWALTail(walPath, syncedOffset)
}
