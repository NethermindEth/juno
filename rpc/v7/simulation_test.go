package rpcv7_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv7 "github.com/NethermindEth/juno/rpc/v7"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//nolint:dupl
func TestSimulateTransactions(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	n := &utils.Mainnet

	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)
	handler := rpcv7.New(mockReader, nil, mockVM, "", n, utils.NewNopZapLogger())

	mockState := mocks.NewMockStateHistoryReader(mockCtrl)
	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil).AnyTimes()
	headsHeader := &core.Header{
		SequencerAddress: n.BlockHashMetaInfo.FallBackSequencerAddress,
	}
	mockReader.EXPECT().HeadsHeader().Return(headsHeader, nil).AnyTimes()

	t.Run("ok with zero values, skip fee", func(t *testing.T) {
		stepsUsed := uint64(123)
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, true, false, false, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         stepsUsed,
			}, nil)

		_, httpHeader, err := handler.SimulateTransactions(
			rpcv7.BlockID{Latest: true},
			[]rpcv7.BroadcastedTransaction{},
			[]rpcv6.SimulationFlag{rpcv6.SkipFeeChargeFlag},
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "123")
	})

	t.Run("ok with zero values, skip validate", func(t *testing.T) {
		stepsUsed := uint64(123)
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, false, true, false, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{},
				DataAvailability: []core.DataAvailability{},
				Traces:           []vm.TransactionTrace{},
				NumSteps:         stepsUsed,
			}, nil)

		_, httpHeader, err := handler.SimulateTransactions(
			rpcv7.BlockID{Latest: true},
			[]rpcv7.BroadcastedTransaction{},
			[]rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
		)
		require.Nil(t, err)
		assert.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "123")
	})

	t.Run("transaction execution error", func(t *testing.T) {
		t.Run("v0_7, v0_8", func(t *testing.T) { //nolint:dupl
			mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
				Header: headsHeader,
			}, mockState, n, false, true, false, false).
				Return(vm.ExecutionResults{}, vm.TransactionExecutionError{
					Index: 44,
					Cause: json.RawMessage("oops"),
				})

			_, httpHeader, err := handler.SimulateTransactions(
				rpcv7.BlockID{Latest: true},
				[]rpcv7.BroadcastedTransaction{},
				[]rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
			)
			require.Equal(t, rpccore.ErrTransactionExecutionError.CloneWithData(rpcv7.TransactionExecutionErrorData{
				TransactionIndex: 44,
				ExecutionError:   json.RawMessage("oops"),
			}), err)
			require.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
		})
	})

	t.Run("incosistant length error", func(t *testing.T) {
		mockVM.EXPECT().Execute([]core.Transaction{}, nil, []*felt.Felt{}, &vm.BlockInfo{
			Header: headsHeader,
		}, mockState, n, false, true, false, false).
			Return(vm.ExecutionResults{
				OverallFees:      []*felt.Felt{&felt.Zero},
				DataAvailability: []core.DataAvailability{{L1Gas: 0}, {L1Gas: 0}},
				GasConsumed:      []core.GasConsumed{{L1Gas: 0, L1DataGas: 0, L2Gas: 0}},
				Traces:           []vm.TransactionTrace{{}},
				NumSteps:         uint64(0),
			}, nil)

		_, httpHeader, err := handler.SimulateTransactions(
			rpcv7.BlockID{Latest: true},
			[]rpcv7.BroadcastedTransaction{},
			[]rpcv6.SimulationFlag{rpcv6.SkipValidateFlag},
		)
		require.Equal(t, rpccore.ErrInternal.CloneWithData(errors.New(
			"inconsistent lengths: 1 overall fees, 1 traces, 1 gas consumed, 2 data availability, 0 txns",
		)), err)
		require.Equal(t, httpHeader.Get(rpcv7.ExecutionStepsHeader), "0")
	})
}
