package transaction

import (
	"fmt"
	"sync"
	"sync/atomic"
	"wer/api"
	"wer/state"

	"github.com/NethermindEth/cairo-vm-go/pkg/runner"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

type CallEntryPointVariant struct {
	// ClassHash is the class hash of the entry point.
	// This can be either a ClassHash or nil if it can be deduced from the storage address.
	ClassHash *felt.Felt // To do: revist with optional stack allocated felt

	// CodeAddress is optional, since there is no address to the code implementation
	// in a library call and for outermost calls (triggered by the transaction itself).
	CodeAddress *felt.Felt

	EntryPointType EntryPointType

	EntryPointSelector felt.Felt

	Calldata []felt.Felt

	StorageAddress felt.Felt

	CallerAddress felt.Felt

	CallType CallType

	// InitialGas is the initial gas for this call
	// We can assume that the initial gas is less than 2^64.
	InitialGas uint64
}

// Execute executes a call to an entry point and returns the result, even if execution failed
func (c CallEntryPointVariant) Execute(
	state state.StateReader,
	context *EntryPointExecutionContext,
	remainingGas uint64,
) (CallInfo, error) {
	// 1. Implement Guard against excessive recursion depth
	guard := NewRecursionDepthGuard(context.currentRecursionDepth, 100)
	if err := guard.TryIncrementAndCheckDepth(); err != nil {
		return CallInfo{}, err
	}
	defer guard.Release()

	// 2. Validate contract is deployed
	storageClassHash, err := state.GetClassHashFn(c.StorageAddress)
	if err != nil {
		return CallInfo{}, err // Todo: return standarised error that the contract is not deployed
	}

	// Check if the contract is uninitialized (has default/zero class hash)
	if storageClassHash.IsZero() {
		return CallInfo{}, fmt.Errorf("uninitialized storage address: %s", c.StorageAddress.String())
	}

	// Determine which class hash to use
	var classHash felt.Felt
	if c.ClassHash != nil {
		classHash = *c.ClassHash
	} else {
		classHash = storageClassHash // If not given, take the storage contract class hash
	}

	// Todo: implement hack to prevent v0 hack on argent accounts

	// Get class
	classBox, err := state.GetClassFn(classHash)
	if err != nil {
		return CallInfo{}, err // Todo: return standarised error
	}

	// Todo: push some info to EntryPointRevertInfo

	// 5. Execute the entry point executeEntryPointCallWrapper
	remainingGasPtr := &remainingGas // Todo
	return executeEntryPointCallWrapper(c, classBox, state, context, remainingGasPtr)
}

// NonRevertingExecute executes a call to an entry point and returns an error if execution fails
func (c CallEntryPointVariant) NonRevertingExecute(
	state state.StateReader,
	context *EntryPointExecutionContext,
	remainingGas uint64,
) (CallInfo, error) {
	// 1. call c.Execute
	callInfo, err := c.Execute(state, context, remainingGas)

	// Only update gas tracking and check for execution failure if the initial execute call succeeded
	if err == nil {
		context.SierraGasRevertTracker.UpdateWithNextRemainingGas(
			TrackedResourceSierraGas,
			GasAmount{remainingGas},
		)
		// 2. If the execution of the outer call failed, revert the transaction
		if callInfo.Execution.Failed {
			return CallInfo{}, fmt.Errorf("to do: implement AND Revert - see non_reverting_execute")
		}
	}

	return callInfo, err
}

// EntryPointType represents the type of an entry point
type EntryPointType int

const (
	EntryPointTypeExternal EntryPointType = iota
	EntryPointTypeConstructor
	EntryPointTypeL1Handler
)

// String returns the string representation of EntryPointType
func (e EntryPointType) String() string {
	switch e {
	case EntryPointTypeConstructor:
		return "CONSTRUCTOR"
	case EntryPointTypeExternal:
		return "EXTERNAL"
	case EntryPointTypeL1Handler:
		return "L1_HANDLER"
	default:
		return fmt.Sprintf("Unknown(%d)", int(e))
	}
}

// CallType represents the type of call, regular, or delegate
type CallType int

const (
	CallTypeCall CallType = iota
	CallTypeDelegate
)

// String returns the string representation of CallType
func (c CallType) String() string {
	switch c {
	case CallTypeCall:
		return "CALL"
	case CallTypeDelegate:
		return "DELEGATE"
	default:
		return fmt.Sprintf("Unknown(%d)", int(c))
	}
}

// EntryPointExecutionContext holds the context for executing an entry point
type EntryPointExecutionContext struct {
	// Transaction context shared across execution
	TxContext *TransactionContext

	// VM execution limits
	VMRunResources RunResources

	// Used for tracking events order during the current execution
	NEmittedEvents uint64

	// Used for tracking L2-to-L1 messages order during the current execution
	NSentMessagesToL1 uint64

	// Managed by dedicated guard object
	currentRecursionDepth *atomic.Uint64
	currentRecursionMutex sync.Mutex

	// The execution mode affects the behavior of the hint processor
	ExecutionMode ExecutionMode

	// The call stack of tracked resources from the first entry point to the current
	TrackedResourceStack []TrackedResource

	// Information for reverting the state (includes the revert info of the callers)
	RevertInfos []EntryPointRevertInfo

	// Used to support charging for gas consumed in blockifier revert flow
	SierraGasRevertTracker SierraGasRevertTracker
}

// SierraGasRevertTracker tracks gas consumption for revert flow
type SierraGasRevertTracker struct {
	// Initial gas remaining at the start
	initialRemainingGas GasAmount

	// Last observed remaining gas amount
	lastSeenRemainingGas GasAmount
}

// UpdateWithNextRemainingGas updates the last seen remaining gas
func (t *SierraGasRevertTracker) UpdateWithNextRemainingGas(
	trackedResource TrackedResource,
	nextRemainingGas GasAmount,
) {
	// Only update if we're tracking Sierra gas
	if trackedResource == TrackedResourceSierraGas {
		t.lastSeenRemainingGas = nextRemainingGas
	}
}

// TransactionContext holds the context for a transaction
type TransactionContext struct {
	// Block context shared across transactions
	BlockContext api.BlockContext

	// Information about the transaction
	TxInfo TransactionInfo
}

// Todo
func (tc *TransactionContext) InitialSierraGas() GasAmount {

	switch tc.TxInfo.ResourceBounds.(type) {
	case ValidResourceBoundsL1Gas:
		return tc.BlockContext.VersionedConstants.InitialGasNoUserL2Bound()
	case AllResourceBounds:
		return tc.TxInfo.ResourceBounds.(AllResourceBounds).L2Gas.MaxAmount
	}

	// Default case if none of the above match
	return GasAmount{}
}

// BlockContext holds the context for a block
type BlockContext struct { // Todo: implement
	// Information about the block
	blockInfo BlockInfo

	// Information about the chain
	// chainInfo ChainInfo

	// Constants that may change with versions
	versionedConstants VersionedConstants

	// Configuration for the bouncer
	// bouncerConfig BouncerConfig
}

// RunResources defines the resource limits for a VM run
type RunResources struct {
	// Maximum number of steps, nil means no limit
	nSteps *uint64 // Todo: return with optional uint64
}

// ExecutionMode represents the mode of execution
type ExecutionMode uint8

const (
	// ExecutionModeExecute represents normal execution mode
	ExecutionModeExecute ExecutionMode = iota

	// ExecutionModeValidate represents validation execution mode
	ExecutionModeValidate
)

// EntryPointRevertInfo holds information needed to revert a contract's state
type EntryPointRevertInfo struct {
	// The contract address that the revert info applies to
	ContractAddress felt.Felt

	// The original class hash of the contract that was called
	OriginalClassHash felt.Felt

	// The original storage values
	OriginalValues map[felt.Felt]felt.Felt // map[storage_key]value

	// The number of emitted events before the call
	nEmittedEvents uint64

	// The number of sent messages to L1 before the call
	nSentMessagesToL1 uint64
}

// RecursionDepthGuard ensures that the recursion depth does not exceed the maximum allowed depth.
type RecursionDepthGuard struct {
	currentDepth *atomic.Uint64
	maxDepth     uint64
}

// NewRecursionDepthGuard creates a new RecursionDepthGuard.
func NewRecursionDepthGuard(currentDepth *atomic.Uint64, maxDepth uint64) *RecursionDepthGuard {
	return &RecursionDepthGuard{
		currentDepth: currentDepth,
		maxDepth:     maxDepth,
	}
}

// TryIncrementAndCheckDepth tries to increment the current recursion depth and returns an error
// if the maximum depth would be exceeded.
func (g *RecursionDepthGuard) TryIncrementAndCheckDepth() error {
	newDepth := g.currentDepth.Add(1)
	if newDepth > g.maxDepth {
		// Decrement back since we're returning an error
		g.currentDepth.Add(^uint64(0)) // Subtracting 1 using bitwise complement
		return fmt.Errorf("recursion depth exceeded: %d > %d", newDepth, g.maxDepth)
	}

	return nil
}

// Release decrements the recursion depth when the guard is no longer needed.
func (g *RecursionDepthGuard) Release() {
	g.currentDepth.Add(^uint64(0)) // Subtracting 1 using bitwise complement
}

// executeEntryPointCallWrapper wraps the execution of an entry point call
func executeEntryPointCallWrapper(
	call CallEntryPointVariant,
	classBox state.ClassBox, // Todo: pesky interface. This has a knownon effect for other functions. How can we improve this?
	state state.StateReader,
	context *EntryPointExecutionContext,
	remainingGas *uint64,
) (CallInfo, error) {
	// Determine the current tracked resource
	currentTrackedResource := getTrackedResourceForClass(class, context)

	// If we're tracking Cairo steps, override the initial gas with a high value
	if currentTrackedResource == TrackedResourceCairoSteps {
		// Set a very high gas limit so it won't limit the run
		call.InitialGas = context.TxContext.BlockContext.versionedConstants.InfiniteGasForVMMode()
	}

	// Save original call for potential error handling
	origCall := call // Todo: make a deep copy

	// Todo: look into this
	// Push the current tracked resource to the stack
	context.TrackedResourceStack = append(context.TrackedResourceStack, currentTrackedResource)

	// Execute the entry point call
	callInfo, err := executeEntryPointCall(call, class, state, context)

	// Pop the tracked resource from the stack
	if len(context.TrackedResourceStack) > 0 {
		context.TrackedResourceStack = context.TrackedResourceStack[:len(context.TrackedResourceStack)-1]
	} else {
		// This shouldn't happen, but handle it gracefully
		return CallInfo{}, fmt.Errorf("unexpected empty tracked resource stack")
	}

	// Handle execution results
	if err != nil {
		// Check if it's a pre-execution error and if reverts are enabled
		if preExecErr, ok := err.(PreExecutionError); ok && context.TxContext.BlockContext.versionedConstants.EnableReverts {
			// Create a failed call info with appropriate error code
			errorCode := ""

			switch preExecErr.Type() {
			case PreExecutionErrorTypeEntryPointNotFound, PreExecutionErrorTypeNoEntryPointOfTypeFound:
				errorCode = "0x" + EntryPointNotFoundError
			case PreExecutionErrorTypeInsufficientEntryPointGas:
				errorCode = "0x" + OutOfGasError
			default:
				return CallInfo{}, err
			}

			// Parse error code to felt
			errorCodeFelt, parseErr := new(felt.Felt).SetString(errorCode)
			if parseErr != nil {
				return CallInfo{}, parseErr
			}

			return CallInfo{
				Call: &origCall,
				Execution: CallExecution{
					Retdata:     []felt.Felt{*errorCodeFelt},
					Failed:      true,
					GasConsumed: 0,
					// Other fields would be initialized to zero values
				},
				TrackedResource: currentTrackedResource,
				// Other fields would be initialized to zero values
			}, nil
		}

		return CallInfo{}, err
	}

	// Handle successful execution
	if callInfo.Execution.Failed && !context.TxContext.BlockContext.versionedConstants.EnableReverts {
		// Reverts are disabled, so return an error
		return CallInfo{}, fmt.Errorf("execution failed: %s", extractTrailingCairo1RevertTrace(callInfo, Cairo1RevertHeaderExecution))
	}

	// Update remaining gas
	updateRemainingGas(remainingGas, callInfo)

	return callInfo, nil
}

// Helper function to get the tracked resource for a compiled class
func getTrackedResourceForClass(compiledClass core.CompiledClass, context *EntryPointExecutionContext) TrackedResource {
	var contractTrackedResource TrackedResource

	// Todo
	switch compiledClass.Version() {
	case 0:
		contractTrackedResource = TrackedResourceCairoSteps
	case 1:
		// For Sierra (V1) classes, use Sierra gas
		contractTrackedResource = TrackedResourceSierraGas
	default:
		// Default to Sierra gas for unknown versions
		contractTrackedResource = TrackedResourceSierraGas
	}

	// Todo: double check
	// Check if there's a previous tracked resource in the stack
	if len(context.TrackedResourceStack) > 0 {
		lastTrackedResource := context.TrackedResourceStack[len(context.TrackedResourceStack)-1]

		// Once we ran with CairoSteps, we will continue to run using it for all nested calls
		if lastTrackedResource == TrackedResourceCairoSteps {
			return TrackedResourceCairoSteps
		}
	}

	// Otherwise, use the contract's tracked resource
	return contractTrackedResource
}

// Helper function to update remaining gas
func updateRemainingGas(remainingGas *uint64, callInfo *CallInfo) {
	// Implementation would depend on how gas is tracked in your system
	// This is a placeholder
	if callInfo.Execution.GasConsumed <= *remainingGas {
		*remainingGas -= callInfo.Execution.GasConsumed
	} else {
		*remainingGas = 0
	}
}

// Constants for error codes
const (
	EntryPointNotFoundError = "0x1"
	OutOfGasError           = "0x2"
)

// PreExecutionError represents errors that can occur before execution
type PreExecutionError struct {
	errorType int
	message   string
}

// Error types
const (
	PreExecutionErrorTypeEntryPointNotFound = iota
	PreExecutionErrorTypeNoEntryPointOfTypeFound
	PreExecutionErrorTypeInsufficientEntryPointGas
	// Add other error types as needed
)

func (e PreExecutionError) Error() string {
	return e.message
}

func (e PreExecutionError) Type() int {
	return e.errorType
}

// Cairo1RevertHeader represents the header for Cairo 1 reverts
type Cairo1RevertHeader int

const (
	Cairo1RevertHeaderExecution Cairo1RevertHeader = iota
	// Add other header types as needed
)

// Helper function to extract trailing Cairo 1 revert trace
func extractTrailingCairo1RevertTrace(callInfo *CallInfo, header Cairo1RevertHeader) string {
	// Implementation would depend on your system
	// This is a placeholder
	return "revert trace"
}

// executeEntryPointCall executes an entry point call based on the compiled class version
func executeEntryPointCall(
	call CallEntryPointVariant,
	classBox state.ClassBox,
	stateReader state.StateReader,
	context *EntryPointExecutionContext,
) (CallInfo, error) {
	// Handle different class versions
	switch classBox.Version {
	case state.ClassBoxTypeV0:
		// For Cairo 0 (legacy) classes
		return executeDeprecatedEntryPointCall(call, classBox.V0, stateReader, context)
	case 1:
		// For Cairo 1 (Sierra) classes
		return executeSierraEntryPointCall(call, classBox.V1, stateReader, context)
	default: // Todo: handle Native execution
		return CallInfo{}, fmt.Errorf("unsupported class version: %d", classBox.Version)
	}
}

// executeDeprecatedEntryPointCall handles execution for Cairo 0 classes
func executeDeprecatedEntryPointCall(
	call CallEntryPointVariant,
	class core.Cairo0Class,
	state state.StateReader,
	context *EntryPointExecutionContext,
) (CallInfo, error) {
	// TODO: Implement Cairo 0 execution logic
	return CallInfo{}, fmt.Errorf("Cairo 0 execution not yet implemented")
}

// executeSierraEntryPointCall handles execution for Cairo 1 (Sierra) classes
func executeSierraEntryPointCall(
	call CallEntryPointVariant,
	class core.Cairo1Class,
	state state.StateReader,
	context *EntryPointExecutionContext,
) (CallInfo, error) {
	// Get the current tracked resource from the stack
	if len(context.TrackedResourceStack) == 0 {
		return CallInfo{}, fmt.Errorf("unexpected empty tracked resource stack")
	}
	trackedResource := context.TrackedResourceStack[len(context.TrackedResourceStack)-1]

	// Extract information from the context
	entryPointInitialBudget := context.TxContext.BlockContext.versionedConstants.OsConstants.EntryPointInitialBudget

	// Initialize execution context
	vmContext, err := initializeExecutionContext(call, class, state, context)
	if err != nil {
		return CallInfo{}, err
	}

	// Prepare call arguments
	args, err := prepareCallArguments( // Todo
		vmContext.syscallHandler.baseCall,
		vmContext.runner,
		vmContext.initialSyscallPtr,
		vmContext.syscallHandler.readOnlySegments,
		vmContext.entryPoint,
		entryPointInitialBudget,
	)
	if err != nil {
		return CallInfo{}, err
	}

	nTotalArgs := len(args)

	// Execute
	bytecodeLength := uint64(len(class.Compiled.Bytecode))
	programSegmentSize := bytecodeLength + vmContext.programExtraDataLength
	err = runEntryPoint(
		vmContext.runner,
		vmContext.syscallHandler,
		vmContext.entryPoint,
		args,
		programSegmentSize,
	)
	if err != nil {
		return CallInfo{}, err
	}

	// Finalize execution
	return finalizeExecution(
		vmContext.runner,
		vmContext.syscallHandler,
		nTotalArgs,
		vmContext.programExtraDataLength,
		trackedResource,
	)
}

// VmExecutionContext holds the context for VM execution
type VmExecutionContext struct {
	runner                 *runner.CairoRunner
	syscallHandler         *SyscallHandler
	initialSyscallPtr      felt.Felt
	entryPoint             EntryPoint
	programExtraDataLength uint64
}

// initializeExecutionContext initializes the VM execution context
func initializeExecutionContext(
	call CallEntryPointVariant,
	class core.Cairo1Class,
	state state.StateReader,
	context *EntryPointExecutionContext,
) (*VmExecutionContext, error) {
	// TODO: Implement initialization logic
	return nil, fmt.Errorf("initialize execution context not yet implemented")
}

// prepareCallArguments prepares the arguments for a call
func prepareCallArguments(
	baseCall interface{},
	runner *runner.Runner,
	initialSyscallPtr felt.Felt,
	readOnlySegments interface{},
	entryPoint EntryPoint,
	entryPointInitialBudget uint64,
) ([]felt.Felt, error) {
	// TODO: Implement argument preparation logic
	return nil, fmt.Errorf("prepare call arguments not yet implemented")
}

// EntryPoint represents a Cairo program entry point
type EntryPoint struct {
	pc uint64
	// Add other fields as needed
}

// SyscallHandler processes system calls during execution
type SyscallHandler struct {
	baseCall         interface{}
	readOnlySegments interface{}
	// Add other fields as needed
}

// runEntryPoint runs the entry point with the given arguments
func runEntryPoint(
	runner *runner.Runner,
	syscallHandler *SyscallHandler,
	entryPoint EntryPoint,
	args []felt.Felt,
	programSegmentSize uint64,
) error {
	// Note that we run verification manually after filling the holes in the segment
	verifySecure := false

	// Run from entry point
	err := runner.RunEntryPoint(
		entryPoint.pc,
		args,
		verifySecure,
		programSegmentSize,
		syscallHandler,
	)
	if err != nil {
		return err
	}

	// Fill holes if needed
	err = maybeFillHoles(entryPoint, runner)
	if err != nil {
		return err
	}

	// Verify secure runner
	err = verifySecureRunner(runner, false, programSegmentSize)
	if err != nil {
		return fmt.Errorf("virtual machine error: %w", err)
	}

	return nil
}

// maybeFillHoles fills holes in the runner's memory if needed
func maybeFillHoles(entryPoint EntryPoint, runner *runner.Runner) error {
	// TODO: Implement hole filling logic
	return nil
}

// verifySecureRunner verifies that the runner is in a secure state
func verifySecureRunner(runner *runner.Runner, checkRangeCheckUsage bool, programSegmentSize uint64) error {
	// TODO: Implement security verification logic
	return nil
}

// finalizeExecution finalizes the execution and returns the call info
func finalizeExecution(
	runner *runner.Runner,
	syscallHandler *SyscallHandler,
	nTotalArgs int,
	programExtraDataLength uint64,
	trackedResource TrackedResource,
) (CallInfo, error) {
	// TODO: Implement finalization logic
	return CallInfo{}, fmt.Errorf("finalize execution not yet implemented")
}
