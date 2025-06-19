package statemachine

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
	"github.com/stretchr/testify/require"
)

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

func ToBytes(felt felt.Felt) []byte {
	feltBytes := felt.Bytes()
	return feltBytes[:]
}

func getRandomFelt(t *testing.T) []byte {
	t.Helper()

	addr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	addrBytes := addr.Bytes()
	return addrBytes[:]
}

func GetRandomAddress(t *testing.T) *common.Address {
	t.Helper()

	addr, err := new(felt.Felt).SetRandom()
	require.NoError(t, err)

	return &common.Address{Elements: ToBytes(*addr)}
}

func getRandomTransaction(t *testing.T) *consensus.ConsensusTransaction {
	t.Helper()
	address := &common.Address{Elements: []byte{1}}
	someFelt252 := &common.Felt252{Elements: []byte{0x42}}
	return &consensus.ConsensusTransaction{
		Txn: &consensus.ConsensusTransaction_InvokeV3{
			InvokeV3: &transaction.InvokeV3{
				Sender: address,
				Signature: &transaction.AccountSignature{
					Parts: []*common.Felt252{
						{Elements: []byte{0xAA}},
						{Elements: []byte{0xBB}},
					},
				},
				Calldata: []*common.Felt252{
					{Elements: []byte{0x01}},
					{Elements: []byte{0x02}},
				},
				ResourceBounds: &transaction.ResourceBounds{
					L1Gas: &transaction.ResourceLimits{
						MaxAmount:       someFelt252,
						MaxPricePerUnit: someFelt252,
					},
					L2Gas: &transaction.ResourceLimits{
						MaxAmount:       someFelt252,
						MaxPricePerUnit: someFelt252,
					},
					L1DataGas: &transaction.ResourceLimits{
						MaxAmount:       someFelt252,
						MaxPricePerUnit: someFelt252,
					},
				},
				Tip:                       100,
				PaymasterData:             []*common.Felt252{someFelt252},
				AccountDeploymentData:     []*common.Felt252{someFelt252},
				NonceDataAvailabilityMode: common.VolitionDomain_L1,
				FeeDataAvailabilityMode:   common.VolitionDomain_L1,
				Nonce:                     someFelt252,
			},
		},
		TransactionHash: &common.Hash{Elements: getRandomFelt(t)},
	}
}

func GetRandomTransactions(t *testing.T, count int) []*consensus.ConsensusTransaction {
	t.Helper()
	transactions := make([]*consensus.ConsensusTransaction, count)
	for i := range transactions {
		transactions[i] = getRandomTransaction(t)
	}
	return transactions
}

func getExampleProposalCommitment(t *testing.T, blockNumber, timestamp uint64, sender *common.Address) *consensus.ProposalCommitment {
	t.Helper()
	someFelt := &common.Felt252{Elements: []byte{1}}
	someHash := &common.Hash{Elements: []byte{1}}
	someU128 := &common.Uint128{Low: 1, High: 2}
	return &consensus.ProposalCommitment{
		BlockNumber:               blockNumber,
		ParentCommitment:          someHash,
		Builder:                   sender,
		Timestamp:                 timestamp,
		ProtocolVersion:           "0.12.3",
		OldStateRoot:              someHash,
		VersionConstantCommitment: someHash,
		StateDiffCommitment:       someHash,
		TransactionCommitment:     someHash,
		EventCommitment:           someHash,
		ReceiptCommitment:         someHash,
		ConcatenatedCounts:        someFelt,
		L1GasPriceFri:             someU128,
		L1DataGasPriceFri:         someU128,
		L2GasPriceFri:             someU128,
		L2GasUsed:                 someU128,
		NextL2GasPriceFri:         someU128,
		L1DaMode:                  common.L1DataAvailabilityMode_Blob,
	}
}
