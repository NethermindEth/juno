package transaction

import "github.com/NethermindEth/juno/core"

// Todo:
// Note: blockifier has `Deprecated` and `Current` TxnInfo. I didn't impl Deprecated
type TransactionInfo struct {
	CommonFields              CommonAccountFields
	ResourceBounds            ValidResourceBounds
	Tip                       Tip
	NonceDataAvailabilityMode DataAvailabilityMode
	FeeDataAvailabilityMode   DataAvailabilityMode
	PaymasterData             PaymasterData
	AccountDeploymentData     AccountDeploymentData
}

// createInvokeTxInfo creates TransactionInfo for an InvokeTransaction
func createInvokeTxInfo[T core.Transaction](tx T) TransactionInfo {
	// TODO: Implement actual logic to create transaction info
	return TransactionInfo{}
}

// createDeclareTxInfo creates TransactionInfo for a DeclareTransaction
func createDeclareTxInfo[T core.Transaction](tx T) TransactionInfo {
	// TODO: Implement actual logic to create transaction info
	return TransactionInfo{}
}

// createDeployTxInfo creates TransactionInfo for a DeployTransaction
func createDeployTxInfo[T core.Transaction](tx T) TransactionInfo {
	// TODO: Implement actual logic to create transaction info
	return TransactionInfo{}
}

// createL1HandlerTxInfo creates TransactionInfo for an L1HandlerTransaction
func createL1HandlerTxInfo[T core.Transaction](tx T) TransactionInfo {
	// TODO: Implement actual logic to create transaction info
	return TransactionInfo{}
}
