package statemachine_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/builder"
	"github.com/NethermindEth/juno/consensus/p2p/statemachine"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/validator"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/stretchr/testify/require"
)

func TestTransitionInvalidMsgs(t *testing.T) {
	builder := getGenesisBuilder(t)
	val := validator.New[value, felt.Felt, felt.Felt](&builder)
	transition := statemachine.NewTransition[value, felt.Felt, felt.Felt](val)
	t.Run("Invalid OnProposalInit", func(t *testing.T) {
		initMsg := &consensus.ProposalInit{}
		_, err := transition.OnProposalInit(t.Context(), nil, initMsg)
		require.Error(t, err)
	})
}

func TestTransition(t *testing.T) {
	builder := getGenesisBuilder(t)
	val := validator.New[value, felt.Felt, felt.Felt](&builder)
	transition := statemachine.NewTransition[value, felt.Felt, felt.Felt](val)

	proposerAddress := &common.Address{Elements: []byte{1}}
	someFelt := &common.Felt252{Elements: []byte{1}}
	someHash := &common.Hash{Elements: []byte{1}}
	someU128 := &common.Uint128{Low: 1, High: 2}
	timestamp := uint64(123)
	height := uint64(1)
	round := 0
	header := &starknet.MessageHeader{
		Height: types.Height(height),
		Round:  types.Round(round),
		Sender: starknet.Address(*new(felt.Felt).SetBytes(proposerAddress.Elements)),
	}

	t.Run("Valid Empty Block", func(t *testing.T) {
		// 1. OnProposalInit
		initMsg := &consensus.ProposalInit{
			BlockNumber: height,
			Round:       0,
			ValidRound:  nil,
			Proposer:    proposerAddress,
		}
		_, err := transition.OnProposalInit(t.Context(), nil, initMsg)
		require.NoError(t, err)

		// 2. OnProposalCommitment
		commitState := &statemachine.ReceivingTransactionsState{
			Header:     header,
			ValidRound: -1,
			Value:      nil,
		}
		proposalCommitMsg := &consensus.ProposalCommitment{
			BlockNumber:               height,
			ParentCommitment:          someHash, // Todo
			Builder:                   proposerAddress,
			Timestamp:                 timestamp,
			ProtocolVersion:           "0.12.3",
			OldStateRoot:              someHash, // Todo
			VersionConstantCommitment: someHash, // Todo
			StateDiffCommitment:       someHash, // Todo
			TransactionCommitment:     someHash, // Todo
			EventCommitment:           someHash, // Todo
			ReceiptCommitment:         someHash, // Todo
			ConcatenatedCounts:        someFelt, // Todo
			L1GasPriceFri:             someU128,
			L1DataGasPriceFri:         someU128,
			L2GasPriceFri:             someU128,
			L2GasUsed:                 someU128,
			NextL2GasPriceFri:         someU128,
			L1DaMode:                  common.L1DataAvailabilityMode_Blob,
		}
		_, err = transition.OnProposalCommitment(t.Context(), commitState, proposalCommitMsg)
		require.NoError(t, err)

		// 3. OnProposalFin
		finState := &statemachine.AwaitingProposalFinState{
			Header:     header,
			ValidRound: -1,
			Value:      nil,
		}
		proposalFinMsg := &consensus.ProposalFin{
			ProposalCommitment: &common.Hash{Elements: []byte{123}}, // Todo
		}
		_, err = transition.OnProposalFin(t.Context(), finState, proposalFinMsg)
		require.NoError(t, err)

	})

	t.Run("Valid NonEmpty Block", func(t *testing.T) { // Todo
		// 1. OnProposalInit
		initMsg := &consensus.ProposalInit{
			BlockNumber: height,
			Round:       0,
			ValidRound:  nil,
			Proposer:    proposerAddress,
		}
		_, err := transition.OnProposalInit(t.Context(), nil, initMsg)
		require.NoError(t, err)

		// 2. OnBlockInfo
		onBlockInfoMsg := &consensus.BlockInfo{
			BlockNumber:       height,
			Builder:           proposerAddress,
			Timestamp:         123,
			L2GasPriceFri:     someU128,
			L1GasPriceWei:     someU128,
			L1DataGasPriceWei: someU128,
			EthToStrkRate:     someU128,
			L1DaMode:          common.L1DataAvailabilityMode_Blob,
		}
		waitingState := &statemachine.AwaitingBlockInfoOrCommitmentState{
			Header:     header,
			ValidRound: -1,
		}
		_, err = transition.OnBlockInfo(t.Context(), waitingState, onBlockInfoMsg)
		require.NoError(t, err)

		// 3. OnTransactions
		txnsMsg := []*consensus.ConsensusTransaction{
			{
				// Txn: getConsensusTxn(),
				// TransactionHash: Todo,
			},
		}
		txnState := &statemachine.ReceivingTransactionsState{
			Header:     header,
			ValidRound: -1,
			Value:      nil,
		}
		_, err = transition.OnTransactions(t.Context(), txnState, txnsMsg)
		require.NoError(t, err)
	})

}

// Just to keep the code tidier
// func getConsensusTxn() *consensus.ConsensusTransaction {
// 	// transfer tokens to 0x101
// 	return &consensus.ConsensusTransaction{
// 		Txn: &consensus.ConsensusTransaction_InvokeV3{
// 			InvokeV3: &transaction.InvokeV3{
// 				Sender: utils.HexToFelt(t, "0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
// 				Signature: &transaction.AccountSignature{
// 					Parts: []*common.Felt252{
// 						utils.HexToFelt(t, "0x239a9d44d7b7dd8d31ba0d848072c22643beb2b651d4e2cd8a9588a17fd6811"),
// 						utils.HexToFelt(t, "0x6e7d805ee0cc02f3790ab65c8bb66b235341f97d22d6a9a47dc6e4fdba85972"),
// 					},
// 				},
// 				CallData: &[]*felt.Felt{
// 					utils.HexToFelt(t, "0x1"),
// 					utils.HexToFelt(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
// 					utils.HexToFelt(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
// 					utils.HexToFelt(t, "0x3"),
// 					utils.HexToFelt(t, "0x101"),
// 					utils.HexToFelt(t, "0x12345678"),
// 					utils.HexToFelt(t, "0x0"),
// 				},
// 				Nonce: new(felt.Felt).SetUint64(0),
// 			},
// 		},
// 	}
// }

func getGenesisBuilder(t *testing.T) builder.Builder {
	t.Helper()

	testDB := memory.New()
	network := &utils.Mainnet
	bc := blockchain.New(testDB, network)
	log := utils.NewNopZapLogger()

	genesisConfig, err := genesis.Read("../../../genesis/genesis_prefund_accounts.json")
	require.NoError(t, err)
	genesisConfig.Classes = []string{
		"../../../genesis/classes/strk.json", "../../../genesis/classes/account.json",
		"../../../genesis/classes/universaldeployer.json", "../../../genesis/classes/udacnt.json",
	}
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm.New(false, log), bc.Network(), 40000000) //nolint:gomnd
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	return builder.New(bc, vm.New(false, log), log, false)
}
