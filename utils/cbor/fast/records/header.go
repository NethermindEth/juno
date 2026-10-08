package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/bits-and-blooms/bloom/v3"
)

// ReadHeader reads a stored block header. The keys of the legacy fields GasPrice,
// GasPriceSTRK and ExtraData are skipped, as fxamacker skips them.
func ReadHeader(r *fast.Reader, dst *core.Header) error {
	return readFields(r, foldHeader, func(key string) (bool, error) {
		var err error
		switch key {
		case "Hash":
			err = readFelt(r, &dst.Hash)
		case "ParentHash":
			err = readFelt(r, &dst.ParentHash)
		case "Number":
			err = readUint64(r, &dst.Number)
		case "GlobalStateRoot":
			err = readFelt(r, &dst.GlobalStateRoot)
		case "SequencerAddress":
			err = readFelt(r, &dst.SequencerAddress)
		case "TransactionCount":
			err = readUint64(r, &dst.TransactionCount)
		case "EventCount":
			err = readUint64(r, &dst.EventCount)
		case "Timestamp":
			err = readUint64(r, &dst.Timestamp)
		case "ProtocolVersion":
			err = readString(r, &dst.ProtocolVersion)
		case "EventsBloom":
			err = readBloom(r, &dst.EventsBloom)
		case "gasprice":
			err = readFelt(r, &dst.L1GasPriceETH)
		case "Signatures":
			err = readSlice(r, &dst.Signatures, readFeltPtrSlice)
		case "gaspricestrk":
			err = readFelt(r, &dst.L1GasPriceSTRK)
		case "L1DAMode":
			var mode uint64
			err = readUint64(r, &mode)
			dst.L1DAMode = core.L1DAMode(mode)
		case "L1DataGasPrice":
			err = readPtr(r, &dst.L1DataGasPrice, readGasPrice)
		case "L2GasPrice":
			err = readPtr(r, &dst.L2GasPrice, readGasPrice)
		default:
			return false, nil
		}
		return true, err
	})
}

func readGasPrice(r *fast.Reader, dst *core.GasPrice) error {
	return readFields(r, foldGasPrice, func(key string) (bool, error) {
		var err error
		switch key {
		case "PriceInWei":
			err = readFelt(r, &dst.PriceInWei)
		case "PriceInFri":
			err = readFelt(r, &dst.PriceInFri)
		default:
			return false, nil
		}
		return true, err
	})
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
	filter := new(bloom.BloomFilter)
	if err := filter.UnmarshalBinary(data); err != nil {
		return err
	}
	*dst = filter
	return nil
}
