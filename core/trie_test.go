package core

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
	"github.com/stretchr/testify/assert"
)

func TestNode_Marshall(t *testing.T) {
	value, _ := new(felt.Felt).SetRandom()
	path1 := bitset.FromWithLength(44, []uint64{44})
	path2 := bitset.FromWithLength(22, []uint64{22})

	tests := [...]TrieNode{
		{
			value: value,
			left:  nil,
			right: nil,
		},
		{
			value: value,
			left:  path1,
			right: nil,
		},
		{
			value: value,
			left:  nil,
			right: path2,
		},
		{
			value: value,
			left:  path1,
			right: path2,
		},
		{
			value: value,
			left:  path2,
			right: path1,
		},
	}
	for _, test := range tests {
		data, _ := test.MarshalBinary()
		unmarshaled := new(TrieNode)
		_ = unmarshaled.UnmarshalBinary(data)
		if !test.Equal(unmarshaled) {
			t.Error("TestNode_Marshall failed")
		}
	}

	malformed := new([felt.Bytes + 1]byte)
	malformed[felt.Bytes] = 'l'
	if err := new(TrieNode).UnmarshalBinary(malformed[2:]); err == nil {
		t.Error("TestNode_Marshall failed")
	}
	if err := new(TrieNode).UnmarshalBinary(malformed[:]); err == nil {
		t.Error("TestNode_Marshall failed")
	}
	malformed[felt.Bytes] = 'z'
	if err := new(TrieNode).UnmarshalBinary(malformed[:]); err == nil {
		t.Error("TestNode_Marshall failed")
	}
}

func TestPathFromKey(t *testing.T) {
	key, _ := new(felt.Felt).SetRandom()
	path := PathFromKey(key)
	keyRegular := key.ToRegular()
	for bit := 0; bit < felt.Bits; bit++ {
		if keyRegular.Bit(uint64(bit)) > 0 != path.Test(uint(bit)) {
			t.Error("TestPathFromKey failed")
			break
		}
	}

	// Make sure they dont share the same underlying memory
	key.Halve()
	keyRegular = key.ToRegular()
	for bit := 0; bit < felt.Bits; bit++ {
		if keyRegular.Bit(uint64(bit)) > 0 != path.Test(uint(bit)) {
			return
		}
	}
	t.Error("TestPathFromKey failed")
}

type (
	storage         map[string]string
	testTrieStorage struct {
		storage storage
	}
)

func (s *testTrieStorage) Put(key *StoragePath, value *StorageValue) error {
	keyEnc, err := key.MarshalBinary()
	if err != nil {
		return err
	}
	vEnc, err := value.MarshalBinary()
	if err != nil {
		return err
	}
	s.storage[hex.EncodeToString(keyEnc)] = hex.EncodeToString(vEnc)
	return nil
}

func (s *testTrieStorage) Get(key *StoragePath) (*StorageValue, error) {
	keyEnc, _ := key.MarshalBinary()
	value, found := s.storage[hex.EncodeToString(keyEnc)]
	if !found {
		panic("not found")
	}

	v := new(StorageValue)
	decoded, _ := hex.DecodeString(value)
	err := v.UnmarshalBinary(decoded)
	return v, err
}

func TestFindCommonPath(t *testing.T) {
	tests := [...]struct {
		path1  *StoragePath
		path2  *StoragePath
		common *StoragePath
		subset bool
	}{
		{
			path1:  bitset.New(16).Set(4).Set(3),
			path2:  bitset.New(16).Set(4),
			common: bitset.New(12).Set(0),
			subset: false,
		},
		{
			path1:  bitset.New(2).Set(1),
			path2:  bitset.New(2),
			common: bitset.New(0),
			subset: false,
		},
		{
			path1:  bitset.New(2).Set(1),
			path2:  bitset.New(2).Set(1),
			common: bitset.New(2).Set(1),
			subset: true,
		},
		{
			path1:  bitset.New(10),
			path2:  bitset.New(8),
			common: bitset.New(8),
			subset: true,
		},
	}

	for _, test := range tests {
		if common, subset := FindCommonPath(test.path1, test.path2); !test.common.Equal(common) || subset != test.subset {
			t.Errorf("TestFindCommonPath: Expected %s (%d) Got %s (%d)", test.common.DumpAsBits(),
				test.common.Len(), common.DumpAsBits(), common.Len())
		}
	}
}

func TestTriePut(t *testing.T) {
	storage := &testTrieStorage{
		storage: make(storage),
	}
	trie := &Trie{
		root:    nil,
		storage: storage,
	}

	tests := [...]struct {
		key   *TrieKey
		value *TrieValue
		root  *StoragePath
	}{
		{
			key:   new(felt.Felt).SetUint64(2),
			value: new(felt.Felt).SetUint64(0),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(1),
			value: new(felt.Felt).SetUint64(1),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(3),
			value: new(felt.Felt).SetUint64(3),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(3),
			value: new(felt.Felt).SetUint64(4),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(0),
			value: new(felt.Felt).SetUint64(5),
			root:  nil,
		},
	}
	for idx, test := range tests {
		if err := trie.Put(test.key, test.value); err != nil {
			t.Errorf("TestTriePut: Put() failed at test #%d", idx)
		}
		if value, err := trie.Get(test.key); err != nil || !value.Equal(test.value) {
			t.Errorf("TestTriePut: Get() failed at test #%d", idx)
		}
		if test.root != nil && !test.root.Equal(trie.root) {
			t.Errorf("TestTriePut: Unexpected root at test #%d", idx)
		}
	}
}

func TestGetSpecPath(t *testing.T) {
	storage := &testTrieStorage{
		storage: make(storage),
	}
	trie := &Trie{
		root:    nil,
		storage: storage,
	}

	// build example trie from https://docs.starknet.io/documentation/develop/State/starknet-state/
	// and check paths
	two := felt.NewFelt(2)
	five := felt.NewFelt(5)
	one := felt.One()
	trie.Put(&two, &one)
	assert.Equal(t, true, GetSpecPath(trie.root, nil).Equal(PathFromKey(&two)))

	trie.Put(&five, &one)
	expectedRoot, _ := FindCommonPath(PathFromKey(&two), PathFromKey(&five))
	assert.Equal(t, true, GetSpecPath(trie.root, nil).Equal(expectedRoot))

	rootNode, err := trie.storage.Get(trie.root)
	if err != nil {
		t.Error()
	}

	assert.Equal(t, true, rootNode.left != nil && rootNode.right != nil)

	expectedLeftSpecPath := bitset.New(2).Set(1)
	expectedRightSpecPath := bitset.New(2).Set(0)
	assert.Equal(t, true, GetSpecPath(rootNode.left, trie.root).Equal(expectedLeftSpecPath))
	assert.Equal(t, true, GetSpecPath(rootNode.right, trie.root).Equal(expectedRightSpecPath))
}

func TestGetSpecPath_ZeroRoot(t *testing.T) {
	storage := &testTrieStorage{
		storage: make(storage),
	}
	trie := &Trie{
		root:    nil,
		storage: storage,
	}

	zero := felt.NewFelt(0)
	msbOne, _ := new(felt.Felt).SetString("0x800000000000000000000000000000000000000000000000000000000000000")
	one := felt.One()
	trie.Put(&zero, &one)
	trie.Put(msbOne, &one)

	zeroPath := bitset.New(0)
	assert.Equal(t, true, trie.root.Equal(zeroPath))
	assert.Equal(t, true, GetSpecPath(trie.root, nil).Equal(zeroPath))
}
