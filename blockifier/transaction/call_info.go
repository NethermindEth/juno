package transaction

import "github.com/NethermindEth/juno/core/felt"

// CallInfo represents the full effects of executing an entry point, including the inner calls it invoked.
type CallInfo struct {
	Call *CallEntryPointVariant // Todo: in Rust this is an CallEntryPointVariant<Option>>. To revist with option-no-pointer.

	Execution CallExecution

	InnerCalls []CallInfo

	Resources ExecutionResources // Resources used during execution

	TrackedResource TrackedResource // Resource tracking information

	StorageAccessTracker StorageAccessTracker // Additional information gathered during execution
}

// Default returns a new CallInfo with default values
func DefaultCallInfo() *CallInfo {
	return &CallInfo{
		InnerCalls: make([]CallInfo, 0),
	}
}

// CallExecution contains information about the execution of a call.
type CallExecution struct {
	Retdata []felt.Felt // Return data from the execution

	Events []OrderedEvent // Events emitted during execution

	// L2ToL1Messages []OrderedL2ToL1Message // Messages sent from L2 to L1 during execution // To do

	Failed bool // Indicates whether the execution failed

	GasConsumed uint64 // Amount of gas consumed during execution
}

// OrderedEvent represents an event with its execution order
type OrderedEvent struct {
	// The order in which the event was emitted
	Order uint64

	// The content of the event
	Event EventContent
}

// EventContent contains the keys and data for an event
type EventContent struct {
	// The event keys
	Keys []felt.Felt

	// The event data
	Data []felt.Felt
}

// ExecutionResources tracks computational resources used during execution.
type ExecutionResources struct {
	NSteps uint64 // Number of steps executed

	NMemoryHoles uint64 // Number of memory holes

	BuiltinInstanceCounter map[BuiltinName]uint64 // Counter for each builtin instance
}

// Todo: use cairo-vm??
// BuiltinName represents the various Cairo builtin operations
type BuiltinName string

const (
	BuiltinNameOutput       BuiltinName = "output"
	BuiltinNameRangeCheck   BuiltinName = "range_check"
	BuiltinNamePedersen     BuiltinName = "pedersen"
	BuiltinNameEcdsa        BuiltinName = "ecdsa"
	BuiltinNameKeccak       BuiltinName = "keccak"
	BuiltinNameBitwise      BuiltinName = "bitwise"
	BuiltinNameEcOp         BuiltinName = "ec_op"
	BuiltinNamePoseidon     BuiltinName = "poseidon"
	BuiltinNameSegmentArena BuiltinName = "segment_arena"
	BuiltinNameRangeCheck96 BuiltinName = "range_check96"
	BuiltinNameAddMod       BuiltinName = "add_mod"
	BuiltinNameMulMod       BuiltinName = "mul_mod"
)

// TrackedResource represents the resource tracking mode
type TrackedResource uint8

const (
	// TrackedResourceCairoSteps represents VM mode tracking
	TrackedResourceCairoSteps TrackedResource = iota

	// TrackedResourceSierraGas represents Sierra mode tracking
	TrackedResourceSierraGas
)

// StorageAccessTracker keeps track of storage elements accessed during execution
type StorageAccessTracker struct {
	// TODO: refactor all to use a single enum with accessed_keys and ordered_values.

	// Values read from storage
	StorageReadValues []felt.Felt

	// Storage keys that were accessed
	AccessedStorageKeys map[felt.Felt]bool //map[key]bool

	// Class hash values that were read
	ReadClassHashValues []felt.Felt

	// Contract addresses that were accessed
	AccessedContractAddresses map[felt.Felt]bool

	// TODO: add tests for storage tracking of contract 0x1

	// Block hash values that were read
	ReadBlockHashValues []felt.Felt

	// Block numbers that were accessed
	AccessedBlocks map[uint64]bool // map[block_ number]bool
}
