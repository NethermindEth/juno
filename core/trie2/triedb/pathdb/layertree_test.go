package pathdb

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/require"
)

var r = rand.New(rand.NewSource(42))

func TestLayers(t *testing.T) {
	testCases := []struct {
		name          string
		numDiffs      int
		nodesPerLayer int
	}{
		{"disk only", 0, 20},
		{"1 diff", 1, 20},
		{"5 diffs", 5, 20},
		{"25 diffs", 25, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree, tracker := setupLayerTree(tc.numDiffs, tc.nodesPerLayer, r)

			// Verify all layers
			for i := 0; i <= tc.numDiffs; i++ {
				root := *new(felt.Felt).SetUint64(uint64(i))
				err := verifyLayer(tree, root, tracker)
				require.NoError(t, err)
			}
		})
	}
}

func TestLayersNonExistNode(t *testing.T) {
	testCases := []struct {
		name          string
		numDiffs      int
		nodesPerLayer int
	}{
		{"disk only", 0, 20},
		{"1 diff", 1, 20},
		{"5 diffs", 5, 20},
		{"25 diffs", 25, 10},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree, _ := setupLayerTree(tc.numDiffs, tc.nodesPerLayer, r)

			// Invalid root
			invalidRoot := *new(felt.Felt).SetUint64(uint64(tc.numDiffs + 1))
			layer := tree.get(invalidRoot)
			require.Nil(t, layer)

			validRoot := *new(felt.Felt).SetUint64(uint64(tc.numDiffs))
			layer = tree.get(validRoot)
			require.NotNil(t, layer)
			invalidPath := generateRandomPath(r) // very unlikely we get the same path

			// Invalid class node
			blob, err := layer.node(trieutils.NewClassTrieID(validRoot), felt.Zero, invalidPath, true)
			require.Error(t, err)
			require.Nil(t, blob)

			// Invalid contract node
			blob, err = layer.node(trieutils.NewContractTrieID(validRoot), felt.Zero, invalidPath, false)
			require.Error(t, err)
			require.Nil(t, blob)

			// Invalid contract storage node
			blob, err = layer.node(trieutils.NewContractStorageTrieID(felt.Zero, validRoot), felt.Zero, invalidPath, false)
			require.Error(t, err)
			require.Nil(t, blob)
		})
	}
}

func TestLayersCap(t *testing.T) {
	numDiffs := 25
	nodesPerLayer := 100

	testCases := []struct {
		name      string
		capLayers int
	}{
		{"0 (persist all to disk)", 0},
		{"5 diffs", 5},
		{"10 diffs", 10},
		{"100 diffs (no persist to disk)", 100},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tree, tracker := setupLayerTree(numDiffs, nodesPerLayer, r)
			root := *new(felt.Felt).SetUint64(uint64(numDiffs))
			require.NoError(t, tree.cap(root, tc.capLayers))
			err := verifyLayer(tree, root, tracker)
			require.NoError(t, err)

			require.Equal(t, tree.len(), min(tc.capLayers+1, numDiffs+1))

			exp := max(0, numDiffs-tc.capLayers)
			expDiskHash := *new(felt.Felt).SetUint64(uint64(exp))
			actualDiskHash := tree.diskLayer().rootHash()
			require.Equal(t, expDiskHash, actualDiskHash, fmt.Sprintf("expected disk hash %s, got %s", expDiskHash.String(), actualDiskHash.String()))
		})
	}
}

func TestCap(t *testing.T) {
	nodeCount := 10
	numLayers := 10
	head := *new(felt.Felt).SetUint64(uint64(numLayers - 1))
	tree, _ := setupLayerTree(numLayers, nodeCount, r)
	err := tree.cap(head, 0) // everything in disk layer
	require.NoError(t, err)
}

// mockTrieNode is a simple implementation of trienode.TrieNode for testing
type mockTrieNode struct {
	blob []byte
}

func (m *mockTrieNode) Blob() []byte    { return m.blob }
func (m *mockTrieNode) Hash() felt.Felt { return *new(felt.Felt).SetBytes(m.blob) }
func (m *mockTrieNode) IsLeaf() bool    { return len(m.blob) == 0 }

