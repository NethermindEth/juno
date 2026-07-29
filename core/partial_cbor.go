package core

import (
	"github.com/NethermindEth/juno/core/felt"
	bloom "github.com/bits-and-blooms/bloom/v3"
)

// discardedCBOR is a no-op unmarshaler for fields a partial projection does not want.
//
// A record (header, receipt, transaction) is stored as a CBOR map keyed by field name. When the
// decoder unmarshals such a map into a struct, it allocates a Go string for every map key that has
// no matching struct field — the string feeds duplicate-key detection that is a no-op in our
// decode mode but is still built unconditionally at the call site (fxamacker/cbor
// decode.go:2800). A projection that names only the one field it wants therefore pays one string
// allocation per skipped key, per record — and that scales with the number of records.
//
// Naming every field and giving the unwanted ones this type makes every wire key match a field, so
// the allocating unmatched-key path is never taken. A field whose type implements Unmarshaler is
// handled by fxamacker with a plain byte-range skip over a non-copied sub-slice (decode.go:1863),
// so discarding costs a structural walk with no allocation and no reflection.
type discardedCBOR struct{}

func (discardedCBOR) UnmarshalCBOR([]byte) error { return nil }

// discardedHeaderSkeleton names every field of Header as discarded. Partial header projections
// embed it and shadow the one field they want with a typed field of the same name: the shadowing
// field (shallower depth) wins for that key, every other key still matches a skeleton field, and
// no key falls through to the allocating unmatched-key path. A reflection test asserts this
// skeleton's field set stays identical to Header's, so a new Header field fails the build's test
// rather than silently regressing allocations.
type discardedHeaderSkeleton struct {
	Hash             discardedCBOR
	ParentHash       discardedCBOR
	Number           discardedCBOR
	GlobalStateRoot  discardedCBOR
	SequencerAddress discardedCBOR
	TransactionCount discardedCBOR
	EventCount       discardedCBOR
	Timestamp        discardedCBOR
	ProtocolVersion  discardedCBOR
	EventsBloom      discardedCBOR
	L1GasPriceETH    discardedCBOR `cbor:"gasprice"`
	Signatures       discardedCBOR
	L1GasPriceSTRK   discardedCBOR `cbor:"gaspricestrk"`
	L1DAMode         discardedCBOR
	L1DataGasPrice   discardedCBOR
	L2GasPrice       discardedCBOR
}

type headerHashProjection struct {
	discardedHeaderSkeleton
	Hash *felt.Felt
}

type headerGlobalStateRootProjection struct {
	discardedHeaderSkeleton
	GlobalStateRoot *felt.Felt
}

type headerTransactionCountProjection struct {
	discardedHeaderSkeleton
	TransactionCount uint64
}

type headerTimestampProjection struct {
	discardedHeaderSkeleton
	Timestamp *uint64
}

type headerEventsBloomProjection struct {
	discardedHeaderSkeleton
	EventsBloom *bloom.BloomFilter
}

type headerHashAndStateRootProjection struct {
	discardedHeaderSkeleton
	Hash            *felt.Felt
	GlobalStateRoot *felt.Felt
}
