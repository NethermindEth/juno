package records

import "github.com/NethermindEth/juno/core"

// The field names each struct is matched against, for keys that differ from them only in case.
var (
	foldBuiltinInstanceCounter   = newFolder[core.BuiltinInstanceCounter]()
	foldCasmClass                = newFolder[core.CasmClass]()
	foldCasmEntryPoint           = newFolder[core.CasmEntryPoint]()
	foldDataAvailability         = newFolder[core.DataAvailability]()
	foldDeclareTransaction       = newFolder[core.DeclareTransaction]()
	foldDeployAccountTransaction = newFolder[core.DeployAccountTransaction]()
	foldDeployTransaction        = newFolder[core.DeployTransaction]()
	foldEvent                    = newFolder[core.Event]()
	foldExecutionResources       = newFolder[core.ExecutionResources]()
	foldGasConsumed              = newFolder[core.GasConsumed]()
	foldGasPrice                 = newFolder[core.GasPrice]()
	foldHeader                   = newFolder[core.Header]()
	foldInvokeTransaction        = newFolder[core.InvokeTransaction]()
	foldL1HandlerTransaction     = newFolder[core.L1HandlerTransaction]()
	foldL1ToL2Message            = newFolder[core.L1ToL2Message]()
	foldL2ToL1Message            = newFolder[core.L2ToL1Message]()
	foldResourceBounds           = newFolder[core.ResourceBounds]()
	foldSegmentLengths           = newFolder[core.SegmentLengths]()
	foldSierraClass              = newFolder[core.SierraClass]()
	foldSierraEntryPoint         = newFolder[core.SierraEntryPoint]()
	foldSierraEntryPointsByType  = newFolder[core.SierraEntryPointsByType]()
	foldStateDiff                = newFolder[core.StateDiff]()
	foldStateUpdate              = newFolder[core.StateUpdate]()
	foldTransactionReceipt       = newFolder[core.TransactionReceipt]()
)
