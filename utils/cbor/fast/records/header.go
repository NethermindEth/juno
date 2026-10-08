package records

import (
	"encoding/binary"
	"errors"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/bits-and-blooms/bloom/v3"
)

const (
	headerHash = iota
	headerParentHash
	headerNumber
	headerGlobalStateRoot
	headerSequencerAddress
	headerTransactionCount
	headerEventCount
	headerTimestamp
	headerProtocolVersion
	headerEventsBloom
	headerL1GasPriceETH
	headerSignatures
	headerL1GasPriceSTRK
	headerL1DAMode
	headerL1DataGasPrice
	headerL2GasPrice
)

var headerFields = fieldsOf[core.Header]([]string{
	headerHash:             "Hash",
	headerParentHash:       "ParentHash",
	headerNumber:           "Number",
	headerGlobalStateRoot:  "GlobalStateRoot",
	headerSequencerAddress: "SequencerAddress",
	headerTransactionCount: "TransactionCount",
	headerEventCount:       "EventCount",
	headerTimestamp:        "Timestamp",
	headerProtocolVersion:  "ProtocolVersion",
	headerEventsBloom:      "EventsBloom",
	headerL1GasPriceETH:    "gasprice",
	headerSignatures:       "Signatures",
	headerL1GasPriceSTRK:   "gaspricestrk",
	headerL1DAMode:         "L1DAMode",
	headerL1DataGasPrice:   "L1DataGasPrice",
	headerL2GasPrice:       "L2GasPrice",
})

// ReadHeader reads a stored block header. The keys of the legacy fields GasPrice,
// GasPriceSTRK and ExtraData are skipped, as fxamacker skips them.
func ReadHeader(r *fast.Reader, dst *core.Header) error {
	s, err := r.OpenStruct(headerFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case headerHash:
			err = readFelt(r, &dst.Hash)
		case headerParentHash:
			err = readFelt(r, &dst.ParentHash)
		case headerNumber:
			err = readUint64(r, &dst.Number)
		case headerGlobalStateRoot:
			err = readFelt(r, &dst.GlobalStateRoot)
		case headerSequencerAddress:
			err = readFelt(r, &dst.SequencerAddress)
		case headerTransactionCount:
			err = readUint64(r, &dst.TransactionCount)
		case headerEventCount:
			err = readUint64(r, &dst.EventCount)
		case headerTimestamp:
			err = readUint64(r, &dst.Timestamp)
		case headerProtocolVersion:
			err = readString(r, &dst.ProtocolVersion)
		case headerEventsBloom:
			err = readBloom(r, &dst.EventsBloom)
		case headerL1GasPriceETH:
			err = readFelt(r, &dst.L1GasPriceETH)
		case headerSignatures:
			err = readSlice(r, &dst.Signatures, readFeltPtrSlice)
		case headerL1GasPriceSTRK:
			err = readFelt(r, &dst.L1GasPriceSTRK)
		case headerL1DAMode:
			var mode uint64
			err = readUint64(r, &mode)
			dst.L1DAMode = core.L1DAMode(mode)
		case headerL1DataGasPrice:
			err = readPtr(r, &dst.L1DataGasPrice, readGasPrice)
		case headerL2GasPrice:
			err = readPtr(r, &dst.L2GasPrice, readGasPrice)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	gasPriceInWei = iota
	gasPriceInFri
)

var gasPriceFields = fieldsOf[core.GasPrice]([]string{
	gasPriceInWei: "PriceInWei",
	gasPriceInFri: "PriceInFri",
})

func readGasPrice(r *fast.Reader, dst *core.GasPrice) error {
	s, err := r.OpenStruct(gasPriceFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case gasPriceInWei:
			err = readFelt(r, &dst.PriceInWei)
		case gasPriceInFri:
			err = readFelt(r, &dst.PriceInFri)
		default:
			err = r.Skip()
		}
	}
	return err
}

// readBloom reads a bloom filter, stored as its binary form in a byte string.
// UnmarshalBinary copies what it keeps.
func readBloom(r *fast.Reader, dst **bloom.BloomFilter) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	data, err := r.BytesNoCopy()
	if err != nil {
		return err
	}
	if !bloomSizeMatches(data) {
		return ErrBloomSize
	}
	filter := new(bloom.BloomFilter)
	if err := filter.UnmarshalBinary(data); err != nil {
		return err
	}
	*dst = filter
	return nil
}

// ErrBloomSize means a stored bloom filter declares more bits than its bytes hold.
var ErrBloomSize = errors.New("records: bloom filter is shorter than it declares")

// bloomSizeMatches checks the bit count the binary form declares against the bytes that
// follow it. The bloom package allocates what the count says before it reads anything, so a
// damaged count would otherwise ask for terabytes.
//
// The form is: m (8 bytes), k (8), the bit count (8), then the words of the bit set.
func bloomSizeMatches(data []byte) bool {
	const header = 3 * 8
	if len(data) < header {
		return false
	}
	bits := binary.BigEndian.Uint64(data[2*8:])
	return bits <= uint64(len(data)-header)*8
}
