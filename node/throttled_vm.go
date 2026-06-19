package node

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/throttler"
	"github.com/NethermindEth/juno/vm"
)

var _ vm.VM = (*ThrottledVM)(nil)

type ThrottledVM struct {
	*throttler.Throttler[vm.VM]
}

func NewThrottledVM(res vm.VM, concurrenyBudget uint, maxQueueLen int32) *ThrottledVM {
	return &ThrottledVM{
		Throttler: throttler.NewThrottler(concurrenyBudget, &res).WithMaxQueueLen(maxQueueLen),
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

func (tvm *ThrottledVM) runExec(
	fn func(inner vm.VM) (vm.ExecutionResults, error),
) (vm.ExecutionResults, error) {
	var result vm.ExecutionResults
	return result, tvm.Do(func(inner *vm.VM) error {
		var err error
		result, err = fn(*inner)
		return err
	})
}

func (tvm *ThrottledVM) Simulate(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
	blockInfo *vm.BlockInfo,
	state core.StateReader,
	opts vm.SimulateOptions,
) (vm.ExecutionResults, error) {
	return tvm.runExec(func(inner vm.VM) (vm.ExecutionResults, error) {
		return inner.Simulate(txns, declaredClasses, blockInfo, state, opts)
	})
}

func (tvm *ThrottledVM) EstimateFee(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
	blockInfo *vm.BlockInfo,
	state core.StateReader,
	opts vm.EstimateFeeOptions,
) (vm.ExecutionResults, error) {
	return tvm.runExec(func(inner vm.VM) (vm.ExecutionResults, error) {
		return inner.EstimateFee(txns, declaredClasses, blockInfo, state, opts)
	})
}

func (tvm *ThrottledVM) Trace(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
	paidFeesOnL1 []*felt.Felt,
	blockInfo *vm.BlockInfo,
	state core.StateReader,
	opts vm.TraceOptions,
) (vm.ExecutionResults, error) {
	return tvm.runExec(func(inner vm.VM) (vm.ExecutionResults, error) {
		return inner.Trace(txns, declaredClasses, paidFeesOnL1, blockInfo, state, opts)
	})
}

func (tvm *ThrottledVM) BuildBlock(
	txns []core.Transaction,
	declaredClasses []core.ClassDefinition,
	paidFeesOnL1 []*felt.Felt,
	blockInfo *vm.BlockInfo,
	state core.StateReader,
	opts vm.BuildBlockOptions,
) (vm.ExecutionResults, error) {
	return tvm.runExec(func(inner vm.VM) (vm.ExecutionResults, error) {
		return inner.BuildBlock(txns, declaredClasses, paidFeesOnL1, blockInfo, state, opts)
	})
}