// layerTracker tracks trie nodes across different layers to simplify testing
type layerTracker struct {
	// Mimic the layer tree by mapping the state root to the nodes
	classNodes           map[felt.Felt]map[trieutils.Path]trienode.TrieNode
	contractNodes        map[felt.Felt]map[trieutils.Path]trienode.TrieNode
	contractStorageNodes map[felt.Felt]map[felt.Felt]map[trieutils.Path]trienode.TrieNode

	// Tracks all unique and latest nodes in the layer tree
	classPaths           map[trieutils.Path]trienode.TrieNode
	contractPaths        map[trieutils.Path]trienode.TrieNode
	contractStoragePaths map[felt.Felt]map[trieutils.Path]trienode.TrieNode

	// Child to parent layer relationship
	childToParent map[felt.Felt]felt.Felt
}

func newLayerTracker() *layerTracker {
	return &layerTracker{
		classNodes:           make(map[felt.Felt]map[trieutils.Path]trienode.TrieNode),
		contractNodes:        make(map[felt.Felt]map[trieutils.Path]trienode.TrieNode),
		contractStorageNodes: make(map[felt.Felt]map[felt.Felt]map[trieutils.Path]trienode.TrieNode),
		childToParent:        make(map[felt.Felt]felt.Felt),
		classPaths:           make(map[trieutils.Path]trienode.TrieNode),
		contractPaths:        make(map[trieutils.Path]trienode.TrieNode),
		contractStoragePaths: make(map[felt.Felt]map[trieutils.Path]trienode.TrieNode),
	}
}

func (t *layerTracker) trackLayer(root, parent felt.Felt) {
	t.childToParent[root] = parent
}

func (t *layerTracker) trackClassNodes(root felt.Felt, nodes map[trieutils.Path]trienode.TrieNode) {
	for path, node := range nodes {
		if t.classNodes[root] == nil {
			t.classNodes[root] = make(map[trieutils.Path]trienode.TrieNode)
		}
		t.classNodes[root][path] = node
		t.classPaths[path] = node
	}
}

func (t *layerTracker) trackContractNodes(root felt.Felt, nodes map[trieutils.Path]trienode.TrieNode) {
	for path, node := range nodes {
		if t.contractNodes[root] == nil {
			t.contractNodes[root] = make(map[trieutils.Path]trienode.TrieNode)
		}
		t.contractNodes[root][path] = node
		t.contractPaths[path] = node
	}
}

func (t *layerTracker) trackContractStorageNodes(root felt.Felt, nodes map[felt.Felt]map[trieutils.Path]trienode.TrieNode) {
	for owner, ownerNodes := range nodes {
		if t.contractStorageNodes[root] == nil {
			t.contractStorageNodes[root] = make(map[felt.Felt]map[trieutils.Path]trienode.TrieNode)
		}
		if t.contractStorageNodes[root][owner] == nil {
			t.contractStorageNodes[root][owner] = make(map[trieutils.Path]trienode.TrieNode)
		}
		for path, node := range ownerNodes {
			t.contractStorageNodes[root][owner][path] = node
			if t.contractStoragePaths[owner] == nil {
				t.contractStoragePaths[owner] = make(map[trieutils.Path]trienode.TrieNode)
			}
			t.contractStoragePaths[owner][path] = node
		}
	}
}

// resolveNode finds a node by traversing the layer hierarchy from the given root
func (t *layerTracker) resolveNode(root, owner felt.Felt, path trieutils.Path, isClass bool) ([]byte, error) {
	currentRoot := root
	for {
		if blob, found := t.findNodeInLayer(currentRoot, owner, path, isClass); found {
			return blob, nil
		}

		// Try parent layer if available
		parent, hasParent := t.childToParent[currentRoot]
		if !hasParent {
			return nil, fmt.Errorf("node not found in layer hierarchy: root=%v, owner=%v, path=%v",
				root.String(), owner.String(), path.String())
		}
		currentRoot = parent
	}
}

// findNodeInLayer checks if a node exists in a specific layer (without parent traversal)
func (t *layerTracker) findNodeInLayer(root, owner felt.Felt, path trieutils.Path, isClass bool) ([]byte, bool) {
	if isClass {
		if nodeMap, ok := t.classNodes[root]; ok {
			if node, exists := nodeMap[path]; exists {
				return node.Blob(), true
			}
		}
		return nil, false
	}

	if owner.IsZero() {
		if nodeMap, ok := t.contractNodes[root]; ok {
			if node, exists := nodeMap[path]; exists {
				return node.Blob(), true
			}
		}
		return nil, false
	}

	if storageMap, ok := t.contractStorageNodes[root]; ok {
		if nodeMap, ok := storageMap[owner]; ok {
			if node, exists := nodeMap[path]; exists {
				return node.Blob(), true
			}
		}
	}
	return nil, false
}

