package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// ReadReceipt reads a stored TransactionReceipt.
func ReadReceipt(r *fast.Reader, dst *core.TransactionReceipt) error {
	var seen uint64
	return readFields(r, foldTransactionReceipt, func(key string) (bool, error) {
		var err error
		switch key {
		case "Fee":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Fee)
		case "FeeUnit":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readByte(r, (*byte)(&dst.FeeUnit))
		case "Events":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readPtrSlice(r, &dst.Events, readEvent)
		case "ExecutionResources":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.ExecutionResources, readExecutionResources)
		case "L1ToL2Message":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.L1ToL2Message, readL1ToL2Message)
		case "L2ToL1Message":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readPtrSlice(r, &dst.L2ToL1Message, readL2ToL1Message)
		case "TransactionHash":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.TransactionHash)
		case "Reverted":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readBool(r, &dst.Reverted)
		case "RevertReason":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
			err = readString(r, &dst.RevertReason)
		default:
			return false, nil
		}
		return true, err
	})
}

func readEvent(r *fast.Reader, dst *core.Event) error {
	var seen uint64
	return readFields(r, foldEvent, func(key string) (bool, error) {
		var err error
		switch key {
		case "From":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.From)
		case "Keys":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.Keys)
		case "Data":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.Data)
		default:
			return false, nil
		}
		return true, err
	})
}

func readL1ToL2Message(r *fast.Reader, dst *core.L1ToL2Message) error {
	var seen uint64
	return readFields(r, foldL1ToL2Message, func(key string) (bool, error) {
		var err error
		switch key {
		case "From":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readEthAddress(r, &dst.From)
		case "Nonce":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Nonce)
		case "Payload":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.Payload)
		case "Selector":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.Selector)
		case "To":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.To)
		default:
			return false, nil
		}
		return true, err
	})
}

func readL2ToL1Message(r *fast.Reader, dst *core.L2ToL1Message) error {
	var seen uint64
	return readFields(r, foldL2ToL1Message, func(key string) (bool, error) {
		var err error
		switch key {
		case "From":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readFelt(r, &dst.From)
		case "Payload":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readFeltSlice(r, &dst.Payload)
		case "To":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readEthAddress(r, &dst.To)
		default:
			return false, nil
		}
		return true, err
	})
}

// readEthAddress reads the 20 bytes of an L1 address, a byte string.
func readEthAddress(r *fast.Reader, dst *eth.Address) error {
	if r.Null() {
		return nil
	}
	value, err := r.BytesNoCopy()
	if err != nil {
		return err
	}
	if len(value) != len(dst) {
		return fast.ErrType
	}
	copy(dst[:], value)
	return nil
}

func readExecutionResources(r *fast.Reader, dst *core.ExecutionResources) error {
	var seen uint64
	return readFields(r, foldExecutionResources, func(key string) (bool, error) {
		var err error
		switch key {
		case "BuiltinInstanceCounter":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readBuiltinInstanceCounter(r, &dst.BuiltinInstanceCounter)
		case "MemoryHoles":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.MemoryHoles)
		case "Steps":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Steps)
		case "DataAvailability":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.DataAvailability, readDataAvailability)
		case "TotalGasConsumed":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readPtr(r, &dst.TotalGasConsumed, readGasConsumed)
		default:
			return false, nil
		}
		return true, err
	})
}

func readDataAvailability(r *fast.Reader, dst *core.DataAvailability) error {
	var seen uint64
	return readFields(r, foldDataAvailability, func(key string) (bool, error) {
		var err error
		switch key {
		case "L1Gas":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.L1Gas)
		case "L1DataGas":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.L1DataGas)
		default:
			return false, nil
		}
		return true, err
	})
}

func readGasConsumed(r *fast.Reader, dst *core.GasConsumed) error {
	var seen uint64
	return readFields(r, foldGasConsumed, func(key string) (bool, error) {
		var err error
		switch key {
		case "L1Gas":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.L1Gas)
		case "L1DataGas":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.L1DataGas)
		case "L2Gas":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.L2Gas)
		default:
			return false, nil
		}
		return true, err
	})
}

func readBuiltinInstanceCounter(r *fast.Reader, dst *core.BuiltinInstanceCounter) error {
	var seen uint64
	return readFields(r, foldBuiltinInstanceCounter, func(key string) (bool, error) {
		var err error
		switch key {
		case "Pedersen":
			if once(&seen, 1) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Pedersen)
		case "RangeCheck":
			if once(&seen, 2) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.RangeCheck)
		case "Bitwise":
			if once(&seen, 3) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Bitwise)
		case "Output":
			if once(&seen, 4) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Output)
		case "Ecsda":
			if once(&seen, 5) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Ecsda)
		case "EcOp":
			if once(&seen, 6) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.EcOp)
		case "Keccak":
			if once(&seen, 7) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Keccak)
		case "Poseidon":
			if once(&seen, 8) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.Poseidon)
		case "SegmentArena":
			if once(&seen, 9) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.SegmentArena)
		case "AddMod":
			if once(&seen, 10) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.AddMod)
		case "MulMod":
			if once(&seen, 11) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.MulMod)
		case "RangeCheck96":
			if once(&seen, 12) {
				return false, fast.ErrUnsupported
			}
			err = readUint64(r, &dst.RangeCheck96)
		default:
			return false, nil
		}
		return true, err
	})
}
