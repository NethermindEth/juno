package core

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

var ErrCheckHeadState = errors.New("check head state")

type history struct {
	txn db.SnapshotBatch
}

func logDBKey(key []byte, height uint64) []byte {
	return binary.BigEndian.AppendUint64(key, height)
}

func (h *history) logOldValue(key, value []byte, height uint64) error {
	return h.txn.Put(logDBKey(key, height), value)
}

func (h *history) deleteLog(key []byte, height uint64) error {
	return h.txn.Delete(logDBKey(key, height))
}

func (h *history) valueAt(key []byte, height uint64) ([]byte, error) {
	it, err := h.txn.NewIterator(nil, false)
	if err != nil {
		return nil, err
	}

	for it.Seek(logDBKey(key, height)); it.Valid(); it.Next() {
		seekedKey := it.Key()
		// seekedKey size should be `len(key) + sizeof(uint64)` and seekedKey should match key prefix
		if len(seekedKey) != len(key)+8 || !bytes.HasPrefix(seekedKey, key) {
			break
		}

		seekedHeight := binary.BigEndian.Uint64(seekedKey[len(key):])
		if seekedHeight < height {
			// last change happened before the height we are looking for
			// check head state
			break
		} else if seekedHeight == height {
			// a log exists for the height we are looking for, so the old value in this log entry is not useful.
			// advance the iterator and see we can use the next entry. If not, ErrCheckHeadState will be returned
			continue
		}

		val, itErr := it.Value()
		if err = utils.RunAndWrapOnError(it.Close, itErr); err != nil {
			return nil, err
		}
		// seekedHeight > height
		return val, nil
	}

	return nil, utils.RunAndWrapOnError(it.Close, ErrCheckHeadState)
}

// LogContractStorage logs the old value of a storage location for the given contract which changed on height `height`
func (h *history) LogContractStorage(contractAddress, storageLocation, oldValue *felt.Felt, height uint64) error {
	key := db.ContractStorageHistoryKey(contractAddress, storageLocation)
	return h.logOldValue(key, oldValue.Marshal(), height)
}

// DeleteContractStorageLog deletes the log at the given height
func (h *history) DeleteContractStorageLog(contractAddress, storageLocation *felt.Felt, height uint64) error {
	return h.deleteLog(db.ContractStorageHistoryKey(contractAddress, storageLocation), height)
}

// ContractStorageAt returns the value of a storage location of the given contract at the height `height`
func (h *history) ContractStorageAt(
	contractAddress,
	storageLocation *felt.Felt,
	height uint64,
) (felt.Felt, error) {
	key := db.ContractStorageHistoryKey(contractAddress, storageLocation)
	value, err := h.valueAt(key, height)
	if err != nil {
		return felt.Felt{}, err
	}

	return felt.FromBytes[felt.Felt](value), nil
}

func (h *history) LogContractNonce(contractAddress, oldValue *felt.Felt, height uint64) error {
	return h.logOldValue(db.ContractNonceHistoryKey(contractAddress), oldValue.Marshal(), height)
}

func (h *history) DeleteContractNonceLog(contractAddress *felt.Felt, height uint64) error {
	return h.deleteLog(db.ContractNonceHistoryKey(contractAddress), height)
}

func (h *history) ContractNonceAt(contractAddress *felt.Felt, height uint64) (felt.Felt, error) {
	key := db.ContractNonceHistoryKey(contractAddress)
	value, err := h.valueAt(key, height)
	if err != nil {
		return felt.Felt{}, err
	}
	return felt.FromBytes[felt.Felt](value), nil
}

func (h *history) LogContractClassHash(contractAddress, oldValue *felt.Felt, height uint64) error {
	return h.logOldValue(db.ContractClassHashHistoryKey(contractAddress), oldValue.Marshal(), height)
}

func (h *history) DeleteContractClassHashLog(contractAddress *felt.Felt, height uint64) error {
	return h.deleteLog(db.ContractClassHashHistoryKey(contractAddress), height)
}

func (h *history) ContractClassHashAt(
	contractAddress *felt.Felt,
	height uint64,
) (felt.Felt, error) {
	key := db.ContractClassHashHistoryKey(contractAddress)
	value, err := h.valueAt(key, height)
	if err != nil {
		return felt.Felt{}, err
	}

	return felt.FromBytes[felt.Felt](value), nil
}
