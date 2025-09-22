package validator

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/common"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/consensus/consensus"
	"github.com/starknet-io/starknet-p2pspecs/p2p/proto/transaction"
	"github.com/stretchr/testify/require"
)

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
	return &consensus.ConsensusTransaction{
		Txn:             &consensus.ConsensusTransaction_InvokeV3{InvokeV3: &transaction.InvokeV3{}},
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
