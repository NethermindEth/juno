package node

import (
	"encoding/json"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
)

var _ vm.VM = (*ThrottledVM)(nil)

type ThrottledVM utils.Throttler[vm.VM]

func NewThrottledVM(res vm.VM, concurrenyBudget uint, maxQueueLen int32) *ThrottledVM {
	return (*ThrottledVM)(utils.NewThrottler[vm.VM](concurrenyBudget, &res).WithMaxQueueLen(maxQueueLen))
}

func (tvm *ThrottledVM) Call(contractAddr, classHash, selector *felt.Felt, calldata []felt.Felt, blockNumber,
	blockTimestamp uint64, state core.StateReader, network utils.Network,
) ([]*felt.Felt, error) {
	var ret []*felt.Felt
	throttler := (*utils.Throttler[vm.VM])(tvm)
	return ret, throttler.Do(func(vm *vm.VM) error {
		var err error
		ret, err = (*vm).Call(contractAddr, classHash, selector, calldata, blockNumber, blockTimestamp, state, network)
		return err
	})
}

func (tvm *ThrottledVM) Execute(txns []core.Transaction, declaredClasses []core.Class, blockNumber, blockTimestamp uint64,
	sequencerAddress *felt.Felt, state core.StateReader, network utils.Network, paidFeesOnL1 []*felt.Felt,
	skipChargeFee, skipValidate bool, gasPriceWEI *felt.Felt, gasPriceSTRK *felt.Felt, legacyTraceJSON bool,
) ([]*felt.Felt, []json.RawMessage, error) {
	var ret []*felt.Felt
	var traces []json.RawMessage
	throttler := (*utils.Throttler[vm.VM])(tvm)
	return ret, traces, throttler.Do(func(vm *vm.VM) error {
		var err error
		ret, traces, err = (*vm).Execute(txns, declaredClasses, blockNumber, blockTimestamp, sequencerAddress,
			state, network, paidFeesOnL1, skipChargeFee, skipValidate, gasPriceWEI, gasPriceSTRK, legacyTraceJSON)
		return err
	})
}
