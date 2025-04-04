package transaction

// TransactionExecutionInfo contains information about a transaction's execution.
type TransactionExecutionInfo struct {
	// // ValidationCallInfo is the transaction validation call info; nil for L1Handler.
	// ValidationCallInfo *CallInfo
	// // ExecuteCallInfo is the transaction execution call info; nil for Declare.
	// ExecuteCallInfo *CallInfo
	// // FeeTransferCallInfo is the fee transfer call info; nil for L1Handler.
	// FeeTransferCallInfo *CallInfo
	// // RevertError contains error information if the transaction was reverted.
	// RevertError *RevertError
	// // Receipt is the receipt of the transaction.
	// // Contains the actual fee charged (in units of the relevant fee token),
	// // actual gas consumption charged for data availability,
	// // actual execution resources charged (including L1 gas and OS resources),
	// // and total gas consumed.
	// Receipt TransactionReceipt
}

// (Big) todo: define TransactionExecutionInfo methods as we go along

// (Big) todo: RevertError should handle error formating (ErrorStack,FeeCheckError)
type RevertError string
