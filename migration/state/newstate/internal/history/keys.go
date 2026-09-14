package history

import (
	"fmt"

	"github.com/NethermindEth/juno/core/deprecatedstate"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

const blockNumberLen = 8

// Key sizes. These mirror the production schema, and the deprecated layout of
// each bucket is identical apart from the leading bucket byte:
//
//	address   : [bucket][addr:32]                    = 33
//	leaf      : [bucket][addr:32][height:1]          = 34
//	block     : [bucket][addr:32][block:8]           = 41  (nonce, class hash)
//	storage   : [bucket][addr:32][slot:32][block:8]  = 73
const (
	addressKeyLen        = 1 + felt.Bytes
	leafPrefixLen        = addressKeyLen + 1
	blockKeyLen          = 1 + felt.Bytes + blockNumberLen
	storageHistoryKeyLen = 1 + 2*felt.Bytes + blockNumberLen
)

// Offsets into a storage history key.
const (
	slotOffset  = 1 + felt.Bytes
	blockOffset = slotOffset + felt.Bytes
)

// The two forms core/state writes a Contract record in. Both start with the
// nonce then the class hash and end with the deploy height.
const (
	contractRecordEmptyRootLen = 2*felt.Bytes + blockNumberLen
	contractRecordFullLen      = 3*felt.Bytes + blockNumberLen
)

// zeroValue backs entries whose slot has no head-trie leaf, i.e. was zeroed out.
var zeroValue [felt.Bytes]byte

// historyScratch holds the reusable byte buffers for one worker's walk over a
// contract. Each worker owns one, allocated once at ingestor construction.
//
// key is overwritten on every row and consumed via batch.Put before the next
// row touches it. The prefixes are overwritten once per contract, after the
// previous contract's iterators are closed; pebble keeps an iterator's bound for
// its lifetime, which is why each iterator gets its own buffer rather than
// sharing one.
type historyScratch struct {
	key              [storageHistoryKeyLen]byte // largest key; slice to the walk's length
	deprecatedPrefix [addressKeyLen]byte
	leafPrefix       [leafPrefixLen]byte
	contractKey      [addressKeyLen]byte
}

// fillAddressKey writes [bucket][addr:32] into dst and returns it.
func fillAddressKey(dst []byte, bucket db.Bucket, addr *felt.Address) []byte {
	dst[0] = byte(bucket)
	addrBytes := addr.Bytes()
	copy(dst[1:], addrBytes[:])
	return dst
}

// fillLeafPrefix writes [ContractStorage][addr:32][height:1] into dst and
// returns it: the prefix under which a contract's head storage trie stores its
// leaves.
func fillLeafPrefix(dst []byte, addr *felt.Address) []byte {
	fillAddressKey(dst, db.ContractStorage, addr)
	dst[addressKeyLen] = deprecatedstate.ContractStorageTrieHeight
	return dst
}

// fillBlockHistoryKey writes [bucket][addr:32][block:8] into dst and returns it.
func fillBlockHistoryKey(dst []byte, bucket db.Bucket, addr *felt.Address, blockBE []byte) []byte {
	fillAddressKey(dst, bucket, addr)
	copy(dst[addressKeyLen:], blockBE)
	return dst
}

// fillHistoryKeyFrom writes [bucket][deprecatedKey[1:]] into dst. A deprecated
// row moves to its new bucket with a one-byte change, since both layouts encode
// every other field identically.
//
// The length check is not separable from the copy: copy is sized by dst, so a
// short key would silently leave the previous row's bytes in place and write the
// entry under the wrong block.
func fillHistoryKeyFrom(dst []byte, bucket db.Bucket, deprecatedKey []byte) error {
	if len(deprecatedKey) != len(dst) {
		return fmt.Errorf(
			"malformed deprecated history key: length %d, want %d", len(deprecatedKey), len(dst),
		)
	}
	dst[0] = byte(bucket)
	copy(dst[1:], deprecatedKey[1:])
	return nil
}

// checkContractRecord accepts either form core/state writes.
func checkContractRecord(data []byte) error {
	if len(data) != contractRecordEmptyRootLen && len(data) != contractRecordFullLen {
		return fmt.Errorf(
			"malformed contract record: length %d, want %d or %d",
			len(data), contractRecordEmptyRootLen, contractRecordFullLen,
		)
	}
	return nil
}

// readHeadNonce returns the nonce in effect now, read from the Contract record
// at contractKey. It closes the nonce register: the history never stored a
// value for blocks past the last change.
func readHeadNonce(r db.KeyValueReader, contractKey []byte) ([felt.Bytes]byte, error) {
	var nonce [felt.Bytes]byte
	err := r.Get(contractKey, func(data []byte) error {
		if err := checkContractRecord(data); err != nil {
			return err
		}
		copy(nonce[:], data[:felt.Bytes])
		return nil
	})
	return nonce, err
}

// readHeadClassHash returns the class hash in effect now, which closes the
// class-hash register, and the deploy height, which is the block the register
// starts at because the new layout adds a row there.
func readHeadClassHash(r db.KeyValueReader, contractKey []byte) (
	classHash [felt.Bytes]byte, deployHeight [blockNumberLen]byte, err error,
) {
	err = r.Get(contractKey, func(data []byte) error {
		if err := checkContractRecord(data); err != nil {
			return err
		}
		copy(classHash[:], data[felt.Bytes:2*felt.Bytes])
		copy(deployHeight[:], data[len(data)-blockNumberLen:])
		return nil
	})
	return classHash, deployHeight, err
}
