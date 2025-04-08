package hashdb

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
	memorydb "github.com/NethermindEth/juno/db/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBasicCacheOperations(t *testing.T) {
	diskdb := memorydb.New()
	database := New(diskdb, &Config{
		CleanCacheType: CacheTypeLRU,
		CleanCacheSize: 4 * 1024 * 1024,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 1 * 1024 * 1024,
	})

	owner := felt.Zero
	path := trieutils.Path{}
	hash := new(felt.Felt).SetUint64(123)
	blob := []byte("test node data")
	isLeaf := false

	err := database.insert(db.ContractStorage, owner, path, *hash, blob, isLeaf)
	require.NoError(t, err)

	retrievedBlob, err := database.Node(db.ContractStorage, owner, path, *hash, isLeaf)
	require.NoError(t, err)
	assert.Equal(t, blob, retrievedBlob)

	err = database.Cap(0)
	require.NoError(t, err)

	retrievedBlob, err = database.Node(db.ContractStorage, owner, path, *hash, isLeaf)
	require.NoError(t, err)
	assert.Equal(t, blob, retrievedBlob)

	database2 := New(diskdb, &Config{
		CleanCacheType: CacheTypeFastCache,
		CleanCacheSize: 0,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 0,
	})
	retrievedBlob, err = database2.Node(db.ContractStorage, owner, path, *hash, isLeaf)
	require.NoError(t, err)
	assert.Equal(t, blob, retrievedBlob)
}

func TestRefCountingMechanism(t *testing.T) {
	diskdb := memorydb.New()
	database := New(diskdb, &Config{
		CleanCacheType: CacheTypeFastCache,
		CleanCacheSize: 4 * 1024 * 1024,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 1 * 1024 * 1024,
	})

	parentHash := new(felt.Felt).SetUint64(100)
	childHash1 := new(felt.Felt).SetUint64(101)
	childHash2 := new(felt.Felt).SetUint64(102)

	parentBlob := []byte("parent node data")
	childBlob1 := []byte("child node 1 data")
	childBlob2 := []byte("child node 2 data")

	err := database.insert(db.ContractStorage, felt.Zero, trieutils.Path{}, *parentHash, parentBlob, false)
	require.NoError(t, err)

	err = database.insert(db.ContractStorage, felt.Zero, trieutils.Path{}, *childHash1, childBlob1, false)
	require.NoError(t, err)

	err = database.insert(db.ContractStorage, felt.Zero, trieutils.Path{}, *childHash2, childBlob2, false)
	require.NoError(t, err)

	refcountCache, ok := database.DirtyCache.(*RefCountCache)
	require.True(t, ok, "Expected RefCountCache")

	parentKey := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, trieutils.Path{}, *parentHash, false)
	child1Key := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, trieutils.Path{}, *childHash1, false)
	child2Key := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, trieutils.Path{}, *childHash2, false)

	refcountCache.Reference(child1Key, parentKey)
	refcountCache.Reference(child2Key, parentKey)

	parentNode, found := refcountCache.Get(parentKey)
	require.True(t, found)
	assert.Contains(t, parentNode.external, string(child1Key))
	assert.Contains(t, parentNode.external, string(child2Key))

	child1Node, found := refcountCache.Get(child1Key)
	require.True(t, found)
	assert.Equal(t, uint32(1), child1Node.parents)

	child2Node, found := refcountCache.Get(child2Key)
	require.True(t, found)
	assert.Equal(t, uint32(1), child2Node.parents)

	refcountCache.Dereference(parentKey)

	_, found = refcountCache.Get(child1Key)
	assert.False(t, found)

	_, found = refcountCache.Get(child2Key)
	assert.False(t, found)

	_, found = refcountCache.Get(parentKey)
	assert.False(t, found)
}

