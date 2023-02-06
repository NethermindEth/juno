package core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/fxamacker/cbor/v2"
)

type StateUpdate struct {
	BlockHash *felt.Felt
	NewRoot   *felt.Felt
	OldRoot   *felt.Felt
	StateDiff *StateDiff
}

type StateDiff struct {
	StorageDiffs      map[felt.Felt][]StorageDiff
	Nonces            map[felt.Felt]*felt.Felt
	DeployedContracts []DeployedContract
	DeclaredContracts DeclaredContract
}

type StorageDiff struct {
	Key   *felt.Felt
	Value *felt.Felt
}

type DeclaredContract map[*felt.Felt]ClassDefinition

type DeployedContract struct {
	Address   *felt.Felt
	ClassHash *felt.Felt
}

type ClassDefinition struct {
	Abi         interface{}
	EntryPoints EntryPointsByType
	Program     string
}

type EntryPointsByType struct {
	Constructors []EntryPoint
	Externals    []EntryPoint
	L1Handlers   []EntryPoint
}

// ClassStorage allows saving and retrieving Class from the database
type ClassStorage struct {
	txn db.Transaction
}

func NewClassStorage(txn db.Transaction) *ClassStorage {
	return &ClassStorage{txn: txn}
}

// Put stores a given class in the database
func (s *ClassStorage) Put(blockNum uint64, classHash *felt.Felt, class *ClassDefinition) error {
	key := generateKey(blockNum, classHash)
	if valueBytes, err := cbor.Marshal(class); err != nil {
		return err
	} else {
		s.txn.Set(db.ContractClass.Key(key), valueBytes)
	}

	return nil
}

func (s *ClassStorage) GetClass(blockNum uint64, classHash *felt.Felt) (*ClassDefinition, error) {
	key := generateKey(blockNum, classHash)
	if classBytes, err := s.txn.Get(db.ContractClass.Key(key)); err != nil {
		return nil, err
	} else {
		class := new(ClassDefinition)
		return class, cbor.Unmarshal(classBytes, class)
	}
}

func generateKey(blockNum uint64, classHash *felt.Felt) []byte {
	var blockNumBytes [8]byte
	binary.BigEndian.PutUint64(blockNumBytes[:], blockNum)
	key := append(blockNumBytes[:], classHash.Marshal()...)
	return key
}
