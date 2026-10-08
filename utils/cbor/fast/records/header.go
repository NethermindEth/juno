package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/utils/cbor/fast"
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
