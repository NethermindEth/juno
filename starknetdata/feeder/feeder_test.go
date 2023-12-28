package feeder_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/feeder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockByNumber(t *testing.T) {
	numbers := []uint64{147, 11817}

	client := feeder.NewTestClient(t, utils.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, number := range numbers {
		t.Run("mainnet block number "+strconv.FormatUint(number, 10), func(t *testing.T) {
			response, err := client.Block(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			sig, err := client.Signature(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			block, err := adapter.BlockByNumber(ctx, number)
			require.NoError(t, err)
			adaptedResponse, err := sn2core.AdaptBlock(response, sig)
			require.NoError(t, err)
			assert.Equal(t, adaptedResponse, block)
		})
	}
}

func TestBlockLatest(t *testing.T) {
	client := feeder.NewTestClient(t, utils.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	response, err := client.Block(ctx, "latest")
	require.NoError(t, err)
	sig, err := client.Signature(ctx, "latest")
	require.NoError(t, err)
	block, err := adapter.BlockLatest(ctx)
	require.NoError(t, err)
	adaptedResponse, err := sn2core.AdaptBlock(response, sig)
	require.NoError(t, err)
	assert.Equal(t, adaptedResponse, block)
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client := feeder.NewTestClient(t, utils.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, number := range numbers {
		t.Run("number "+strconv.FormatUint(number, 10), func(t *testing.T) {
			response, err := client.StateUpdate(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			feederUpdate, err := adapter.StateUpdate(ctx, number)
			require.NoError(t, err)

			adaptedResponse, err := sn2core.AdaptStateUpdate(response)
			require.NoError(t, err)
			assert.Equal(t, adaptedResponse, feederUpdate)
		})
	}
}

func TestClassV0(t *testing.T) {
	classHashes := []string{
		"0x79e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
		"0x1924aa4b0bedfd884ea749c7231bafd91650725d44c91664467ffce9bf478d0",
		"0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		"0x56b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
	}

	client := feeder.NewTestClient(t, utils.Goerli)
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, hashString := range classHashes {
		t.Run("hash "+hashString, func(t *testing.T) {
			hash := utils.HexToFelt(t, hashString)
			response, err := client.ClassDefinition(ctx, hash)
			require.NoError(t, err)
			classGeneric, err := adapter.Class(ctx, hash)
			require.NoError(t, err)

			adaptedResponse, err := sn2core.AdaptCairo0Class(response.V0)
			require.NoError(t, err)
			require.Equal(t, adaptedResponse, classGeneric)
		})
	}
}

func TestTransaction(t *testing.T) {
	clientGoerli := feeder.NewTestClient(t, utils.Goerli)
	adapterGoerli := adaptfeeder.New(clientGoerli)

	clientMainnet := feeder.NewTestClient(t, utils.Mainnet)
	adapterMainnet := adaptfeeder.New(clientMainnet)

	ctx := context.Background()

	t.Run("invoke transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x7e3a229febf47c6edfd96582d9476dd91a58a5ba3df4553ae448a14a2f132d9")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		invokeTx, ok := txn.(*core.InvokeTransaction)
		require.True(t, ok)
		assert.Equal(t, sn2core.AdaptInvokeTransaction(responseTx), invokeTx)
	})

	t.Run("deploy transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x15b51c2f4880b1e7492d30ada7254fc59c09adde636f37eb08cdadbd9dabebb")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		deployTx, ok := txn.(*core.DeployTransaction)
		require.True(t, ok)
		assert.Equal(t, sn2core.AdaptDeployTransaction(responseTx), deployTx)
	})

	t.Run("deploy account transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		deployAccountTx, ok := txn.(*core.DeployAccountTransaction)
		require.True(t, ok)
		assert.Equal(t, sn2core.AdaptDeployAccountTransaction(responseTx), deployAccountTx)
	})

	t.Run("declare transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x6eab8252abfc9bbfd72c8d592dde4018d07ce467c5ce922519d7142fcab203f")
		response, err := clientGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterGoerli.Transaction(ctx, hash)
		require.NoError(t, err)
		declareTx, ok := txn.(*core.DeclareTransaction)
		require.True(t, ok)
		assert.Equal(t, sn2core.AdaptDeclareTransaction(responseTx), declareTx)
	})

	t.Run("l1handler transaction", func(t *testing.T) {
		hash := utils.HexToFelt(t, "0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8")
		response, err := clientMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		responseTx := response.Transaction

		txn, err := adapterMainnet.Transaction(ctx, hash)
		require.NoError(t, err)
		l1HandlerTx, ok := txn.(*core.L1HandlerTransaction)
		require.True(t, ok)
		assert.Equal(t, sn2core.AdaptL1HandlerTransaction(responseTx), l1HandlerTx)
	})
}

