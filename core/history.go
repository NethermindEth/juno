package core

import (
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

var ErrCheckHeadState = errors.New("check head state")

type history struct {
	txn db.Transaction
}

func logDBKey(key []byte, height uint64) []byte {
	return binary.BigEndian.AppendUint64(key, height)
}

func (h *history) logOldValue(key, value []byte, height uint64) error {
	return h.txn.Set(logDBKey(key, height), value)
}

func (h *history) deleteLog(key []byte, height uint64) error {
	return h.txn.Delete(logDBKey(key, height))
}

// valueAt returns the old value at the given height for the given key if it exists
func (h *history) valueAt(key []byte, height uint64) ([]byte, error) {
	var value []byte
	err := h.txn.Get(logDBKey(key, height), func(val []byte) error {
		value = val
		return nil
	})
	if err != nil {
		return nil, ErrCheckHeadState
	}

	return value, nil
}

func storageLogKey(contractAddress, storageLocation *felt.Felt) []byte {
	return db.ContractStorageHistory.Key(contractAddress.Marshal(), storageLocation.Marshal())
}

// LogContractStorage logs the old value of a storage location for the given contract which changed on height `height`
func (h *history) LogContractStorage(contractAddress, storageLocation, oldValue *felt.Felt, height uint64) error {
	key := storageLogKey(contractAddress, storageLocation)
	return h.logOldValue(key, oldValue.Marshal(), height)
}

// DeleteContractStorageLog deletes the log at the given height
func (h *history) DeleteContractStorageLog(contractAddress, storageLocation *felt.Felt, height uint64) error {
	return h.deleteLog(storageLogKey(contractAddress, storageLocation), height)
}

// ContractStorageAt returns the old value of a storage location for the given contract at the given height
func (h *history) ContractStorageAt(contractAddress, storageLocation *felt.Felt, height uint64) (*felt.Felt, error) {
	key := storageLogKey(contractAddress, storageLocation)
	value, err := h.valueAt(key, height)
	if err != nil {
		return nil, err
	}

	return new(felt.Felt).SetBytes(value), nil
}

func nonceLogKey(contractAddress *felt.Felt) []byte {
	return db.ContractNonceHistory.Key(contractAddress.Marshal())
}

func (h *history) LogContractNonce(contractAddress, oldValue *felt.Felt, height uint64) error {
	return h.logOldValue(nonceLogKey(contractAddress), oldValue.Marshal(), height)
}

func (h *history) DeleteContractNonceLog(contractAddress *felt.Felt, height uint64) error {
	return h.deleteLog(nonceLogKey(contractAddress), height)
}

func (h *history) ContractNonceAt(contractAddress *felt.Felt, height uint64) (*felt.Felt, error) {
	key := nonceLogKey(contractAddress)
	value, err := h.valueAt(key, height)
	if err != nil {
		return nil, err
	}

	return new(felt.Felt).SetBytes(value), nil
}

func classHashLogKey(contractAddress *felt.Felt) []byte {
	return db.ContractClassHashHistory.Key(contractAddress.Marshal())
}

func (h *history) LogContractClassHash(contractAddress, oldValue *felt.Felt, height uint64) error {
	return h.logOldValue(classHashLogKey(contractAddress), oldValue.Marshal(), height)
}

func (h *history) DeleteContractClassHashLog(contractAddress *felt.Felt, height uint64) error {
	return h.deleteLog(classHashLogKey(contractAddress), height)
}

func (h *history) ContractClassHashAt(contractAddress *felt.Felt, height uint64) (*felt.Felt, error) {
	key := classHashLogKey(contractAddress)
	value, err := h.valueAt(key, height)
	if err != nil {
		return nil, err
	}

	return new(felt.Felt).SetBytes(value), nil
}