// ---- Test Data Generators ----

// generateRandomNode creates a random trie node for testing
func generateRandomNode(r *rand.Rand) *mockTrieNode {
	blob := make([]byte, 32)
	_, _ = r.Read(blob)
	return &mockTrieNode{blob: blob}
}

// generateRandomPath creates a random trie path for testing
func generateRandomPath(r *rand.Rand) trieutils.Path {
	var path trieutils.Path
	path.SetUint64(uint8(r.Uint64()%251), r.Uint64())
	return path
}

// generateRandomOwner creates a random contract owner for testing
func generateRandomOwner(r *rand.Rand, maxOwners int) felt.Felt {
	var owner felt.Felt
	owner.SetUint64(r.Uint64()%uint64(maxOwners) + 1)
	return owner
}

// createTestNodeSet generates a MergeNodeSet with controlled overlapping paths
func createTestNodeSet(nodeCount, layerIndex, totalLayers int, classNodesOnly bool, r *rand.Rand) *trienode.MergeNodeSet {
	// Create reusable path and owner sets for consistent overlaps
	paths := make([]trieutils.Path, 0, nodeCount)
	owners := make([]felt.Felt, 0, nodeCount)

	for i := 1; i < nodeCount+1; i++ { // starts at 1 to make sure owner is not zero
		var path trieutils.Path
		path.SetUint64(uint8(i), uint64(i))
		paths = append(paths, path)

		var owner felt.Felt
		owner.SetUint64(uint64(i))
		owners = append(owners, owner)
	}

	ownerSet := trienode.NewNodeSet(felt.Zero)
	childSets := make(map[felt.Felt]*trienode.NodeSet)

	// Deterministically add some nodes based on the layer index
	nodesPerLayer := nodeCount / totalLayers
	if nodesPerLayer == 0 {
		nodesPerLayer = 1
	}

	startIdx := (layerIndex * nodesPerLayer) % nodeCount
	for i := 0; i < nodesPerLayer; i++ {
		idx := (startIdx + i) % nodeCount
		path := paths[idx]
		node := generateRandomNode(r)

		if classNodesOnly || r.Intn(2) == 0 {
			ownerSet.Add(path, node)
		} else {
			owner := owners[r.Intn(len(owners))]
			childSet, exists := childSets[owner]
			if !exists {
				childSet = trienode.NewNodeSet(owner)
				childSets[owner] = childSet
			}
			childSet.Add(path, node)
		}
	}

	return &trienode.MergeNodeSet{
		OwnerSet:  ownerSet,
		ChildSets: childSets,
	}
}

// ---- Test Setup Helpers ----

// createPathDB creates a new PathDB instance for testing
func createPathDB() *Database {
	return New(memory.New(), &Config{
		CleanCacheSize:  1000,
		WriteBufferSize: 1000,
	})
}

// setupLayerTreeOverlap creates a layer tree with controlled overlapping nodes
// and returns both the tree and a tracker for verification
func setupLayerTree(numDiffs, nodesPerLayer int, r *rand.Rand) (*layerTree, *layerTracker) {
	pathDB := createPathDB()
	parent := felt.Zero
	tracker := newLayerTracker()

	// Create initial empty disk layer
	classNodes := createTestNodeSet(nodesPerLayer, 0, numDiffs+1, true, r)
	contractNodes := createTestNodeSet(nodesPerLayer, 0, numDiffs+1, false, r)
	flatClass, _ := classNodes.Flatten()
	flatContract, flatStorage := contractNodes.Flatten()

	diskLayer := newDiskLayer(
		parent,
		0,
		pathDB,
		nil,
		newBuffer(pathDB.config.WriteBufferSize, &nodeSet{
			classNodes:           flatClass,
			contractNodes:        flatContract,
			contractStorageNodes: flatStorage,
		}, 0),
	)

	tree := newLayerTree(diskLayer)

	// Track the base layer nodes
	tracker.trackClassNodes(parent, flatClass)
	tracker.trackContractNodes(parent, flatContract)
	tracker.trackContractStorageNodes(parent, flatStorage)

	// Create additional layers with controlled overlap
	for i := 1; i < numDiffs+1; i++ {
		layerRoot := *new(felt.Felt).SetUint64(uint64(i))

		// Generate nodes for this layer with controlled overlap
		classNodes := createTestNodeSet(nodesPerLayer, i, numDiffs+1, true, r)
		contractNodes := createTestNodeSet(nodesPerLayer, i, numDiffs+1, false, r)

		// Add layer to tree
		err := tree.add(layerRoot, parent, uint64(i), classNodes, contractNodes)
		if err != nil {
			panic(fmt.Sprintf("Failed to add layer %d: %v", i, err))
		}

		// Track nodes for verification
		flatClass, _ := classNodes.Flatten()
		flatContract, flatStorage := contractNodes.Flatten()
		tracker.trackClassNodes(layerRoot, flatClass)
		tracker.trackContractNodes(layerRoot, flatContract)
		tracker.trackContractStorageNodes(layerRoot, flatStorage)

		// Track parent relationship
		tracker.trackLayer(layerRoot, parent)
		parent = layerRoot
	}

	return tree, tracker
}

