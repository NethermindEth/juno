package core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

func GetContractClassHash(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var classHash felt.Felt
	data, err := r.Get(db.ContractClassHashKey(addr))
	if err != nil {
		return felt.Zero, err
	}
	classHash.SetBytes(data)
	return classHash, nil
}

func WriteContractClassHash(txn db.KeyValueWriter, addr, classHash *felt.Felt) error {
	return txn.Put(db.ContractClassHashKey(addr), classHash.Marshal())
}

func GetContractNonce(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var nonce felt.Felt
	data, err := r.Get(db.ContractNonceKey(addr))
	if err != nil {
		return felt.Zero, err
	}
	nonce.SetBytes(data)
	return nonce, nil
}

func WriteContractNonce(w db.KeyValueWriter, addr, nonce *felt.Felt) error {
	return w.Put(db.ContractNonceKey(addr), nonce.Marshal())
}

func HasClass(r db.KeyValueReader, classHash *felt.Felt) (bool, error) {
	return r.Has(db.ClassKey(classHash))
}

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*DeclaredClass, error) {
	var class *DeclaredClass

	data, err := r.Get(db.ClassKey(classHash))
	if err != nil {
		return nil, err
	}

	err = encoder.Unmarshal(data, &class)
	if err != nil {
		return nil, err
	}

	return class, nil
}

func WriteClass(w db.KeyValueWriter, classHash *felt.Felt, class *DeclaredClass) error {
	data, err := encoder.Marshal(class)
	if err != nil {
		return err
	}
	return w.Put(db.ClassKey(classHash), data)
}

func DeleteClass(w db.KeyValueWriter, classHash *felt.Felt) error {
	return w.Delete(db.ClassKey(classHash))
}

func WriteBlockHeaderNumberByHash(w db.KeyValueWriter, hash *felt.Felt, number uint64) error {
	enc := MarshalBlockNumber(number)
	return w.Put(db.BlockHeaderNumbersByHashKey(hash), enc)
}

func GetBlockHeaderNumberByHash(r db.KeyValueReader, hash *felt.Felt) (uint64, error) {
	data, err := r.Get(db.BlockHeaderNumbersByHashKey(hash))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data), nil
}

func DeleteBlockHeaderNumberByHash(w db.KeyValueWriter, hash *felt.Felt) error {
	return w.Delete(db.BlockHeaderNumbersByHashKey(hash))
}

func GetBlockHeaderByNumber(r db.KeyValueReader, number uint64) (*Header, error) {
	var header *Header
	data, err := r.Get(db.BlockHeaderByNumberKey(number))
	if err != nil {
		return nil, err
	}
	err = encoder.Unmarshal(data, &header)
	if err != nil {
		return nil, err
	}

	return header, nil
}

func WriteBlockHeaderByNumber(w db.KeyValueWriter, number uint64, header *Header) error {
	data, err := encoder.Marshal(header)
	if err != nil {
		return err
	}
	return w.Put(db.BlockHeaderByNumberKey(number), data)
}

func DeleteBlockHeaderByNumber(w db.KeyValueWriter, number uint64) error {
	return w.Delete(db.BlockHeaderByNumberKey(number))
}

func WriteContractDeploymentHeight(w db.KeyValueWriter, addr *felt.Felt, height uint64) error {
	enc := MarshalBlockNumber(height)
	return w.Put(db.ContractDeploymentHeightKey(addr), enc)
}

func GetContractDeploymentHeight(r db.KeyValueReader, addr *felt.Felt) (uint64, error) {
	data, err := r.Get(db.ContractDeploymentHeightKey(addr))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(data), nil
}

func DeleteContractDeploymentHeight(w db.KeyValueWriter, addr *felt.Felt) error {
	return w.Delete(db.ContractDeploymentHeightKey(addr))
}
