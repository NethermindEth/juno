package core

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
)

/**
State-related accessors

███████ ████████  █████  ████████ ███████
██         ██    ██   ██    ██    ██
███████    ██    ███████    ██    █████
     ██    ██    ██   ██    ██    ██
███████    ██    ██   ██    ██    ███████

**/

func GetContractClassHash(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var classHash felt.Felt
	err := r.Get(db.ContractClassHashKey(addr), func(data []byte) error {
		classHash.SetBytes(data)
		return nil
	})
	return classHash, err
}

func WriteContractClassHash(w db.KeyValueWriter, addr, classHash *felt.Felt) error {
	return w.Put(db.ContractClassHashKey(addr), classHash.Marshal())
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

func GetClass(r db.KeyValueReader, classHash *felt.Felt) (*DeclaredClassDefinition, error) {
	var class *DeclaredClassDefinition

	err := r.Get(db.ClassKey(classHash), func(data []byte) error {
		return encoder.Unmarshal(data, &class)
	})
	return class, err
}

func WriteClass(w db.KeyValueWriter, classHash *felt.Felt, class *DeclaredClassDefinition) error {
	data, err := encoder.Marshal(class)
	if err != nil {
		return err
	}
	return w.Put(db.ClassKey(classHash), data)
}

func DeleteClass(w db.KeyValueWriter, classHash *felt.Felt) error {
	return w.Delete(db.ClassKey(classHash))
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

func GetStateUpdateByBlockNum(r db.KeyValueReader, blockNum uint64) (*StateUpdate, error) {
	var stateUpdate *StateUpdate
	err := r.Get(db.StateUpdateByBlockNumKey(blockNum), func(data []byte) error {
		return encoder.Unmarshal(data, &stateUpdate)
	})
	if err != nil {
		return nil, err
	}
	return stateUpdate, nil
}

func WriteStateUpdateByBlockNum(w db.KeyValueWriter, blockNum uint64, stateUpdate *StateUpdate) error {
	data, err := encoder.Marshal(stateUpdate)
	if err != nil {
		return err
	}
	return w.Put(db.StateUpdateByBlockNumKey(blockNum), data)
}

func DeleteStateUpdateByBlockNum(w db.KeyValueWriter, blockNum uint64) error {
	return w.Delete(db.StateUpdateByBlockNumKey(blockNum))
}

func GetStateUpdateByHash(r db.KeyValueReader, hash *felt.Felt) (*StateUpdate, error) {
	var blockNum uint64
	err := r.Get(db.BlockHeaderNumbersByHashKey(hash), func(data []byte) error {
		blockNum = binary.BigEndian.Uint64(data)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetStateUpdateByBlockNum(r, blockNum)
}

// WriteContractStorageHistory writes the old value of a storage location
// for the given contract which changed on height `height`.
func WriteContractStorageHistory(
	w db.KeyValueWriter,
	contractAddress,
	storageLocation,
	oldValue *felt.Felt,
	height uint64,
) error {
	key := db.ContractStorageHistoryAtBlockKey(contractAddress, storageLocation, height)
	return w.Put(key, oldValue.Marshal())
}

// DeleteContractStorageHistory deletes the history at the given height
func DeleteContractStorageHistory(
	w db.KeyValueWriter,
	contractAddress,
	storageLocation *felt.Felt,
	height uint64,
) error {
	key := db.ContractStorageHistoryAtBlockKey(contractAddress, storageLocation, height)
	return w.Delete(key)
}

// WriteContractNonceHistory writes the old value of a nonce
// for the given contract which changed on height `height`
func WriteContractNonceHistory(
	w db.KeyValueWriter,
	contractAddress,
	oldValue *felt.Felt,
	height uint64,
) error {
	key := db.ContractNonceHistoryAtBlockKey(contractAddress, height)
	return w.Put(key, oldValue.Marshal())
}

// DeleteContractNonceHistory deletes the history at the given height
func DeleteContractNonceHistory(
	w db.KeyValueWriter,
	contractAddress *felt.Felt,
	height uint64,
) error {
	key := db.ContractNonceHistoryAtBlockKey(contractAddress, height)
	return w.Delete(key)
}

func WriteContractClassHashHistory(
	w db.KeyValueWriter,
	contractAddress,
	oldValue *felt.Felt,
	height uint64,
) error {
	key := db.ContractClassHashHistoryAtBlockKey(contractAddress, height)
	return w.Put(key, oldValue.Marshal())
}

func DeleteContractClassHashHistory(
	w db.KeyValueWriter,
	contractAddress *felt.Felt,
	height uint64,
) error {
	key := db.ContractClassHashHistoryAtBlockKey(contractAddress, height)
	return w.Delete(key)
}

/**
 Chain-related accessors

 ██████ ██   ██  █████  ██ ███    ██
██      ██   ██ ██   ██ ██ ████   ██
██      ███████ ███████ ██ ██ ██  ██
██      ██   ██ ██   ██ ██ ██  ██ ██
 ██████ ██   ██ ██   ██ ██ ██   ████

 **/

func GetL1Head(r db.KeyValueReader) (L1Head, error) {
	var l1Head L1Head
	err := r.Get(db.L1Height.Key(), func(data []byte) error {
		return encoder.Unmarshal(data, &l1Head)
	})
	return l1Head, err
}

func WriteL1Head(w db.KeyValueWriter, l1Head *L1Head) error {
	data, err := encoder.Marshal(l1Head)
	if err != nil {
		return err
	}
	return w.Put(db.L1Height.Key(), data)
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

func WriteBlockHeaderByNumber(w db.KeyValueWriter, header *Header) error {
	data, err := encoder.Marshal(header)
	if err != nil {
		return err
	}
	return w.Put(db.BlockHeaderByNumberKey(header.Number), data)
}

func DeleteBlockHeaderByNumber(w db.KeyValueWriter, number uint64) error {
	return w.Delete(db.BlockHeaderByNumberKey(number))
}

func GetBlockCommitmentByBlockNum(r db.KeyValueReader, blockNum uint64) (*BlockCommitments, error) {
	var commitment *BlockCommitments
	err := r.Get(db.BlockCommitmentsKey(blockNum), func(data []byte) error {
		return encoder.Unmarshal(data, &commitment)
	})
	return commitment, err
}

func WriteBlockCommitment(w db.KeyValueWriter, blockNum uint64, commitment *BlockCommitments) error {
	data, err := encoder.Marshal(commitment)
	if err != nil {
		return err
	}
	return w.Put(db.BlockCommitmentsKey(blockNum), data)
}

func DeleteBlockCommitment(w db.KeyValueWriter, blockNum uint64) error {
	return w.Delete(db.BlockCommitmentsKey(blockNum))
}

func GetL1HandlerTxnHashByMsgHash(r db.KeyValueReader, msgHash []byte) (felt.Felt, error) {
	var l1HandlerTxnHash felt.Felt
	err := r.Get(db.L1HandlerTxnHashByMsgHashKey(msgHash), func(data []byte) error {
		l1HandlerTxnHash.Unmarshal(data)
		return nil
	})
	return l1HandlerTxnHash, err
}

func WriteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte, l1HandlerTxnHash *felt.Felt) error {
	return w.Put(db.L1HandlerTxnHashByMsgHashKey(msgHash), l1HandlerTxnHash.Marshal())
}

func DeleteL1HandlerTxnHashByMsgHash(w db.KeyValueWriter, msgHash []byte) error {
	return w.Delete(db.L1HandlerTxnHashByMsgHashKey(msgHash))
}

func WriteL1HandlerMsgHashes(w db.KeyValueWriter, txns []Transaction) error {
	for _, txn := range txns {
		if l1Handler, ok := txn.(*L1HandlerTransaction); ok {
			err := WriteL1HandlerTxnHashByMsgHash(
				w,
				l1Handler.MessageHash(),
				l1Handler.Hash(),
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func GetChainHeight(r db.KeyValueReader) (uint64, error) {
	var height uint64
	err := r.Get(db.ChainHeight.Key(), func(data []byte) error {
		height = binary.BigEndian.Uint64(data)
		return nil
	})
	return height, err
}

func WriteChainHeight(w db.KeyValueWriter, height uint64) error {
	return w.Put(db.ChainHeight.Key(), MarshalBlockNumber(height))
}

func DeleteChainHeight(w db.KeyValueWriter) error {
	return w.Delete(db.ChainHeight.Key())
}

func GetBlockHeaderByNumber(r db.KeyValueReader, blockNum uint64) (*Header, error) {
	var header *Header
	err := r.Get(db.BlockHeaderByNumberKey(blockNum), func(data []byte) error {
		return encoder.Unmarshal(data, &header)
	})
	return header, err
}

func GetBlockHeaderByHash(r db.KeyValueReader, hash *felt.Felt) (*Header, error) {
	var blockNum uint64
	err := r.Get(db.BlockHeaderNumbersByHashKey(hash), func(data []byte) error {
		blockNum = binary.BigEndian.Uint64(data)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return GetBlockHeaderByNumber(r, blockNum)
}

func WriteBlockHeader(w db.KeyValueWriter, header *Header) error {
	if err := WriteBlockHeaderNumberByHash(w, header.Hash, header.Number); err != nil {
		return err
	}

	return WriteBlockHeaderByNumber(w, header)
}

func GetAggregatedBloomFilter(r db.KeyValueReader, fromBlock, toBLock uint64) (AggregatedBloomFilter, error) {
	var filter AggregatedBloomFilter
	err := r.Get(db.AggregatedBloomFilterKey(fromBlock, toBLock), func(data []byte) error {
		err := encoder.Unmarshal(data, &filter)
		return err
	})
	if err != nil {
		return AggregatedBloomFilter{}, err
	}

	return filter, nil
}

func WriteAggregatedBloomFilter(w db.KeyValueWriter, filter *AggregatedBloomFilter) error {
	enc, err := encoder.Marshal(filter)
	if err != nil {
		return err
	}
	return w.Put(db.AggregatedBloomFilterKey(filter.FromBlock(), filter.ToBlock()), enc)
}

func GetRunningEventFilter(r db.KeyValueReader) (*RunningEventFilter, error) {
	var filter RunningEventFilter
	err := r.Get(db.RunningEventFilter.Key(), func(data []byte) error {
		err := encoder.Unmarshal(data, &filter)
		return err
	})
	if err != nil {
		return nil, err
	}

	return &filter, nil
}

func WriteRunningEventFilter(w db.KeyValueWriter, filter *RunningEventFilter) error {
	enc, err := encoder.Marshal(filter)
	if err != nil {
		return err
	}

	return w.Put(db.RunningEventFilter.Key(), enc)
}

func GetClassCasmHashMetadata(
	r db.KeyValueReader,
	classHash *felt.SierraClassHash,
) (ClassCasmHashMetadata, error) {
	return ClassCasmHashMetadataBucket.Get(r, classHash)
}

func WriteClassCasmHashMetadata(
	w db.KeyValueWriter,
	classHash *felt.SierraClassHash,
	metadata *ClassCasmHashMetadata,
) error {
	return ClassCasmHashMetadataBucket.Put(
		w,
		classHash,
		metadata,
	)
}

func DeleteClassCasmHashMetadata(
	w db.KeyValueWriter,
	classHash *felt.SierraClassHash,
) error {
	return ClassCasmHashMetadataBucket.Delete(
		w,
		classHash,
	)
}