func TestMultiTrieOperations(t *testing.T) {
	diskdb := memorydb.New()
	database := New(diskdb, &Config{
		CleanCacheType: CacheTypeFastCache,
		CleanCacheSize: 4 * 1024 * 1024,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 1 * 1024 * 1024,
	})

	// Create nodes for different tries
	classHash := new(felt.Felt).SetUint64(1000)
	contractHash := new(felt.Felt).SetUint64(2000)
	storageHash := new(felt.Felt).SetUint64(3000)

	classBlob := []byte("class node data")
	contractBlob := []byte("contract node data")
	storageBlob := []byte("storage node data")

	// Insert into different buckets
	err := database.insert(db.ClassTrie, felt.Zero, trieutils.Path{}, *classHash, classBlob, false)
	require.NoError(t, err)

	contractOwner := new(felt.Felt).SetUint64(5000)
	err = database.insert(db.ContractTrieContract, *contractOwner, trieutils.Path{}, *contractHash, contractBlob, false)
	require.NoError(t, err)

	storageOwner := new(felt.Felt).SetUint64(6000)
	err = database.insert(db.ContractTrieStorage, *storageOwner, trieutils.Path{}, *storageHash, storageBlob, false)
	require.NoError(t, err)

	// Retrieve from each bucket
	retrievedClassBlob, err := database.Node(db.ClassTrie, felt.Zero, trieutils.Path{}, *classHash, false)
	require.NoError(t, err)
	assert.Equal(t, classBlob, retrievedClassBlob)

	retrievedContractBlob, err := database.Node(db.ContractTrieContract, *contractOwner, trieutils.Path{}, *contractHash, false)
	require.NoError(t, err)
	assert.Equal(t, contractBlob, retrievedContractBlob)

	retrievedStorageBlob, err := database.Node(db.ContractTrieStorage, *storageOwner, trieutils.Path{}, *storageHash, false)
	require.NoError(t, err)
	assert.Equal(t, storageBlob, retrievedStorageBlob)

	// Commit everything
	err = database.Commit(*classHash)
	require.NoError(t, err)

	err = database.Commit(*contractHash)
	require.NoError(t, err)

	err = database.Commit(*storageHash)
	require.NoError(t, err)

	// Verify everything is in the database by creating a new instance
	database2 := New(diskdb, &Config{
		CleanCacheType: CacheTypeFastCache,
		CleanCacheSize: 0,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 0,
	})

	retrievedClassBlob, err = database2.Node(db.ClassTrie, felt.Zero, trieutils.Path{}, *classHash, false)
	require.NoError(t, err)
	assert.Equal(t, classBlob, retrievedClassBlob)

	retrievedContractBlob, err = database2.Node(db.ContractTrieContract, *contractOwner, trieutils.Path{}, *contractHash, false)
	require.NoError(t, err)
	assert.Equal(t, contractBlob, retrievedContractBlob)

	retrievedStorageBlob, err = database2.Node(db.ContractTrieStorage, *storageOwner, trieutils.Path{}, *storageHash, false)
	require.NoError(t, err)
	assert.Equal(t, storageBlob, retrievedStorageBlob)
}

// TestCommitAndCapFunctionality 	tests the commit and cap functionality
func TestCommitAndCapFunctionality(t *testing.T) {
	diskdb := memorydb.New()
	database := New(diskdb, &Config{
		CleanCacheType: CacheTypeFastCache,
		CleanCacheSize: 4 * 1024 * 1024,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 1 * 1024 * 1024,
	})

	rootHash := new(felt.Felt).SetUint64(100)
	child1Hash := new(felt.Felt).SetUint64(101)
	child2Hash := new(felt.Felt).SetUint64(102)

	rootBlob := []byte("root node data")
	child1Blob := []byte("child node 1 data")
	child2Blob := []byte("child node 2 data")

	err := database.insert(db.ContractStorage, felt.Zero, trieutils.Path{}, *rootHash, rootBlob, false)
	require.NoError(t, err)

	err = database.insert(db.ContractStorage, felt.Zero, trieutils.Path{}, *child1Hash, child1Blob, false)
	require.NoError(t, err)

	err = database.insert(db.ContractStorage, felt.Zero, trieutils.Path{}, *child2Hash, child2Blob, false)
	require.NoError(t, err)

	refcountCache, ok := database.DirtyCache.(*RefCountCache)
	require.True(t, ok, "Expected RefCountCache")

	rootKey := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, trieutils.Path{}, *rootHash, false)
	child1Key := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, trieutils.Path{}, *child1Hash, false)
	child2Key := trieutils.NodeKeyByHash(db.ContractStorage, felt.Zero, trieutils.Path{}, *child2Hash, false)

	refcountCache.Reference(child1Key, rootKey)
	refcountCache.Reference(child2Key, rootKey)

	err = database.Cap(uint64(len(rootBlob)))
	require.NoError(t, err)

	retrievedRootBlob, err := database.Node(db.ContractStorage, felt.Zero, trieutils.Path{}, *rootHash, false)
	require.NoError(t, err)
	assert.Equal(t, rootBlob, retrievedRootBlob)

	retrievedChild1Blob, err := database.Node(db.ContractStorage, felt.Zero, trieutils.Path{}, *child1Hash, false)
	require.NoError(t, err)
	assert.Equal(t, child1Blob, retrievedChild1Blob)

	retrievedChild2Blob, err := database.Node(db.ContractStorage, felt.Zero, trieutils.Path{}, *child2Hash, false)
	require.NoError(t, err)
	assert.Equal(t, child2Blob, retrievedChild2Blob)

	err = database.Commit(*rootHash)
	require.NoError(t, err)

	retrievedRootBlob, err = database.Node(db.ContractStorage, felt.Zero, trieutils.Path{}, *rootHash, false)
	require.NoError(t, err)
	assert.Equal(t, rootBlob, retrievedRootBlob)

	retrievedChild1Blob, err = database.Node(db.ContractStorage, felt.Zero, trieutils.Path{}, *child1Hash, false)
	require.NoError(t, err)
	assert.Equal(t, child1Blob, retrievedChild1Blob)

	retrievedChild2Blob, err = database.Node(db.ContractStorage, felt.Zero, trieutils.Path{}, *child2Hash, false)
	require.NoError(t, err)
	assert.Equal(t, child2Blob, retrievedChild2Blob)
}

