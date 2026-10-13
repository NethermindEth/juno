//nolint:goconst // A wire name reads best spelled out beside the field number it maps to.
package records

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/utils/cbor/fast"
)

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
