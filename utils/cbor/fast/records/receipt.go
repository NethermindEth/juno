//nolint:goconst // A wire name reads best spelled out beside the field number it maps to.
package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

const (
	receiptFee = iota
	receiptFeeUnit
	receiptEvents
	receiptExecutionResources
	receiptL1ToL2Message
	receiptL2ToL1Message
	receiptTransactionHash
	receiptReverted
	receiptRevertReason
)

var receiptFields = fieldsOf[core.TransactionReceipt]([]string{
	receiptFee:                "Fee",
	receiptFeeUnit:            "FeeUnit",
	receiptEvents:             "Events",
	receiptExecutionResources: "ExecutionResources",
	receiptL1ToL2Message:      "L1ToL2Message",
	receiptL2ToL1Message:      "L2ToL1Message",
	receiptTransactionHash:    "TransactionHash",
	receiptReverted:           "Reverted",
	receiptRevertReason:       "RevertReason",
})

// ReadReceipt reads a stored TransactionReceipt.
func ReadReceipt(r *fast.Reader, dst *core.TransactionReceipt) error {
	s, err := r.OpenStruct(receiptFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case receiptFee:
			err = readFelt(r, &dst.Fee)
		case receiptFeeUnit:
			err = readByte(r, (*byte)(&dst.FeeUnit))
		case receiptEvents:
			err = readPtrSlice(r, &dst.Events, readEvent)
		case receiptExecutionResources:
			err = readExecutionResources(r, &dst.ExecutionResources)
		case receiptL1ToL2Message:
			err = readPtr(r, &dst.L1ToL2Message, readL1ToL2Message)
		case receiptL2ToL1Message:
			err = readPtrSlice(r, &dst.L2ToL1Message, readL2ToL1Message)
		case receiptTransactionHash:
			err = readFelt(r, &dst.TransactionHash)
		case receiptReverted:
			err = readBool(r, &dst.Reverted)
		case receiptRevertReason:
			err = readString(r, &dst.RevertReason)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	eventFrom = iota
	eventKeys
	eventData
)

var eventFields = fieldsOf[core.Event]([]string{
	eventFrom: "From",
	eventKeys: "Keys",
	eventData: "Data",
})

func readEvent(r *fast.Reader, dst *core.Event) error {
	s, err := r.OpenStruct(eventFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case eventFrom:
			err = readFelt(r, &dst.From)
		case eventKeys:
			err = readFeltSlice(r, &dst.Keys)
		case eventData:
			err = readFeltSlice(r, &dst.Data)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	l1ToL2From = iota
	l1ToL2Nonce
	l1ToL2Payload
	l1ToL2Selector
	l1ToL2To
)

var l1ToL2Fields = fieldsOf[core.L1ToL2Message]([]string{
	l1ToL2From:     "From",
	l1ToL2Nonce:    "Nonce",
	l1ToL2Payload:  "Payload",
	l1ToL2Selector: "Selector",
	l1ToL2To:       "To",
})

func readL1ToL2Message(r *fast.Reader, dst *core.L1ToL2Message) error {
	s, err := r.OpenStruct(l1ToL2Fields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case l1ToL2From:
			err = readEthAddress(r, &dst.From)
		case l1ToL2Nonce:
			err = readFelt(r, &dst.Nonce)
		case l1ToL2Payload:
			err = readFeltSlice(r, &dst.Payload)
		case l1ToL2Selector:
			err = readFelt(r, &dst.Selector)
		case l1ToL2To:
			err = readFelt(r, &dst.To)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	l2ToL1From = iota
	l2ToL1Payload
	l2ToL1To
)

var l2ToL1Fields = fieldsOf[core.L2ToL1Message]([]string{
	l2ToL1From:    "From",
	l2ToL1Payload: "Payload",
	l2ToL1To:      "To",
})

func readL2ToL1Message(r *fast.Reader, dst *core.L2ToL1Message) error {
	s, err := r.OpenStruct(l2ToL1Fields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case l2ToL1From:
			err = readFelt(r, &dst.From)
		case l2ToL1Payload:
			err = readFeltSlice(r, &dst.Payload)
		case l2ToL1To:
			err = readEthAddress(r, &dst.To)
		default:
			err = r.Skip()
		}
	}
	return err
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

const (
	resourcesBuiltinInstanceCounter = iota
	resourcesMemoryHoles
	resourcesSteps
	resourcesDataAvailability
	resourcesTotalGasConsumed
)

var executionResourcesFields = fieldsOf[core.ExecutionResources]([]string{
	resourcesBuiltinInstanceCounter: "BuiltinInstanceCounter",
	resourcesMemoryHoles:            "MemoryHoles",
	resourcesSteps:                  "Steps",
	resourcesDataAvailability:       "DataAvailability",
	resourcesTotalGasConsumed:       "TotalGasConsumed",
})

// executionResources holds execution resources and the structs they point to, so that one
// allocation serves all three.
type executionResources struct {
	core.ExecutionResources
	dataAvailability core.DataAvailability
	totalGasConsumed core.GasConsumed
}

// readExecutionResources reads execution resources behind a pointer. Null leaves the pointer nil.
func readExecutionResources(r *fast.Reader, dst **core.ExecutionResources) error {
	if r.Null() {
		*dst = nil
		return nil
	}
	block := new(executionResources)
	resources := &block.ExecutionResources
	s, err := r.OpenStruct(executionResourcesFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			if err == nil {
				*dst = resources
			}
			return err
		case resourcesBuiltinInstanceCounter:
			err = readBuiltinInstanceCounter(r, &resources.BuiltinInstanceCounter)
		case resourcesMemoryHoles:
			err = readUint64(r, &resources.MemoryHoles)
		case resourcesSteps:
			err = readUint64(r, &resources.Steps)
		case resourcesDataAvailability:
			if r.Null() {
				resources.DataAvailability = nil
			} else if err = readDataAvailability(r, &block.dataAvailability); err == nil {
				resources.DataAvailability = &block.dataAvailability
			}
		case resourcesTotalGasConsumed:
			if r.Null() {
				resources.TotalGasConsumed = nil
			} else if err = readGasConsumed(r, &block.totalGasConsumed); err == nil {
				resources.TotalGasConsumed = &block.totalGasConsumed
			}
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	dataAvailabilityL1Gas = iota
	dataAvailabilityL1DataGas
)

var dataAvailabilityFields = fieldsOf[core.DataAvailability]([]string{
	dataAvailabilityL1Gas:     "L1Gas",
	dataAvailabilityL1DataGas: "L1DataGas",
})

func readDataAvailability(r *fast.Reader, dst *core.DataAvailability) error {
	s, err := r.OpenStruct(dataAvailabilityFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case dataAvailabilityL1Gas:
			err = readUint64(r, &dst.L1Gas)
		case dataAvailabilityL1DataGas:
			err = readUint64(r, &dst.L1DataGas)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	gasConsumedL1Gas = iota
	gasConsumedL1DataGas
	gasConsumedL2Gas
)

var gasConsumedFields = fieldsOf[core.GasConsumed]([]string{
	gasConsumedL1Gas:     "L1Gas",
	gasConsumedL1DataGas: "L1DataGas",
	gasConsumedL2Gas:     "L2Gas",
})

func readGasConsumed(r *fast.Reader, dst *core.GasConsumed) error {
	s, err := r.OpenStruct(gasConsumedFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case gasConsumedL1Gas:
			err = readUint64(r, &dst.L1Gas)
		case gasConsumedL1DataGas:
			err = readUint64(r, &dst.L1DataGas)
		case gasConsumedL2Gas:
			err = readUint64(r, &dst.L2Gas)
		default:
			err = r.Skip()
		}
	}
	return err
}

const (
	builtinPedersen = iota
	builtinRangeCheck
	builtinBitwise
	builtinOutput
	builtinEcsda
	builtinEcOp
	builtinKeccak
	builtinPoseidon
	builtinSegmentArena
	builtinAddMod
	builtinMulMod
	builtinRangeCheck96
)

var builtinFields = fieldsOf[core.BuiltinInstanceCounter]([]string{
	builtinPedersen:     "Pedersen",
	builtinRangeCheck:   "RangeCheck",
	builtinBitwise:      "Bitwise",
	builtinOutput:       "Output",
	builtinEcsda:        "Ecsda",
	builtinEcOp:         "EcOp",
	builtinKeccak:       "Keccak",
	builtinPoseidon:     "Poseidon",
	builtinSegmentArena: "SegmentArena",
	builtinAddMod:       "AddMod",
	builtinMulMod:       "MulMod",
	builtinRangeCheck96: "RangeCheck96",
})

func readBuiltinInstanceCounter(r *fast.Reader, dst *core.BuiltinInstanceCounter) error {
	s, err := r.OpenStruct(builtinFields)
	for err == nil {
		var field int
		field, err = r.NextField(&s)
		switch field {
		case fast.Done:
			return err
		case builtinPedersen:
			err = readUint64(r, &dst.Pedersen)
		case builtinRangeCheck:
			err = readUint64(r, &dst.RangeCheck)
		case builtinBitwise:
			err = readUint64(r, &dst.Bitwise)
		case builtinOutput:
			err = readUint64(r, &dst.Output)
		case builtinEcsda:
			err = readUint64(r, &dst.Ecsda)
		case builtinEcOp:
			err = readUint64(r, &dst.EcOp)
		case builtinKeccak:
			err = readUint64(r, &dst.Keccak)
		case builtinPoseidon:
			err = readUint64(r, &dst.Poseidon)
		case builtinSegmentArena:
			err = readUint64(r, &dst.SegmentArena)
		case builtinAddMod:
			err = readUint64(r, &dst.AddMod)
		case builtinMulMod:
			err = readUint64(r, &dst.MulMod)
		case builtinRangeCheck96:
			err = readUint64(r, &dst.RangeCheck96)
		default:
			err = r.Skip()
		}
	}
	return err
}
