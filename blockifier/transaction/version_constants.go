package transaction

import (
	"encoding/json"
	"fmt"
	"os"
)

// VersionedConstants holds constants that may change with versions of the protocol
type VersionedConstants struct {
	// Transaction event limits
	TxEventLimits EventLimits

	// Maximum number of steps for invoke transactions
	InvokeTxMaxNSteps uint32

	// Gas costs for archival data
	ArchivalDataGasCosts ArchivalDataGasCosts

	// Maximum recursion depth
	MaxRecursionDepth uint64

	// Maximum number of steps for validation
	ValidateMaxNSteps uint32

	// Minimum Sierra version for Sierra gas
	MinSierraVersionForSierraGas SierraVersion

	// Transaction settings
	DisableCairo0Redeclaration bool
	EnableStatefulCompression  bool
	ComprehensiveStateDiff     bool
	IgnoreInnerEventResources  bool

	// Compiler settings
	EnableReverts bool

	// Fee related
	VMResourceFeeCost VmResourceCosts
	EnableTip         bool
	AllocationCost    AllocationCost

	// OS constants
	OsConstants OsConstants
}

// OsConstants holds constants related to the Cairo OS
type OsConstants struct {
	// Todo : implement

	// Todo: DefaultInitialGasCost is actually nested in other types
	DefaultInitialGasCost uint64

	// Todo: this needs to be nested / moved
	EntryPointInitialBudget uint64
}

// InfiniteGasForVMMode returns a very high gas limit for VM mode
func (c *VersionedConstants) InfiniteGasForVMMode() uint64 {
	return c.OsConstants.DefaultInitialGasCost
}

// EventLimits defines limits for events
type EventLimits struct {
	// Maximum number of keys in an event
	MaxKeysLength uint32

	// Maximum number of data elements in an event
	MaxDataLength uint32
}

// ArchivalDataGasCosts defines gas costs for archival data
type ArchivalDataGasCosts struct {
	// Cost per byte of calldata
	CalldataPerByte uint64

	// Cost per byte of signature
	SignaturePerByte uint64
}

// SierraVersion represents a Sierra compiler version
type SierraVersion struct {
	Major uint32
	Minor uint32
	Patch uint32
}

type VmResourceCosts struct {
	// Cost per Cairo step
	CairoSteps uint64

	// Cost per memory hole
	MemoryHoles uint64
}

// AllocationCost defines the cost of allocating storage
type AllocationCost struct {
	// Cost per storage cell
	PerCell uint64
}

// LoadVersionedConstantsFromFile loads VersionedConstants from a JSON file
func LoadVersionedConstantsFromFile(filePath string) (*VersionedConstants, error) {
	// Read the file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read constants file: %w", err)
	}

	// Parse the JSON
	var constants VersionedConstants
	if err := json.Unmarshal(data, &constants); err != nil {
		return nil, fmt.Errorf("failed to parse constants file: %w", err)
	}

	return &constants, nil
}
