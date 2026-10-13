package historyprunner

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/db"
)

type layout struct {
	id                 byte
	name               string
	buckets            []db.Bucket
	keepsLatestAtFloor bool
}

var (
	// deprecatedLayout entries record the value before their block, and a
	// read at h takes the first entry above h, falling back to the head state.
	deprecatedLayout = layout{
		id:   0,
		name: "deprecated",
		//nolint:staticcheck,nolintlint // the old state layout is still pruned
		buckets: []db.Bucket{
			db.DeprecatedContractStorageHistory,
			db.DeprecatedContractNonceHistory,
			db.DeprecatedContractClassHashHistory,
		},
	}

	// newStateLayout entries record the value after their block, and a read
	// at h takes the latest entry at or below h.
	newStateLayout = layout{
		id:   1,
		name: "new-state",
		buckets: []db.Bucket{
			db.ContractStorageHistory,
			db.ContractNonceHistory,
			db.ContractClassHashHistory,
		},
		keepsLatestAtFloor: true,
	}
)

func layoutByID(id byte) (*layout, error) {
	for _, l := range []*layout{&deprecatedLayout, &newStateLayout} {
		if l.id == id {
			return l, nil
		}
	}
	return nil, fmt.Errorf("unknown history layout %d", id)
}

// detectLayout reports the layout of the history on disk. The new-state
// migration converts all of it, so a database holding both is mid-migration
// and can't be pruned until it finishes. Staged scratch entries count too:
// after the history wipe they are all a resumed run can see.
func detectLayout(r db.KeyValueReader) (*layout, error) {
	deprecated, err := deprecatedLayout.hasEntries(r)
	if err != nil {
		return nil, err
	}
	newState, err := newStateLayout.hasEntries(r)
	if err != nil {
		return nil, err
	}
	if deprecated && newState {
		return nil, errors.New("history is partly migrated to the new state; " +
			"finish the --new-state migration before enabling --prune-mode")
	}
	if newState {
		return &newStateLayout, nil
	}
	return &deprecatedLayout, nil
}

// hasEntries reports whether any of the layout's buckets, or their scratch
// copies, has a key.
func (l *layout) hasEntries(r db.KeyValueReader) (bool, error) {
	for _, bucket := range l.buckets {
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

// bucketsFrom returns the layout's buckets from the one holding cursor on;
// all of them when cursor is nil. bucketAt is the cursor byte naming its bucket.
func (l *layout) bucketsFrom(cursor []byte, bucketAt int) ([]db.Bucket, error) {
	if cursor == nil {
		return l.buckets, nil
	}
	for i, b := range l.buckets {
		if byte(b) == cursor[bucketAt] {
			return l.buckets[i:], nil
		}
	}
	return nil, fmt.Errorf("cursor %x is not in the %s history", cursor, l.name)
}
