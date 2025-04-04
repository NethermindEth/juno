package transaction

import (
	"errors"
	"wer/state"

	"github.com/NethermindEth/juno/core/felt"
)

// TODO : move these functions (here for tmp neatness)

// AssertActualFeeInBounds checks if the actual fee is within the allowed bounds.
func (tx ExecutableTransaction[T]) AssertActualFeeInBounds(actualFee Fee) error {
	maxFee := tx.TransactionContext.TxInfo.ResourceBounds.MaxPossibleFee()
	if actualFee > maxFee {
		return errors.New("actual fee exceeded bounds; max possible fee is " + maxFee.String())
	}
	return nil
}

// HandleFee manages the fee transfer process.
func (tx ExecutableTransaction[T]) HandleFee(state *state.CachedState, actualFee Fee) (CallInfo, error) {
	if !tx.ExecutionFlags.ChargeFee || actualFee.Amount.IsZero() {
		// Fee charging is not enforced in some tests.
		return CallInfo{}, nil
	}

	if err := tx.AssertActualFeeInBounds(actualFee); err != nil {
		return CallInfo{}, err
	}

	var feeTransferCallInfo CallInfo
	var err error
	if tx.ExecutionFlags.OnlyQuery && !tx.TransactionContext.IsSequencerTheSender() {
		feeTransferCallInfo, err = tx.ConcurrencyExecuteFeeTransfer(state, actualFee)
	} else {
		feeTransferCallInfo, err = tx.ExecuteFeeTransfer(state, actualFee)
	}

	if err != nil {
		return CallInfo{}, err
	}

	return feeTransferCallInfo, nil
}

// ExecuteFeeTransfer performs the fee transfer.
func (tx ExecutableTransaction[T]) ExecuteFeeTransfer(state *state.CachedState, actualFee Fee) (CallInfo, error) {
	lsbAmount := actualFee.Amount
	msbAmount := new(felt.Felt).SetUint64(0)

	blockContext := tx.TransactionContext.BlockContext
	storageAddress := tx.TransactionContext.FeeTokenAddress()
	remainingGasForFeeTransfer := blockContext.VersionedConstants.OSConstants.GasCosts.Base.DefaultInitialGasCost

	feeTransferCall := CallEntryPoint{
		ClassHash:          nil,
		CodeAddress:        nil,
		EntryPointType:     External,
		EntryPointSelector: SelectorFromName(TransferEntryPointName),
		Calldata:           []felt.Felt{blockContext.BlockInfo.SequencerAddress.Key(), lsbAmount, msbAmount},
		StorageAddress:     storageAddress,
		CallerAddress:      tx.TransactionContext.TxInfo.SenderAddress(),
		CallType:           Call,
		InitialGas:         remainingGasForFeeTransfer,
	}

	context := NewEntryPointExecutionContextForInvoke(
		tx.TransactionContext,
		true,
		NewSierraGasRevertTracker(GasAmount{remainingGasForFeeTransfer}),
	)

	return feeTransferCall.Execute(state, context, &remainingGasForFeeTransfer)
}