// verifyClassNodes verifies all class nodes in a layer against expected values
func verifyClassNodes(layer layer, root felt.Felt, tracker *layerTracker) error {
	for path := range tracker.classPaths {
		expectedBlob, expectedErr := tracker.resolveNode(root, felt.Zero, path, true)
		actualBlob, actualErr := layer.node(trieutils.NewClassTrieID(root), felt.Zero, path, true)

		if expectedErr != nil {
			if actualErr == nil {
				return fmt.Errorf("expected error for class path %s", path.String())
			}
		} else {
			if actualErr != nil {
				return fmt.Errorf("unexpected error for class path %s: %v", path.String(), actualErr)
			}
			if !bytes.Equal(expectedBlob, actualBlob) {
				return fmt.Errorf("blob mismatch for class path %s", path.String())
			}
		}
	}
	return nil
}

// verifyContractNodes verifies all contract nodes in a layer against expected values
func verifyContractNodes(layer layer, root felt.Felt, tracker *layerTracker) error {
	for path := range tracker.contractPaths {
		expectedBlob, expectedErr := tracker.resolveNode(root, felt.Zero, path, false)
		actualBlob, actualErr := layer.node(trieutils.NewContractTrieID(root), felt.Zero, path, false)

		if expectedErr != nil {
			if actualErr == nil {
				return fmt.Errorf("expected error for contract path %s", path.String())
			}
		} else {
			if actualErr != nil {
				return fmt.Errorf("unexpected error for contract path %s: %v", path.String(), actualErr)
			}
			if !bytes.Equal(expectedBlob, actualBlob) {
				return fmt.Errorf("blob mismatch for contract path %s", path.String())
			}
		}
	}
	return nil
}

// verifyContractStorageNodes verifies all contract storage nodes in a layer against expected values
func verifyContractStorageNodes(layer layer, root felt.Felt, tracker *layerTracker) error {
	for owner, paths := range tracker.contractStoragePaths {
		for path := range paths {
			expectedBlob, expectedErr := tracker.resolveNode(root, owner, path, false)
			actualBlob, actualErr := layer.node(trieutils.NewContractStorageTrieID(owner, root), owner, path, false)

			if expectedErr != nil {
				if actualErr == nil {
					return fmt.Errorf("expected error for storage path %s (owner %s)",
						path.String(), owner.String())
				}
			} else {
				if actualErr != nil {
					return fmt.Errorf("unexpected error for storage path %s (owner %s): %v",
						path.String(), owner.String(), actualErr)
				}
				if !bytes.Equal(expectedBlob, actualBlob) {
					return fmt.Errorf("blob mismatch for storage path %s (owner %s)",
						path.String(), owner.String())
				}
			}
		}
	}
	return nil
}

// verifyLayer verifies all nodes in a single layer against expected values
func verifyLayer(tree *layerTree, root felt.Felt, tracker *layerTracker) error {
	layer := tree.get(root)
	if layer == nil {
		return fmt.Errorf("layer not found for root %s", root.String())
	}

	err := verifyClassNodes(layer, root, tracker)
	if err != nil {
		return err
	}
	err = verifyContractNodes(layer, root, tracker)
	if err != nil {
		return err
	}
	err = verifyContractStorageNodes(layer, root, tracker)
	if err != nil {
		return err
	}
	return nil
}
