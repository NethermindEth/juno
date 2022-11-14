package core

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/bits-and-blooms/bitset"
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
