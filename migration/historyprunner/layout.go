package historyprunner

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/db"
)

// layout is how the state history is stored. Both layouts key an entry as
// [bucket][history key][block] and differ only in where a history key's kept
// entries start, see [layout.seekStart].
type layout byte

const (
	// deprecatedLayout entries record the value before their block, and a
	// read at h takes the first entry above h, falling back to the head state.
	deprecatedLayout layout = iota
	// newStateLayout entries record the value after their block, and a read
	// at h takes the latest entry at or below h.
	newStateLayout
)

func (l layout) String() string {
	if l == newStateLayout {
		return "new-state"
	}
	return "deprecated"
}

func (l layout) buckets() []db.Bucket {
	if l == newStateLayout {
		return []db.Bucket{
			db.ContractStorageHistory,
			db.ContractNonceHistory,
			db.ContractClassHashHistory,
		}
	}
	//nolint:staticcheck,nolintlint // the old state layout is still pruned
	return []db.Bucket{
		db.DeprecatedContractStorageHistory,
		db.DeprecatedContractNonceHistory,
		db.DeprecatedContractClassHashHistory,
	}
}

// nextsBeforeSeek is how many entries seekStart steps over before it seeks:
// a Seek costs several Nexts, so short histories are cheaper to walk.
const nextsBeforeSeek = 8

// seekStart moves it from a history key's first entry, below the floor, to
// the key's first entry to keep: the first at or above the floor, and for
// new-state the latest below it, which reads at floor-1 resolve to.
func (l layout) seekStart(it db.Iterator, historyKey []byte, floor uint64) bool {
	ok := it.Next()
	for i := 1; ok && belowFloor(it.Key(), historyKey, floor); i++ {
		if i == nextsBeforeSeek {
			ok = it.Seek(binary.BigEndian.AppendUint64(historyKey, floor))
			break
		}
		ok = it.Next()
	}
	if l == newStateLayout {
		return it.Prev()
	}
	return ok
}

// belowFloor reports whether key is an entry of historyKey below floor.
func belowFloor(key, historyKey []byte, floor uint64) bool {
	return bytes.HasPrefix(key, historyKey) && binary.BigEndian.Uint64(key[len(historyKey):]) < floor
}

// detectLayout reports the layout of the history on disk. The new-state
// migration converts all of it, so a database holding both is mid-migration
// and can't be pruned until it finishes. Staged scratch entries count too:
// after the history wipe they are all a resumed run can see.
func detectLayout(r db.KeyValueReader) (layout, error) {
	deprecated, err := deprecatedLayout.hasEntries(r)
	if err != nil {
		return 0, err
	}
	newState, err := newStateLayout.hasEntries(r)
	if err != nil {
		return 0, err
	}
	if deprecated && newState {
		return 0, errors.New("history is partly migrated to the new state; " +
			"finish the --new-state migration before enabling --prune-mode")
	}
	if newState {
		return newStateLayout, nil
	}
	return deprecatedLayout, nil
}

// hasEntries reports whether any of the layout's buckets, or their scratch
// copies, has a key.
func (l layout) hasEntries(r db.KeyValueReader) (bool, error) {
	for _, bucket := range l.buckets() {
		for _, prefix := range [][]byte{bucket.Key(), scratchPrefix(bucket)} {
			it, err := r.NewIterator(prefix, true)
			if err != nil {
				return false, err
			}
			found := it.First()
			if err := it.Close(); err != nil {
				return false, err
			}
			if found {
				return true, nil
			}
		}
	}
	return false, nil
}

// bucketsFrom returns the layout's buckets from the one holding
// resumeHistoryFrom on; all of them when it is nil. bucketAt is its byte
// naming the bucket.
func (l layout) bucketsFrom(resumeHistoryFrom []byte, bucketAt int) ([]db.Bucket, error) {
	buckets := l.buckets()
	if resumeHistoryFrom == nil {
		return buckets, nil
	}
	for i, b := range buckets {
		if byte(b) == resumeHistoryFrom[bucketAt] {
			return buckets[i:], nil
		}
	}
	return nil, fmt.Errorf("resume key %x is not in the %s history", resumeHistoryFrom, l)
}