func TestUpdateFunctionality(t *testing.T) {
	diskdb := memorydb.New()
	database := New(diskdb, &Config{
		CleanCacheType: CacheTypeFastCache,
		CleanCacheSize: 4 * 1024 * 1024,
		DirtyCacheType: CacheTypeRefCount,
		DirtyCacheSize: 1 * 1024 * 1024,
	})

	classRoot := new(felt.Felt).SetUint64(1000)

	path1 := trieutils.NewBitArray(3, 0b1101)
	path2 := trieutils.NewBitArray(3, 0b110)

	classNode1 := createTestNode([]byte("class node 1"), false)
	classNode2 := createTestNode([]byte("class node 2"), true)

	contractOwner := new(felt.Felt).SetUint64(5000)
	contractNode1 := createTestNode([]byte("contract node 1"), false)
	contractNode2 := createTestNode([]byte("contract node 2"), true)

	classNodes := map[trieutils.Path]trienode.TrieNode{
		path1: classNode1,
		path2: classNode2,
	}

	contractNodes := map[felt.Felt]map[trieutils.Path]trienode.TrieNode{
		*contractOwner: {
			path1: contractNode1,
			path2: contractNode2,
		},
	}

	database.Update(*classRoot, felt.Zero, 1, classNodes, contractNodes)

	retrievedClassNode1, err := database.Node(db.ContractStorage, felt.Zero, path1, classNode1.Hash(), false)
	require.NoError(t, err)
	assert.Equal(t, classNode1.Blob(), retrievedClassNode1)

	retrievedClassNode2, err := database.Node(db.ContractStorage, felt.Zero, path2, classNode2.Hash(), true)
	require.NoError(t, err)
	assert.Equal(t, classNode2.Blob(), retrievedClassNode2)

	retrievedContractNode1, err := database.Node(db.ContractTrieStorage, *contractOwner, path1, contractNode1.Hash(), false)
	require.NoError(t, err)
	assert.Equal(t, contractNode1.Blob(), retrievedContractNode1)

	retrievedContractNode2, err := database.Node(db.ContractTrieStorage, *contractOwner, path2, contractNode2.Hash(), true)
	require.NoError(t, err)
	assert.Equal(t, contractNode2.Blob(), retrievedContractNode2)

	deletedClassNode := &testDeletedNode{hash: *new(felt.Felt).SetUint64(9999), blob: []byte("deleted node")}
	classNodesWithDelete := map[trieutils.Path]trienode.TrieNode{
		path1: deletedClassNode,
	}

	database.Update(*classRoot, felt.Zero, 2, classNodesWithDelete, nil)

	_, err = database.Node(db.ContractStorage, felt.Zero, path1, deletedClassNode.Hash(), false)
	assert.Error(t, err)
}

type testNode struct {
	hash   felt.Felt
	blob   []byte
	isLeaf bool
}

func createTestNode(blob []byte, isLeaf bool) *testNode {
	hash := new(felt.Felt).SetBytes(blob)
	return &testNode{
		hash:   *hash,
		blob:   blob,
		isLeaf: isLeaf,
	}
}

func (n *testNode) Hash() felt.Felt {
	return n.hash
}

func (n *testNode) Blob() []byte {
	return n.blob
}

func (n *testNode) IsLeaf() bool {
	return n.isLeaf
}

type testDeletedNode struct {
	hash felt.Felt
	blob []byte
}

func (n *testDeletedNode) Hash() felt.Felt {
	return n.hash
}

func (n *testDeletedNode) Blob() []byte {
	return n.blob
}

func (n *testDeletedNode) IsLeaf() bool {
	return false
}

func (n *testDeletedNode) IsDeleted() bool {
	return true
}
