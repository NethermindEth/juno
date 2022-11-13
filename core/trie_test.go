package core

import (
	"encoding/hex"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
)

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

func (s *testTrieStorage) Put(key *StoragePath, value StorageValue) error {
	keyEnc, _ := key.MarshalBinary()
	s.storage[hex.EncodeToString(keyEnc)] = hex.EncodeToString(value)
	return nil
}

func (s *testTrieStorage) Get(key *StoragePath) (StorageValue, error) {
	keyEnc, _ := key.MarshalBinary()
	value, found := s.storage[hex.EncodeToString(keyEnc)]
	if !found {
		panic("not found")
	}
	return hex.DecodeString(value)
}
