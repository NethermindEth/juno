package statemachine_test

import (
	"testing"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/consensus/p2p/statemachine"
	"github.com/NethermindEth/juno/consensus/starknet"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/genesis"
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/vm"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
	"github.com/stretchr/testify/require"
)

// Todo:clean, and move.
func hexToCommonHash(t *testing.T, in string) *common.Hash {
	inFelt, err := new(felt.Felt).SetString(in)
	require.NoError(t, err)
	inBytes := inFelt.Bytes()
	return &common.Hash{Elements: inBytes[:]}
}

func hexToCommonAddress(t *testing.T, in string) *common.Address {
	inFelt, err := new(felt.Felt).SetString(in)
	require.NoError(t, err)
	inBytes := inFelt.Bytes()
	return &common.Address{Elements: inBytes[:]}
}

func hexToCommonFelt252(t *testing.T, in string) *common.Felt252 {
	inFelt, err := new(felt.Felt).SetString(in)
	require.NoError(t, err)
	inBytes := inFelt.Bytes()
	return &common.Felt252{Elements: inBytes[:]}
}

func TestTransition(t *testing.T) {
	bc, vm := getTransitionInputs(t)
	transition := statemachine.NewTransition[value, starknet.Hash, starknet.Address](bc, vm, utils.NewNopZapLogger(), false)

	// We can't use random values here since 1. certain values need to match those in the header,
	// and 2. it will alter the block hash, and we need a fixed expected value
	proposerAddress := hexToCommonAddress(t, "0x1")
	zeroConcat := hexToCommonFelt252(t, "0x0")
	someHash := hexToCommonHash(t, "0x1")
	zeroHashCommitment := hexToCommonHash(t, "0x0")
	headerFeesU128 := &common.Uint128{Low: 0, High: 0}

	timestamp := uint64(0)
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
		commitState := &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{
			Header:     header,
			ValidRound: -1,
		}
		proposalCommitMsg := &consensus.ProposalCommitment{
			BlockNumber:               height,
			ParentCommitment:          hexToCommonHash(t, "0x36a24bd21cdd58b47aaa18342781ab3a5a938ff775d72eca9f4528e89e90e08"),
			Builder:                   proposerAddress,
			Timestamp:                 timestamp,
			ProtocolVersion:           "0.12.3",
			OldStateRoot:              hexToCommonHash(t, "0x5eed4c967bf69f5574663f19635c031c03294283827119495aa3d4b14f55b8d"),
			VersionConstantCommitment: someHash, // Todo: update when we support this
			StateDiffCommitment:       zeroHashCommitment,
			TransactionCommitment:     zeroHashCommitment,
			EventCommitment:           zeroHashCommitment,
			ReceiptCommitment:         zeroHashCommitment,
			ConcatenatedCounts:        zeroConcat,
			L1GasPriceFri:             headerFeesU128,
			L1DataGasPriceFri:         headerFeesU128,
			L2GasPriceFri:             headerFeesU128,
			L2GasUsed:                 headerFeesU128,
			NextL2GasPriceFri:         headerFeesU128,
			L1DaMode:                  common.L1DataAvailabilityMode_Calldata,
		}
		_, err = transition.OnProposalCommitment(t.Context(), commitState, proposalCommitMsg)
		require.NoError(t, err)

		// 3. OnProposalFin
		finState := &statemachine.AwaitingProposalFinState[value, starknet.Hash, starknet.Address]{
			Header:     header,
			ValidRound: -1,
		}
		expectedProposalFin := "0x2c0c3d895adb9535198914fe603fd291d8d0eabc9f70b1e41ef0cacc306d60f"
		proposalFinMsg := &consensus.ProposalFin{
			ProposalCommitment: hexToCommonHash(t, expectedProposalFin),
		}
		proposalFin, err := transition.OnProposalFin(t.Context(), finState, proposalFinMsg)
		require.NoError(t, err)
		finalValue := proposalFin.Value.Hash()
		require.Equal(t, expectedProposalFin, finalValue.String())
	})

	t.Run("Valid NonEmpty Block", func(t *testing.T) {
		// The txn we try to execute in this test returns an 'argent/invalid-tx-version' error
		// despite all the fields being correctly populated.
		// {"query_bit":false,"txn":{"Invoke":{"V3":{"version":"0x3","sender_address":"0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923","signature":["0x2df74069c1e88187be5f091cfdd5cddaa1b8359d2b71db631f0ea0d47a8ebe6","0x78b112b02974ba8a960a7b95966153ff7cf1552abb06a15074a7baba093867c"],"calldata":["0x1","0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7","0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e","0x3","0x101","0x12345678","0x0"],"nonce":"0x0","resource_bounds":{"L1_DATA":{"max_amount":"0x1","max_price_per_unit":"0x1"},"L1_GAS":{"max_amount":"0x1","max_price_per_unit":"0x1"},"L2_GAS":{"max_amount":"0x1","max_price_per_unit":"0x1"}},"tip":"0x0","nonce_data_availability_mode":"L1","fee_data_availability_mode":"L1","account_deployment_data":[],"paymaster_data":[]}}},"txn_hash":"0x7966d2619e2e4ebae2c3ea75f60e36b492f281b9fb22581b8cb810832075c1"}
		// I think this might be because the argent account we use doesn't support V3 txns. In which case, we need to declare an upated Class that can handle v3 txns.
		t.Skip("Tendermint test skipped for now")
		feesU128 := &common.Uint128{Low: 0, High: 0}
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
			L2GasPriceFri:     feesU128,
			L1GasPriceWei:     feesU128,
			L1DataGasPriceWei: feesU128,
			EthToStrkRate:     feesU128,
			L1DaMode:          common.L1DataAvailabilityMode_Blob,
		}
		waitingState := &statemachine.AwaitingBlockInfoOrCommitmentState[value, starknet.Hash, starknet.Address]{
			Header:     header,
			ValidRound: -1,
		}
		_, err = transition.OnBlockInfo(t.Context(), waitingState, onBlockInfoMsg)
		require.NoError(t, err)

		// 3. OnTransactions
		txnsMsg := []*consensus.ConsensusTransaction{
			{
				Txn:             getConsensusTxn(t),
				TransactionHash: hexToCommonHash(t, "0x7966d2619e2e4ebae2c3ea75f60e36b492f281b9fb22581b8cb810832075c1"),
			},
		}
		txnState := &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{
			Header:     header,
			ValidRound: -1,
		}

		_, err = transition.OnTransactions(t.Context(), txnState, txnsMsg)
		require.NoError(t, err)

		// 4. OnProposalCommitment
		commitState := &statemachine.ReceivingTransactionsState[value, starknet.Hash, starknet.Address]{
			Header:     header,
			ValidRound: -1,
		}
		proposalCommitMsg := &consensus.ProposalCommitment{
			BlockNumber:               height,
			ParentCommitment:          hexToCommonHash(t, "0x36a24bd21cdd58b47aaa18342781ab3a5a938ff775d72eca9f4528e89e90e08"),
			Builder:                   proposerAddress,
			Timestamp:                 timestamp,
			ProtocolVersion:           "0.12.3",
			OldStateRoot:              hexToCommonHash(t, "0x5eed4c967bf69f5574663f19635c031c03294283827119495aa3d4b14f55b8d"),
			VersionConstantCommitment: someHash, // Todo: update when we support this
			StateDiffCommitment:       hexToCommonHash(t, "0x49973925542c74a9d9ff0efaa98c61e1225d0aedb708092433cbbb20836d30a"),
			TransactionCommitment:     hexToCommonHash(t, "0x0"), // Todo: This should NOT be 0??
			EventCommitment:           hexToCommonHash(t, "0x0"), // Todo: This should NOT be 0??
			ReceiptCommitment:         hexToCommonHash(t, "0x0"), // Todo: This should NOT be 0??
			ConcatenatedCounts:        hexToCommonFelt252(t, "0x8000000000000000"),
			L1GasPriceFri:             headerFeesU128,
			L1DataGasPriceFri:         headerFeesU128,
			L2GasPriceFri:             headerFeesU128,
			L2GasUsed:                 headerFeesU128,
			NextL2GasPriceFri:         headerFeesU128,
			L1DaMode:                  common.L1DataAvailabilityMode_Blob,
		}
		_, err = transition.OnProposalCommitment(t.Context(), commitState, proposalCommitMsg)
		require.NoError(t, err)

		// 5. OnProposalFin
		finState := &statemachine.AwaitingProposalFinState[value, starknet.Hash, starknet.Address]{
			Header:     header,
			ValidRound: -1,
		}
		proposalFinMsg := &consensus.ProposalFin{
			ProposalCommitment: hexToCommonHash(t, "0xcaa5d3f27a87e885a4767c1fd9f18355aec825988e2bf548fd78d13629d538"),
		}
		_, err = transition.OnProposalFin(t.Context(), finState, proposalFinMsg)
		require.NoError(t, err)
	})
}

