package core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

func GetContractClassHash(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var classHash felt.Felt
	err := r.Get(db.ContractClassHashKey(addr), func(data []byte) error {
		classHash.SetBytes(data)
		return nil
	})
	return classHash, err
}

func WriteContractClassHash(txn db.KeyValueWriter, addr, classHash *felt.Felt) error {
	return txn.Put(db.ContractClassHashKey(addr), classHash.Marshal())
}

func GetContractNonce(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var nonce felt.Felt
	err := r.Get(db.ContractNonceKey(addr), func(data []byte) error {
		nonce.SetBytes(data)
		return nil
	})
	return nonce, err
}

func WriteContractNonce(w db.KeyValueWriter, addr, nonce *felt.Felt) error {
	return w.Put(db.ContractNonceKey(addr), nonce.Marshal())
}

func HasClass(r db.KeyValueReader, classHash *felt.Felt) (bool, error) {
	return r.Has(db.ClassKey(classHash))
}

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*DeclaredClass, error) {
	var class *DeclaredClass

	err := r.Get(db.ClassKey(classHash), func(data []byte) error {
		return encoder.Unmarshal(data, &class)
	})
	return class, err
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
	var number uint64
	err := r.Get(db.BlockHeaderNumbersByHashKey(hash), func(data []byte) error {
		number = binary.BigEndian.Uint64(data)
		return nil
	})
	return number, err
}

func DeleteBlockHeaderNumberByHash(w db.KeyValueWriter, hash *felt.Felt) error {
	return w.Delete(db.BlockHeaderNumbersByHashKey(hash))
}

func GetBlockHeaderByNumber(r db.KeyValueReader, number uint64) (*Header, error) {
	var header *Header
	err := r.Get(db.BlockHeaderByNumberKey(number), func(data []byte) error {
		return encoder.Unmarshal(data, &header)
	})
	return header, err
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
	var height uint64
	err := r.Get(db.ContractDeploymentHeightKey(addr), func(data []byte) error {
		height = binary.BigEndian.Uint64(data)
		return nil
	})
	return height, err
}

func DeleteContractDeploymentHeight(w db.KeyValueWriter, addr *felt.Felt) error {
	return w.Delete(db.ContractDeploymentHeightKey(addr))
}
