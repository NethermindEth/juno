package deprecatedstate

import (
	"encoding/binary"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// Accessors for the deprecated per-field contract layout and pre-value history buckets.

func GetContractClassHash(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var classHash felt.Felt
	//nolint:staticcheck,nolintlint // old state layout
	err := r.Get(db.DeprecatedContractClassHashKey(addr), func(data []byte) error {
		classHash.SetBytes(data)
		return nil
	})
	return classHash, err
}

func WriteContractClassHash(w db.KeyValueWriter, addr, classHash *felt.Felt) error {
	//nolint:staticcheck,nolintlint // old state layout
	return w.Put(db.DeprecatedContractClassHashKey(addr), classHash.Marshal())
}

func GetContractNonce(r db.KeyValueReader, addr *felt.Felt) (felt.Felt, error) {
	var nonce felt.Felt
	//nolint:staticcheck,nolintlint // old state layout
	err := r.Get(db.DeprecatedContractNonceKey(addr), func(data []byte) error {
		nonce.SetBytes(data)
		return nil
	})
	return nonce, err
}

func WriteContractNonce(w db.KeyValueWriter, addr, nonce *felt.Felt) error {
	//nolint:staticcheck,nolintlint // old state layout
	return w.Put(db.DeprecatedContractNonceKey(addr), nonce.Marshal())
}

func WriteContractDeploymentHeight(w db.KeyValueWriter, addr *felt.Felt, height uint64) error {
	enc := core.MarshalBlockNumber(height)
	//nolint:staticcheck,nolintlint // old state layout
	return w.Put(db.DeprecatedContractDeploymentHeightKey(addr), enc)
}

func GetContractDeploymentHeight(r db.KeyValueReader, addr *felt.Felt) (uint64, error) {
	var height uint64
	//nolint:staticcheck,nolintlint // old state layout
	err := r.Get(db.DeprecatedContractDeploymentHeightKey(addr), func(data []byte) error {
		height = binary.BigEndian.Uint64(data)
		return nil
	})
	return height, err
}

func DeleteContractDeploymentHeight(w db.KeyValueWriter, addr *felt.Felt) error {
	//nolint:staticcheck,nolintlint // old state layout
	return w.Delete(db.DeprecatedContractDeploymentHeightKey(addr))
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
	//nolint:staticcheck,nolintlint // old state layout
	key := db.DeprecatedContractStorageHistoryAtBlockKey(contractAddress, storageLocation, height)
	return w.Put(key, oldValue.Marshal())
}

// DeleteContractStorageHistory deletes the history at the given height
func DeleteContractStorageHistory(
	w db.KeyValueWriter,
	contractAddress,
	storageLocation *felt.Felt,
	height uint64,
) error {
	//nolint:staticcheck,nolintlint // old state layout
	key := db.DeprecatedContractStorageHistoryAtBlockKey(contractAddress, storageLocation, height)
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
	//nolint:staticcheck,nolintlint // old state layout
	key := db.DeprecatedContractNonceHistoryAtBlockKey(contractAddress, height)
	return w.Put(key, oldValue.Marshal())
}

// DeleteContractNonceHistory deletes the history at the given height
func DeleteContractNonceHistory(
	w db.KeyValueWriter,
	contractAddress *felt.Felt,
	height uint64,
) error {
	//nolint:staticcheck,nolintlint // old state layout
	key := db.DeprecatedContractNonceHistoryAtBlockKey(contractAddress, height)
	return w.Delete(key)
}

func WriteContractClassHashHistory(
	w db.KeyValueWriter,
	contractAddress,
	oldValue *felt.Felt,
	height uint64,
) error {
	//nolint:staticcheck,nolintlint // old state layout
	key := db.DeprecatedContractClassHashHistoryAtBlockKey(contractAddress, height)
	return w.Put(key, oldValue.Marshal())
}

func DeleteContractClassHashHistory(
	w db.KeyValueWriter,
	contractAddress *felt.Felt,
	height uint64,
) error {
	//nolint:staticcheck,nolintlint // old state layout
	key := db.DeprecatedContractClassHashHistoryAtBlockKey(contractAddress, height)
	return w.Delete(key)
}
