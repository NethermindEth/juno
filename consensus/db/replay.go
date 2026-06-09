package db

import (
	"errors"
	"fmt"
	"io"

	"github.com/cockroachdb/pebble/v2"
	"github.com/cockroachdb/pebble/v2/batchrepr"
	"github.com/cockroachdb/pebble/v2/record"
	pebblewal "github.com/cockroachdb/pebble/v2/wal"
)

func (s *tendermintWALStore[V, H, A]) loadExistingEntries(logs pebblewal.Logs) error {
	for i, log := range logs {
		if err := s.loadLogicalLog(log, i == len(logs)-1); err != nil {
			return err
		}
	}

	return nil
}

func (s *tendermintWALStore[V, H, A]) loadLogicalLog(
	log pebblewal.LogicalLog,
	tolerateTail bool,
) error {
	reader := log.OpenForRead()
	defer reader.Close()

	for {
		recordReader, _, err := reader.NextRecord()
		switch {
		case err == nil:
		case errors.Is(err, io.EOF):
			return nil
		case tolerateTail && record.IsInvalidRecord(err):
			return nil
		default:
			return fmt.Errorf("loadLogicalLog: read WAL %s: %w", log.Num, err)
		}

		encodedBatch, err := io.ReadAll(recordReader)
		if err != nil {
			return fmt.Errorf("loadLogicalLog: read WAL %s record: %w", log.Num, err)
		}

		if err := s.applyEncodedBatch(log.Num, encodedBatch); err != nil {
			return fmt.Errorf("loadLogicalLog: apply WAL %s record: %w", log.Num, err)
		}
	}
}

func (s *tendermintWALStore[V, H, A]) applyEncodedBatch(
	walNum pebblewal.NumWAL,
	encodedBatch []byte,
) error {
	header, ok := batchrepr.ReadHeader(encodedBatch)
	if !ok {
		return errors.New("applyEncodedBatch: missing batch header")
	}
	if nextSeq := uint64(header.SeqNum) + uint64(header.Count); nextSeq > s.nextBatchSeqNum {
		s.nextBatchSeqNum = nextSeq
	}

	reader := batchrepr.Read(encodedBatch)
	seen := uint32(0)
	for {
		kind, _, value, ok, err := reader.Next()
		if err != nil {
			return fmt.Errorf("applyEncodedBatch: iterate batch record: %w", err)
		}
		if !ok {
			if seen != header.Count {
				return fmt.Errorf(
					"applyEncodedBatch: batch header count %d does not match record count %d",
					header.Count,
					seen,
				)
			}
			return nil
		}
		if kind != pebble.InternalKeyKindSet {
			return fmt.Errorf("applyEncodedBatch: unexpected batch key kind %v", kind)
		}

		if err := s.applyEncodedRecord(walNum, value); err != nil {
			return err
		}
		seen++
	}
}

func (s *tendermintWALStore[V, H, A]) applyEncodedRecord(
	walNum pebblewal.NumWAL,
	value []byte,
) error {
	envelope, err := decodeWALRecord[V, H, A](value)
	if err != nil {
		return fmt.Errorf("applyEncodedRecord: decode WAL envelope: %w", err)
	}

	switch envelope.Kind {
	case walRecordEntry:
		entry, err := envelope.entry()
		if err != nil {
			return fmt.Errorf("applyEncodedRecord: %w", err)
		}
		if entry.GetHeight() <= s.prunedUpToHeight {
			return nil
		}
		s.addLiveEntry(walNum, entry)
	case walRecordPruneUpToHeight:
		s.pruneLiveEntriesUpTo(envelope.Height)
	default:
		return fmt.Errorf("applyEncodedRecord: unknown WAL envelope kind %d", envelope.Kind)
	}

	return nil
}
