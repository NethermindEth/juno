package pathdb

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

func TestCommit(t *testing.T) {
	numNodes := 50
	testCases := []struct {
		name     string
		numDiffs int
	}{
		{"0 diffs", 0},
		{"50 diffs", 50},
		{"129 diffs", 129},
		{"500 diffs", 500},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testDB := memory.New()
			pathDB, err := New(testDB, nil)
			if err != nil {
				t.Fatal(err)
			}

			parent := &felt.StateRootHash{}
			tracker := newLayerTracker()
			for i := 1; i <= tc.numDiffs; i++ {
				root := felt.NewFromUint64[felt.StateRootHash](uint64(i))
				classNodes := createTestNodeSet(numNodes, i, tc.numDiffs, true)
				contractNodes := createTestNodeSet(numNodes, i, tc.numDiffs, false)
				require.NoError(t, pathDB.Update(root, parent, uint64(i), classNodes, contractNodes, nil))

				flatClass, _ := classNodes.Flatten()
				flatContract, flatStorage := contractNodes.Flatten()
				tracker.trackNodes(root, parent, flatClass, flatContract, flatStorage)

				parent = root
			}

			require.NoError(t, pathDB.Commit(parent))

			require.Equal(t, 1, pathDB.tree.len())
			require.Equal(t, parent, pathDB.tree.diskLayer().rootHash())

			require.NoError(t, verifyLayer(pathDB.tree, parent, tracker))
		})
	}
}
