package pathdb

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/stretchr/testify/assert"
)

var (
	testOwner = *felt.NewFromUint64[felt.Address](1234567890)
	testPath  = *new(trieutils.Path).SetUint64(100, 1234567890)
)

func TestNodeKey(t *testing.T) {
	// Test for non-class node
	key1 := nodeKey(&testOwner, &testPath, false)
	assert.Equal(t, nodeCacheSize, len(key1))
	assert.Equal(t, uint8(0), key1[nodeCacheSize-1])

	// Test for class node
	key2 := nodeKey(&testOwner, &testPath, true)
	assert.Equal(t, nodeCacheSize, len(key2))
	assert.Equal(t, uint8(1), key2[nodeCacheSize-1])

	// Verify keys are different
	assert.NotEqual(t, key1, key2)
}

func TestPutAndGetNode(t *testing.T) {
	cache := newCleanCache(1024 * 1024)
	blob := []byte("test data")

	// Test storing and retrieving non-class node
	cache.putNode(&testOwner, &testPath, false, blob)
	retrieved := cache.getNode(&testOwner, &testPath, false)
	assert.Equal(t, blob, retrieved)

	// Test storing and retrieving class node
	classBlob := []byte("class data")
	cache.putNode(&testOwner, &testPath, true, classBlob)
	classRetrieved := cache.getNode(&testOwner, &testPath, true)
	assert.Equal(t, classBlob, classRetrieved)
}

func TestCacheMisses(t *testing.T) {
	cache := newCleanCache(1024 * 1024)
	owner := *felt.NewFromUint64[felt.Address](1234567890)
	path := *new(trieutils.Path).SetUint64(100, 1234567890)

	// Test retrieving non-existent entry
	result := cache.getNode(&owner, &path, false)
	assert.Nil(t, result)

	// Store data and test retrieving with different parameters
	blob := []byte("test data")
	cache.putNode(&owner, &path, false, blob)

	// Different owner
	diffOwner := *felt.NewFromUint64[felt.Address](9876543210)
	result = cache.getNode(&diffOwner, &testPath, false)
	assert.Nil(t, result)

	// Different path
	diffPath := *new(trieutils.Path).SetUint64(100, 9876543210)
	result = cache.getNode(&testOwner, &diffPath, false)
	assert.Nil(t, result)

	// Different isClass flag
	result = cache.getNode(&testOwner, &testPath, true)
	assert.Nil(t, result)
}
