package core

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
	bloom "github.com/bits-and-blooms/bloom/v3"
)

// discardedCBOR is a no-op unmarshaler for fields a partial projection does not want.
//
// When fxamacker decodes a CBOR map into a struct, every map key with no matching field takes an
// unmatched-key path that allocates ~2 objects per key: a Go string of the key name, plus boxing it
// into an interface. A projection naming only the wanted field pays that for every skipped key, per
// record. Naming every field and discarding the unwanted ones makes every key match, so that path
// is never taken; a discarded field costs only a byte-range skip with no allocation.
type discardedCBOR struct{}

// errDiscardedCBORMarshal is returned when a decode-only projection is marshaled.
var errDiscardedCBORMarshal = errors.New(
	"core: partial CBOR projection is decode-only and must not be marshaled",
)

func (discardedCBOR) UnmarshalCBOR([]byte) error { return nil }

// MarshalCBOR refuses to encode: projections are decode-only, and marshaling one would emit its
// discarded fields as empty maps, silently corrupting the record.
func (discardedCBOR) MarshalCBOR() ([]byte, error) {
	return nil, errDiscardedCBORMarshal
}

// discardedHeaderSkeleton names every Header field as discarded. A partial projection embeds it and
// shadows the field it wants with a typed field of the same name (shallower depth wins the key), so
// every other key still matches and none hits the unmatched-key path. A shadow of a tagged field
// (e.g. L1GasPriceETH `cbor:"gasprice"`) must repeat the tag, or it keys on the Go field name.
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

// discardedReceiptSkeleton names every field of TransactionReceipt as discarded. Receipt
// projections embed it and shadow the fields they want with typed fields of the same name (see
// [discardedCBOR] for why every field must be named). A reflection test asserts this skeleton's
// field set stays identical to TransactionReceipt's.
type discardedReceiptSkeleton struct {
	Fee                discardedCBOR
	FeeUnit            discardedCBOR
	Events             discardedCBOR
	ExecutionResources discardedCBOR
	L1ToL2Message      discardedCBOR
	L2ToL1Message      discardedCBOR
	TransactionHash    discardedCBOR
	Reverted           discardedCBOR
	RevertReason       discardedCBOR
}

type receiptExecutionStatusProjection struct {
	discardedReceiptSkeleton
	Reverted     bool
	RevertReason string
}
