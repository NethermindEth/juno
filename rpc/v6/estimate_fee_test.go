package rpcv6_test

import (
	"encoding/json"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/mocks"
	rpccore "github.com/NethermindEth/juno/rpc/rpccore"
	rpc "github.com/NethermindEth/juno/rpc/v6"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEstimateMessageFee(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	n := &utils.Mainnet
	mockReader := mocks.NewMockReader(mockCtrl)
	mockReader.EXPECT().Network().Return(n).AnyTimes()
	mockVM := mocks.NewMockVM(mockCtrl)

	handler := rpc.New(mockReader, nil, mockVM, "", n, utils.NewNopZapLogger())
	msg := rpc.MsgFromL1{
		From:     common.HexToAddress("0xDEADBEEF"),
		To:       *new(felt.Felt).SetUint64(1337),
		Payload:  []felt.Felt{*new(felt.Felt).SetUint64(1), *new(felt.Felt).SetUint64(2)},
		Selector: *new(felt.Felt).SetUint64(44),
	}

	t.Run("block not found", func(t *testing.T) {
		mockReader.EXPECT().HeadState().Return(nil, nil, db.ErrKeyNotFound)
		_, err := handler.EstimateMessageFee(msg, rpc.BlockID{Latest: true})
		require.Equal(t, rpccore.ErrBlockNotFound, err)
	})

	latestHeader := &core.Header{
		Number:        9,
		Timestamp:     456,
		L1GasPriceETH: new(felt.Felt).SetUint64(42),
	}
	mockState := mocks.NewMockStateHistoryReader(mockCtrl)

	mockReader.EXPECT().HeadState().Return(mockState, nopCloser, nil)
	mockReader.EXPECT().HeadsHeader().Return(latestHeader, nil)

	expectedGasConsumed := new(felt.Felt).SetUint64(37)
	mockVM.EXPECT().Execute(gomock.Any(), gomock.Any(), gomock.Any(), &vm.BlockInfo{
		Header: latestHeader,
	}, gomock.Any(), &utils.Mainnet, gomock.Any(), false, true, false).DoAndReturn(
		func(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt, blockInfo *vm.BlockInfo,
			state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert, errStack bool,
		) (vm.ExecutionResults, error) {
			require.Len(t, txns, 1)
			assert.NotNil(t, txns[0].(*core.L1HandlerTransaction))

			assert.Empty(t, declaredClasses)
			assert.Len(t, paidFeesOnL1, 1)

			actualFee := new(felt.Felt).Mul(expectedGasConsumed, blockInfo.Header.L1GasPriceETH)
			return vm.ExecutionResults{
				OverallFees:      []*felt.Felt{actualFee},
				DataAvailability: []core.DataAvailability{{L1DataGas: 0}},
				GasConsumed:      []core.GasConsumed{{L1Gas: 37}},
				Traces: []vm.TransactionTrace{{
					StateDiff: &vm.StateDiff{
						StorageDiffs:              []vm.StorageDiff{},
						Nonces:                    []vm.Nonce{},
						DeployedContracts:         []vm.DeployedContract{},
						DeprecatedDeclaredClasses: []*felt.Felt{},
						DeclaredClasses:           []vm.DeclaredClass{},
						ReplacedClasses:           []vm.ReplacedClass{},
					},
				}},
				NumSteps: 0,
			}, nil
		},
	)

	estimateFee, err := handler.EstimateMessageFee(msg, rpc.BlockID{Latest: true})
	require.Nil(t, err)
	feeUnit := rpc.WEI
	require.Equal(t, expectedGasConsumed, estimateFee.GasConsumed)
	require.Equal(t, latestHeader.L1GasPriceETH, estimateFee.GasPrice)
	require.Equal(t, new(felt.Felt).Mul(expectedGasConsumed, latestHeader.L1GasPriceETH), estimateFee.OverallFee)
	require.Equal(t, feeUnit, *estimateFee.Unit)
}

func assertEqualCairo0Class(t *testing.T, cairo0Class *core.Cairo0Class, class *rpc.Class) {
	assert.Equal(t, cairo0Class.Program, class.Program)
	assert.Equal(t, cairo0Class.Abi, class.Abi.(json.RawMessage))

	require.Equal(t, len(cairo0Class.L1Handlers), len(class.EntryPoints.L1Handler))
	for idx := range cairo0Class.L1Handlers {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Offset, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo0Class.L1Handlers[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Constructors), len(class.EntryPoints.Constructor))
	for idx := range cairo0Class.Constructors {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo0Class.Constructors[idx].Offset, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo0Class.Constructors[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo0Class.Externals), len(class.EntryPoints.External))
	for idx := range cairo0Class.Externals {
		assert.Nil(t, class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo0Class.Externals[idx].Offset, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo0Class.Externals[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}

func assertEqualCairo1Class(t *testing.T, cairo1Class *core.Cairo1Class, class *rpc.Class) {
	assert.Equal(t, cairo1Class.Program, class.SierraProgram)
	assert.Equal(t, cairo1Class.Abi, class.Abi.(string))
	assert.Equal(t, cairo1Class.SemanticVersion, class.ContractClassVersion)

	require.Equal(t, len(cairo1Class.EntryPoints.L1Handler), len(class.EntryPoints.L1Handler))
	for idx := range cairo1Class.EntryPoints.L1Handler {
		assert.Nil(t, class.EntryPoints.L1Handler[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Index, *class.EntryPoints.L1Handler[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.L1Handler[idx].Selector, class.EntryPoints.L1Handler[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.Constructor), len(class.EntryPoints.Constructor))
	for idx := range cairo1Class.EntryPoints.Constructor {
		assert.Nil(t, class.EntryPoints.Constructor[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Index, *class.EntryPoints.Constructor[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.Constructor[idx].Selector, class.EntryPoints.Constructor[idx].Selector)
	}

	require.Equal(t, len(cairo1Class.EntryPoints.External), len(class.EntryPoints.External))
	for idx := range cairo1Class.EntryPoints.External {
		assert.Nil(t, class.EntryPoints.External[idx].Offset)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Index, *class.EntryPoints.External[idx].Index)
		assert.Equal(t, cairo1Class.EntryPoints.External[idx].Selector, class.EntryPoints.External[idx].Selector)
	}
}
