package feeder

import (
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

// TransactionFromTestData loads a transaction fixture captured from the legacy
// get_transaction endpoint and adapts it to a core.Transaction.
func TransactionFromTestData(
	t testing.TB,
	network *networks.Network,
	transactionHash *felt.Felt,
) core.Transaction {
	t.Helper()

	tx, err := sn2core.AdaptTransaction(feeder.TransactionFromTestData(t, network, transactionHash))
	require.NoError(t, err)

	return tx
}