// Just to keep the code tidier
func getConsensusTxn(t *testing.T) *consensus.ConsensusTransaction_InvokeV3 {
	// transfer tokens to 0x101
	return &consensus.ConsensusTransaction_InvokeV3{
		InvokeV3: &transaction.InvokeV3{
			Sender: hexToCommonAddress(t, "0x406a8f52e741619b17410fc90774e4b36f968e1a71ae06baacfe1f55d987923"),
			Signature: &transaction.AccountSignature{
				Parts: []*common.Felt252{
					hexToCommonFelt252(t, "0x2df74069c1e88187be5f091cfdd5cddaa1b8359d2b71db631f0ea0d47a8ebe6"),
					hexToCommonFelt252(t, "0x78b112b02974ba8a960a7b95966153ff7cf1552abb06a15074a7baba093867c"),
				},
			},
			Calldata: []*common.Felt252{
				hexToCommonFelt252(t, "0x1"),
				hexToCommonFelt252(t, "0x49d36570d4e46f48e99674bd3fcc84644ddd6b96f7c741b1562b82f9e004dc7"),
				hexToCommonFelt252(t, "0x83afd3f4caedc6eebf44246fe54e38c95e3179a5ec9ea81740eca5b482d12e"),
				hexToCommonFelt252(t, "0x3"),
				hexToCommonFelt252(t, "0x101"),
				hexToCommonFelt252(t, "0x12345678"),
				hexToCommonFelt252(t, "0x0"),
			},
			Nonce: hexToCommonFelt252(t, "0x0"),
			ResourceBounds: &transaction.ResourceBounds{
				L1Gas: &transaction.ResourceLimits{
					MaxAmount:       hexToCommonFelt252(t, "0x1"),
					MaxPricePerUnit: hexToCommonFelt252(t, "0x1"),
				},
				L1DataGas: &transaction.ResourceLimits{
					MaxAmount:       hexToCommonFelt252(t, "0x1"),
					MaxPricePerUnit: hexToCommonFelt252(t, "0x1"),
				},
				L2Gas: &transaction.ResourceLimits{
					MaxAmount:       hexToCommonFelt252(t, "0x1"),
					MaxPricePerUnit: hexToCommonFelt252(t, "0x1"),
				},
			},
		},
	}
}

func getTransitionInputs(t *testing.T) (*blockchain.Blockchain, vm.VM) {
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
	vm := vm.New(false, log)
	diff, classes, err := genesis.GenesisStateDiff(genesisConfig, vm, bc.Network(), 40000000) //nolint:gomnd
	require.NoError(t, err)
	require.NoError(t, bc.StoreGenesis(&diff, classes))
	return bc, vm
}
