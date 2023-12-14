package builder_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/mempool"
	"github.com/NethermindEth/juno/mocks"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestValidateAgainstPendingState(t *testing.T) {
	testDB := pebble.NewMemTest(t)
	mockCtrl := gomock.NewController(t)
	mockVM := mocks.NewMockVM(mockCtrl)
	bc := blockchain.New(testDB, &utils.Integration)
	seqAddr := utils.HexToFelt(t, "0xDEADBEEF")
	testBuilder := builder.New(seqAddr, bc, mockVM, utils.NewNopZapLogger())

	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)

	su, b, err := gw.StateUpdateWithBlock(context.Background(), 0)
	require.NoError(t, err)

	require.NoError(t, bc.StorePending(&blockchain.Pending{
		Block:       b,
		StateUpdate: su,
	}))

	userTxn := mempool.BroadcastedTransaction{
		Transaction: &core.InvokeTransaction{
			TransactionHash: utils.HexToFelt(t, "0x1337"),
		},
		DeclaredClass: &core.Cairo0Class{
			Program: "best program",
		},
	}
	pendingBlock, err := bc.Pending()
	require.NoError(t, err)
	blockInfo := &vm.BlockInfo{
		Header: &core.Header{
			Number:           pendingBlock.Block.Number,
			Timestamp:        pendingBlock.Block.Timestamp,
			SequencerAddress: seqAddr,
			GasPrice:         pendingBlock.Block.GasPrice,
			GasPriceSTRK:     pendingBlock.Block.GasPriceSTRK,
		},
	}

	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, []*felt.Felt{}, gomock.Any(),
		gomock.Any(), &utils.Integration, false, false, false, false).DoAndReturn(
		func(txns []core.Transaction, classes []core.Class, felts []*felt.Felt, info *vm.BlockInfo, any interface{}, integration *utils.Network, b1, b2, b3, b4 bool) ([]*core.Event, []*core.Event, []*core.Event, error) {
			// Check all fields of info except for Timestamp
			if info.Header.Number != blockInfo.Header.Number {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: Number. Expected %v, got %v", blockInfo.Header.Number, info.Header.Number)
			}
			if info.Header.SequencerAddress.String() != blockInfo.Header.SequencerAddress.String() {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: SequencerAddress. Expected %v, got %v", blockInfo.Header.SequencerAddress, info.Header.SequencerAddress)
			}
			if info.Header.GasPrice != blockInfo.Header.GasPrice {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPrice. Expected %v, got %v", blockInfo.Header.GasPrice, info.Header.GasPrice)
			}
			if info.Header.GasPriceSTRK != blockInfo.Header.GasPriceSTRK {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPriceSTRK. Expected %v, got %v", blockInfo.Header.GasPriceSTRK, info.Header.GasPriceSTRK)
			}

			// Call the real Execute function
			return nil, nil, nil, nil
		})
	assert.NoError(t, testBuilder.ValidateAgainstPendingState(&userTxn))

	blockInfo.Header.Number += 1

	require.NoError(t, bc.Store(b, &core.BlockCommitments{}, su, nil))
	mockVM.EXPECT().Execute([]core.Transaction{userTxn.Transaction},
		[]core.Class{userTxn.DeclaredClass}, []*felt.Felt{}, gomock.Any(),
		gomock.Any(), &utils.Integration, false, false, false, false).DoAndReturn(
		func(txns []core.Transaction, classes []core.Class, felts []*felt.Felt, info *vm.BlockInfo, any interface{}, integration *utils.Network, b1, b2, b3, b4 bool) ([]*core.Event, []*core.Event, []*core.Event, error) {
			if info.Header.Number != blockInfo.Header.Number {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: Number. Expected %v, got %v", blockInfo.Header.Number, info.Header.Number)
			}
			if info.Header.SequencerAddress.String() != blockInfo.Header.SequencerAddress.String() {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: SequencerAddress. Expected %v, got %v", blockInfo.Header.SequencerAddress, info.Header.SequencerAddress)
			}
			if info.Header.GasPrice != blockInfo.Header.GasPrice {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPrice. Expected %v, got %v", blockInfo.Header.GasPrice, info.Header.GasPrice)
			}
			if info.Header.GasPriceSTRK != blockInfo.Header.GasPriceSTRK {
				return nil, nil, nil, fmt.Errorf("unexpected BlockInfo: GasPriceSTRK. Expected %v, got %v", blockInfo.Header.GasPriceSTRK, info.Header.GasPriceSTRK)
			}

			return nil, nil, nil, errors.New("oops")
		})
	assert.EqualError(t, testBuilder.ValidateAgainstPendingState(&userTxn), "oops")
}
