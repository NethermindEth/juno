package historyprunner

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

// migrationScratchTag is the leading byte of every history-pruner scratch
// key. We reuse db.Temporary, the canonical scratch bucket also used by
// deprecated migrations. Production iterators scoped to a single-byte bucket
// prefix (e.g. NewIterator([]byte{14}, ...) for ContractStorageHistory)
// cannot observe scratch keys.
var migrationScratchTag = byte(db.Temporary)

// Scratch keys carry their source bucket tag as byte 1, making them
// self-describing:
//
//	scratchKey = [Temporary][originalBucketTag][...rest of original key after
//	                                            its own bucket tag byte]
//
// Concretely:
//
//	storage   : [Temporary][14][addr:32][slot:32][block:8]   = 74 bytes
//	nonce     : [Temporary][15][addr:32][block:8]            = 42 bytes
//	classhash : [Temporary][16][addr:32][block:8]            = 42 bytes
//
// Phase 2 (scratch → history) needs no out-of-band mapping: read the
// original tag from byte 1 of the scratch key, prepend it to the rest, done.
const scratchPrefixLen = 2

const blockNumberSuffixLen = 8

// History-bucket key sizes. These mirror the production schema:
//
//	storage   : [bucket][addr:32][slot:32][block:8] = 73
//	nonce     : [bucket][addr:32][block:8]          = 41
//	classhash : [bucket][addr:32][block:8]          = 41
const (
	storageHistoryKeyLen   = 1 + 2*felt.Bytes + blockNumberSuffixLen
	nonceHistoryKeyLen     = 1 + felt.Bytes + blockNumberSuffixLen
	classHashHistoryKeyLen = 1 + felt.Bytes + blockNumberSuffixLen

	// historyKeyBufLen sizes a buffer big enough for any history-bucket key
	// in this migration — caller can reuse one buffer across all three.
	historyKeyBufLen = storageHistoryKeyLen

	// historyValueLen is the felt-sized payload of every history bucket.
	historyValueLen = felt.Bytes

	storageScratchKeyLen   = 1 + storageHistoryKeyLen
	nonceScratchKeyLen     = 1 + nonceHistoryKeyLen
	classHashScratchKeyLen = 1 + classHashHistoryKeyLen

	// scratchKeyBufLen sizes a buffer big enough for any scratch key —
	// callers can reuse one buffer across all three sub-namespaces.
	scratchKeyBufLen = storageScratchKeyLen
)

// fillStorageScratchKey writes [Temporary][14][addr:32][slot:32][block:8] into
// dst and returns dst[:74]. dst must have cap ≥ storageScratchKeyLen.
func fillStorageScratchKey(
	buf []byte,
	addr, slot *felt.Felt,
	blockBE [blockNumberSuffixLen]byte,
) []byte {
	buf[0] = migrationScratchTag
	buf[1] = byte(db.ContractStorageHistory)
	addrBytes := addr.Bytes()
	copy(buf[scratchPrefixLen:], addrBytes[:])
	slotBytes := slot.Bytes()
	copy(buf[scratchPrefixLen+felt.Bytes:], slotBytes[:])
	copy(buf[scratchPrefixLen+2*felt.Bytes:], blockBE[:])
	return buf[:storageScratchKeyLen]
}

// fillNonceScratchKey writes [Temporary][15][addr:32][block:8] into dst.
func fillNonceScratchKey(
	buf []byte,
	addr *felt.Felt,
	blockBE [blockNumberSuffixLen]byte,
) []byte {
	buf[0] = migrationScratchTag
	buf[1] = byte(db.ContractNonceHistory)
	addrBytes := addr.Bytes()
	copy(buf[scratchPrefixLen:], addrBytes[:])
	copy(buf[scratchPrefixLen+felt.Bytes:], blockBE[:])
	return buf[:nonceScratchKeyLen]
}

// fillClassHashScratchKey writes [Temporary][16][addr:32][block:8] into dst.
func fillClassHashScratchKey(
	buf []byte,
	addr *felt.Felt,
	blockBE [blockNumberSuffixLen]byte,
) []byte {
	buf[0] = migrationScratchTag
	buf[1] = byte(db.ContractClassHashHistory)
	addrBytes := addr.Bytes()
	copy(buf[scratchPrefixLen:], addrBytes[:])
	copy(buf[scratchPrefixLen+felt.Bytes:], blockBE[:])
	return buf[:classHashScratchKeyLen]
}

// fillStorageHistoryKey writes [bucket][addr:32][slot:32][block:8] into dst.
func fillStorageHistoryKey(
	buf []byte,
	addr, slot *felt.Felt,
	blockBE [blockNumberSuffixLen]byte,
) []byte {
	buf[0] = byte(db.ContractStorageHistory)
	addrBytes := addr.Bytes()
	copy(buf[1:], addrBytes[:])
	slotBytes := slot.Bytes()
	copy(buf[1+felt.Bytes:], slotBytes[:])
	copy(buf[1+2*felt.Bytes:], blockBE[:])
	return buf[:storageHistoryKeyLen]
}

// fillNonceHistoryKey writes [bucket][addr:32][block:8] into dst.
func fillNonceHistoryKey(
	buf []byte,
	addr *felt.Felt,
	blockBE [blockNumberSuffixLen]byte,
) []byte {
	buf[0] = byte(db.ContractNonceHistory)
	addrBytes := addr.Bytes()
	copy(buf[1:], addrBytes[:])
	copy(buf[1+felt.Bytes:], blockBE[:])
	return buf[:nonceHistoryKeyLen]
}

// fillClassHashHistoryKey writes [bucket][addr:32][block:8] into dst.
func fillClassHashHistoryKey(
	buf []byte,
	addr *felt.Felt,
	blockBE [blockNumberSuffixLen]byte,
) []byte {
	buf[0] = byte(db.ContractClassHashHistory)
	addrBytes := addr.Bytes()
	copy(buf[1:], addrBytes[:])
	copy(buf[1+felt.Bytes:], blockBE[:])
	return buf[:classHashHistoryKeyLen]
}
