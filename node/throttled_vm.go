package node

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var _ vm.VM = (*ThrottledVM)(nil)

type ThrottledVM struct {
	*utils.Throttler[vm.VM]
}

func NewThrottledVM(res vm.VM, concurrenyBudget uint, maxQueueLen int32) *ThrottledVM {
	return &ThrottledVM{
		Throttler: utils.NewThrottler(concurrenyBudget, &res).WithMaxQueueLen(maxQueueLen),
	}
}

func (tvm *ThrottledVM) Call(
	callInfo *vm.CallInfo,
	blockInfo *vm.BlockInfo,
	state core.StateReader,
	maxSteps uint64,
	maxGas uint64,
	errStack, returnStateDiff bool,
) (vm.CallResult, error) {
	ret := vm.CallResult{}
	return ret, tvm.Do(func(vm *vm.VM) error {
		var err error
		ret, err = (*vm).Call(
			callInfo,
			blockInfo,
			state,
			maxSteps,
			maxGas,
			errStack,
			returnStateDiff,
		)
		return err
	})
}

func (tvm *ThrottledVM) Execute(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
	paidFeesOnL1 []*felt.Felt,
	blockInfo *vm.BlockInfo,
	state core.StateReader,
	skipChargeFee,
	skipValidate,
	errOnRevert,
	errStack,
	allowBinarySearch bool,
	isEstimateFee bool,
	returnInitialReads bool,
) (vm.ExecutionResults, error) {
	var executionResult vm.ExecutionResults
	return executionResult, tvm.Do(func(vm *vm.VM) error {
		var err error
		executionResult, err = (*vm).Execute(
			txns,
			declaredClasses,
			paidFeesOnL1,
			blockInfo,
			state,
			skipChargeFee,
			skipValidate,
			errOnRevert,
			errStack,
			allowBinarySearch,
			isEstimateFee,
			returnInitialReads,
		)
		return err
	})
}