func TestClassV1(t *testing.T) {
	client := feeder.NewTestClient(t, utils.Integration)
	adapter := adaptfeeder.New(client)

	tests := []struct {
		classHash        *felt.Felt
		hasCompiledClass bool
	}{
		{
			classHash:        utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5"),
			hasCompiledClass: true,
		},
		{
			classHash:        utils.HexToFelt(t, "0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c"),
			hasCompiledClass: false,
		},
	}

	for _, test := range tests {
		class, err := adapter.Class(context.Background(), test.classHash)
		require.NoError(t, err)

		feederClass, err := client.ClassDefinition(context.Background(), test.classHash)
		require.NoError(t, err)
		compiled, err := client.CompiledClassDefinition(context.Background(), test.classHash)
		if test.hasCompiledClass {
			require.NoError(t, err)
		} else {
			require.EqualError(t, err, "deprecated compiled class")
		}

		adaptedResponse, err := sn2core.AdaptCairo1Class(feederClass.V1, compiled)
		require.NoError(t, err)
		assert.Equal(t, adaptedResponse, class)

		if test.hasCompiledClass {
			assert.NotNil(t, adaptedResponse.Compiled)
		} else {
			assert.Nil(t, adaptedResponse.Compiled)
		}
	}
}

func TestStateUpdateWithBlock(t *testing.T) {
	numbers := []uint64{0, 78541}

	client := feeder.NewTestClient(t, utils.Integration)
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	for _, number := range numbers {
		t.Run("integration block number "+strconv.FormatUint(number, 10), func(t *testing.T) {
			response, err := client.StateUpdateWithBlock(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			sig, err := client.Signature(ctx, strconv.FormatUint(number, 10))
			require.NoError(t, err)
			stateUpdate, block, err := adapter.StateUpdateWithBlock(ctx, number)
			require.NoError(t, err)
			adaptedBlock, err := sn2core.AdaptBlock(response.Block, sig)
			require.NoError(t, err)
			adaptedStateUpdate, err := sn2core.AdaptStateUpdate(response.StateUpdate)
			require.NoError(t, err)
			assert.Equal(t, block, adaptedBlock)
			assert.Equal(t, stateUpdate, adaptedStateUpdate)
		})
	}
}

func TestStateUpdatePendingWithBlock(t *testing.T) {
	client := feeder.NewTestClient(t, utils.Integration)
	adapter := adaptfeeder.New(client)
	ctx := context.Background()

	response, err := client.StateUpdateWithBlock(ctx, "pending")
	require.NoError(t, err)
	adaptedBlock, err := sn2core.AdaptBlock(response.Block, nil)
	require.NoError(t, err)
	adaptedStateUpdate, err := sn2core.AdaptStateUpdate(response.StateUpdate)
	require.NoError(t, err)
	stateUpdate, block, err := adapter.StateUpdatePendingWithBlock(ctx)
	require.NoError(t, err)
	assert.Equal(t, block, adaptedBlock)
	assert.Equal(t, stateUpdate, adaptedStateUpdate)
}
