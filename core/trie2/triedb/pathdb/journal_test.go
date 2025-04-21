package pathdb

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestJournal(t *testing.T) {
	testCases := []struct {
		name     string
		numDiffs int
	}{
		{"disk only", 0},
		{"5 diffs", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDB := memory.New()
			db, err := New(testDB, nil)
			require.NoError(t, err)

			tree, tracker := setupLayerTree(tc.numDiffs, 20)
			db.tree = tree

			// Use the root from the disk layer
			root := *new(felt.Felt).SetUint64(uint64(tc.numDiffs))
			require.NoError(t, db.Journal(root))

			_, err = New(testDB, nil)
			require.NoError(t, err)

			for i := 0; i <= tc.numDiffs; i++ {
				root := *new(felt.Felt).SetUint64(uint64(i))
				err := verifyLayer(tree, root, tracker)
				require.NoError(t, err)
			}
		})
	}
}
