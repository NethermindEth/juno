package records

import (
	"encoding/binary"
	"errors"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/bits-and-blooms/bloom/v3"
)

// ReadHeader reads a stored block header. The keys of the legacy fields GasPrice,
// GasPriceSTRK and ExtraData are skipped, as fxamacker skips them.
func ReadHeader(r *fast.Reader, dst *core.Header) error {
	var seen uint64
	return readFields(r, foldHeader, func(key string) (bool, error) {
		var err error
		switch key {
		case "Hash":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Hash)
		case "ParentHash":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.ParentHash)
		case "Number":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Number)
		case "GlobalStateRoot":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.GlobalStateRoot)
		case "SequencerAddress":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.SequencerAddress)
		case "TransactionCount":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.TransactionCount)
		case "EventCount":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.EventCount)
		case "Timestamp":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Timestamp)
		case "ProtocolVersion":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
			err = readString(r, &dst.ProtocolVersion)
		case "EventsBloom":
			if once(&seen, 10) {
				return false, fast.ErrUnsupported
			}
			err = readBloom(r, &dst.EventsBloom)
		case "gasprice":
			if once(&seen, 11) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.L1GasPriceETH)
		case "Signatures":
			if once(&seen, 12) {
				return false, fast.ErrUnsupported
			}
			err = readSlice(r, &dst.Signatures, readFeltPtrSlice)
		case "gaspricestrk":
			if once(&seen, 13) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.L1GasPriceSTRK)
		case "L1DAMode":
			if once(&seen, 14) {
				return false, fast.ErrUnsupported
			}
			var mode uint64
			err = readUint64(r, &mode)
			dst.L1DAMode = core.L1DAMode(mode)
		case "L1DataGasPrice":
			if once(&seen, 15) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.L1DataGasPrice, readGasPrice)
		case "L2GasPrice":
			if once(&seen, 16) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.L2GasPrice, readGasPrice)
		default:
			return false, nil
		}
		return true, err
	})
}

func readGasPrice(r *fast.Reader, dst *core.GasPrice) error {
	var seen uint64
	return readFields(r, foldGasPrice, func(key string) (bool, error) {
		var err error
		switch key {
		case "PriceInWei":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.PriceInWei)
		case "PriceInFri":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
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
