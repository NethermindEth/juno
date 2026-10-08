package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

// ReadReceipt reads a stored TransactionReceipt.
func ReadReceipt(r *fast.Reader, dst *core.TransactionReceipt) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "Fee":
			err = readFelt(r, &dst.Fee)
		case "FeeUnit":
			err = readByte(r, (*byte)(&dst.FeeUnit))
		case "Events":
			err = readPtrSlice(r, &dst.Events, readEvent)
		case "ExecutionResources":
			err = readPtr(r, &dst.ExecutionResources, readExecutionResources)
		case "L1ToL2Message":
			err = readPtr(r, &dst.L1ToL2Message, readL1ToL2Message)
		case "L2ToL1Message":
			err = readPtrSlice(r, &dst.L2ToL1Message, readL2ToL1Message)
		case "TransactionHash":
			err = readFelt(r, &dst.TransactionHash)
		case "Reverted":
			err = readBool(r, &dst.Reverted)
		case "RevertReason":
			err = readString(r, &dst.RevertReason)
		default:
			return false, nil
		}
		return true, err
	})
}

func readEvent(r *fast.Reader, dst *core.Event) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "From":
			err = readFelt(r, &dst.From)
		case "Keys":
			err = readFeltSlice(r, &dst.Keys)
		case "Data":
			err = readFeltSlice(r, &dst.Data)
		default:
			return false, nil
		}
		return true, err
	})
}

func readL1ToL2Message(r *fast.Reader, dst *core.L1ToL2Message) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "From":
			err = readEthAddress(r, &dst.From)
		case "Nonce":
			err = readFelt(r, &dst.Nonce)
		case "Payload":
			err = readFeltSlice(r, &dst.Payload)
		case "Selector":
			err = readFelt(r, &dst.Selector)
		case "To":
			err = readFelt(r, &dst.To)
		default:
			return false, nil
		}
		return true, err
	})
}

func readL2ToL1Message(r *fast.Reader, dst *core.L2ToL1Message) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "From":
			err = readFelt(r, &dst.From)
		case "Payload":
			err = readFeltSlice(r, &dst.Payload)
		case "To":
			err = readEthAddress(r, &dst.To)
		default:
			return false, nil
		}
		return true, err
	})
}

// readEthAddress reads the 20 bytes of an L1 address, a byte string.
func readEthAddress(r *fast.Reader, dst *eth.Address) error {
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
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "BuiltinInstanceCounter":
			err = readBuiltinInstanceCounter(r, &dst.BuiltinInstanceCounter)
		case "MemoryHoles":
			err = readUint64(r, &dst.MemoryHoles)
		case "Steps":
			err = readUint64(r, &dst.Steps)
		case "DataAvailability":
			err = readPtr(r, &dst.DataAvailability, readDataAvailability)
		case "TotalGasConsumed":
			err = readPtr(r, &dst.TotalGasConsumed, readGasConsumed)
		default:
			return false, nil
		}
		return true, err
	})
}

func readDataAvailability(r *fast.Reader, dst *core.DataAvailability) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "L1Gas":
			err = readUint64(r, &dst.L1Gas)
		case "L1DataGas":
			err = readUint64(r, &dst.L1DataGas)
		default:
			return false, nil
		}
		return true, err
	})
}

func readGasConsumed(r *fast.Reader, dst *core.GasConsumed) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "L1Gas":
			err = readUint64(r, &dst.L1Gas)
		case "L1DataGas":
			err = readUint64(r, &dst.L1DataGas)
		case "L2Gas":
			err = readUint64(r, &dst.L2Gas)
		default:
			return false, nil
		}
		return true, err
	})
}

func readBuiltinInstanceCounter(r *fast.Reader, dst *core.BuiltinInstanceCounter) error {
	return readFields(r, func(key []byte) (bool, error) {
		var err error
		switch string(key) {
		case "Pedersen":
			err = readUint64(r, &dst.Pedersen)
		case "RangeCheck":
			err = readUint64(r, &dst.RangeCheck)
		case "Bitwise":
			err = readUint64(r, &dst.Bitwise)
		case "Output":
			err = readUint64(r, &dst.Output)
		case "Ecsda":
			err = readUint64(r, &dst.Ecsda)
		case "EcOp":
			err = readUint64(r, &dst.EcOp)
		case "Keccak":
			err = readUint64(r, &dst.Keccak)
		case "Poseidon":
			err = readUint64(r, &dst.Poseidon)
		case "SegmentArena":
			err = readUint64(r, &dst.SegmentArena)
		case "AddMod":
			err = readUint64(r, &dst.AddMod)
		case "MulMod":
			err = readUint64(r, &dst.MulMod)
		case "RangeCheck96":
			err = readUint64(r, &dst.RangeCheck96)
		default:
			return false, nil
		}
		return true, err
	})
}
