package walstore

import (
	"slices"

	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
	pebblewal "github.com/cockroachdb/pebble/v2/wal"
)

// walNumSet stores the first WAL inline and subsequent WALs in rest.
// It avoids map overhead because most heights reference only one WAL.
type walNumSet struct {
	first pebblewal.NumWAL
	rest  []pebblewal.NumWAL
}

func (s *walNumSet) addIfMissing(walNum pebblewal.NumWAL) bool {
	if s.first == 0 {
		s.first = walNum
		return true
	}
	if s.first == walNum || slices.Contains(s.rest, walNum) {
		return false
	}
	s.rest = append(s.rest, walNum)
	return true
}

func (s walNumSet) rangeOver(fn func(pebblewal.NumWAL)) {
	if s.first == 0 {
		return
	}
	fn(s.first)
	for _, walNum := range s.rest {
		fn(walNum)
	}
}

func (s *tendermintWALStore[V, H, A]) updateIndexesFromCommittedRecords(
	walNum pebblewal.NumWAL,
	records []walRecordEnvelope[V, H, A],
) {
	for _, record := range records {
		switch record.Kind {
		case walRecordEntry:
			entry, err := record.entry()
			if err != nil {
				panic(err)
			}
			if entry.GetHeight() <= s.prunedUpToHeight {
				continue
			}
			s.addLiveEntry(walNum, entry)
		case walRecordPruneUpToHeight:
			s.pruneLiveEntriesUpTo(record.Height)
		}
	}
}

func (s *tendermintWALStore[V, H, A]) addLiveEntry(
	walNum pebblewal.NumWAL,
	entry wal.Entry[V, H, A],
) {
	height := entry.GetHeight()
	s.entriesByHeight[height] = append(s.entriesByHeight[height], entry)

	referencedWALs := s.walFilesByHeight[height]
	if referencedWALs.addIfMissing(walNum) {
		s.walFilesByHeight[height] = referencedWALs
		s.walHeightRefs[walNum]++
	}
}

func (s *tendermintWALStore[V, H, A]) deleteLiveHeight(height types.Height) {
	delete(s.entriesByHeight, height)

	referencedWALs := s.walFilesByHeight[height]
	delete(s.walFilesByHeight, height)
	referencedWALs.rangeOver(func(walNum pebblewal.NumWAL) {
		s.walHeightRefs[walNum]--
		if s.walHeightRefs[walNum] == 0 {
			delete(s.walHeightRefs, walNum)
		}
	})
}

func (s *tendermintWALStore[V, H, A]) pruneLiveEntriesUpTo(height types.Height) {
	if height <= s.prunedUpToHeight {
		return
	}
	s.prunedUpToHeight = height
	for liveHeight := range s.entriesByHeight {
		if liveHeight <= height {
			s.deleteLiveHeight(liveHeight)
		}
	}
}
