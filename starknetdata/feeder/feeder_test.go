package feeder_test

import (
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockByNumber(t *testing.T) {
	numbers := []uint64{147, 11817}

	client := feeder.NewTestClient(t, &utils.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	for _, number := range numbers {
		numberStr := strconv.FormatUint(number, 10)
		t.Run("mainnet block number "+numberStr, func(t *testing.T) {
			response, err := client.Block(ctx, numberStr)
			require.NoError(t, err)
			sig, err := client.Signature(ctx, numberStr)
			require.NoError(t, err)
			block, err := adapter.BlockByNumber(ctx, number)
			require.NoError(t, err)
			adaptedResponse, err := sn2core.AdaptBlock(response, sig)
			require.NoError(t, err)
			assert.Equal(t, &adaptedResponse, block)
		})
	}
}

func TestBlockLatest(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	response, err := client.Block(ctx, "latest")
	require.NoError(t, err)
	sig, err := client.Signature(ctx, "latest")
	require.NoError(t, err)
	block, err := adapter.BlockLatest(ctx)
	require.NoError(t, err)
	adaptedResponse, err := sn2core.AdaptBlock(response, sig)
	require.NoError(t, err)
	assert.Equal(t, &adaptedResponse, block)
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client := feeder.NewTestClient(t, &utils.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	for _, number := range numbers {
		numberStr := strconv.FormatUint(number, 10)
		t.Run("number "+numberStr, func(t *testing.T) {
			response, err := client.StateUpdate(ctx, numberStr)
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
		"0x7db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909",
		"0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
		"0x78401746828463e2c3f92ebb261fc82f7d4d4c8d9a80a356c44580dab124cb0",
		"0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43",
	}

	client := feeder.NewTestClient(t, &utils.Sepolia)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	for _, hashString := range classHashes {
		t.Run("hash "+hashString, func(t *testing.T) {
			hash := felt.NewUnsafeFromString[felt.Felt](hashString)
			response, err := client.ClassDefinition(ctx, hash)
			require.NoError(t, err)
			classGeneric, err := adapter.Class(ctx, hash)
			require.NoError(t, err)

			adaptedResponse, err := sn2core.AdaptDeprecatedCairoClass(response.DeprecatedCairo)
			require.NoError(t, err)
			require.Equal(t, adaptedResponse, classGeneric)
		})
	}
}

func TestTransaction(t *testing.T) {
	clientGoerli := feeder.NewTestClient(t, &utils.Goerli)
	adapterGoerli := adaptfeeder.New(clientGoerli)

	clientMainnet := feeder.NewTestClient(t, &utils.Mainnet)
	adapterMainnet := adaptfeeder.New(clientMainnet)

	ctx := t.Context()

	t.Run("invoke transaction", func(t *testing.T) {
		hash := felt.NewUnsafeFromString[felt.Felt]("0x7e3a229febf47c6edfd96582d9476dd91a58a5ba3df4553ae448a14a2f132d9")
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0x15b51c2f4880b1e7492d30ada7254fc59c09adde636f37eb08cdadbd9dabebb")
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0xd61fc89f4d1dc4dc90a014957d655d38abffd47ecea8e3fa762e3160f155f2")
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0x6eab8252abfc9bbfd72c8d592dde4018d07ce467c5ce922519d7142fcab203f")
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
		hash := felt.NewUnsafeFromString[felt.Felt]("0x537eacfd3c49166eec905daff61ff7feef9c133a049ea2135cb94eec840a4a8")
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
	client := feeder.NewTestClient(t, &utils.Integration)
	adapter := adaptfeeder.New(client)

	tests := []struct {
		classHash        *felt.Felt
		hasCompiledClass bool
	}{
		{
			classHash:        felt.NewUnsafeFromString[felt.Felt]("0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5"),
			hasCompiledClass: true,
		},
		{
			classHash:        felt.NewUnsafeFromString[felt.Felt]("0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c"),
			hasCompiledClass: false,
		},
	}

	for _, test := range tests {
		class, err := adapter.Class(t.Context(), test.classHash)
		require.NoError(t, err)

		feederClass, err := client.ClassDefinition(t.Context(), test.classHash)
		require.NoError(t, err)
		casmClass, err := client.CasmClassDefinition(t.Context(), test.classHash)
		if test.hasCompiledClass {
			require.NoError(t, err)
		} else {
			require.EqualError(t, err, "deprecated compiled class")
		}

		adaptedResponse, err := sn2core.AdaptSierraClass(feederClass.Sierra, casmClass)
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

	client := feeder.NewTestClient(t, &utils.Integration)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	for _, number := range numbers {
		numberStr := strconv.FormatUint(number, 10)
		t.Run("integration block number "+numberStr, func(t *testing.T) {
			response, err := client.StateUpdateWithBlock(ctx, numberStr)
			require.NoError(t, err)
			sig, err := client.Signature(ctx, numberStr)
			require.NoError(t, err)
			stateUpdate, block, err := adapter.StateUpdateWithBlock(ctx, number)
			require.NoError(t, err)
			adaptedBlock, err := sn2core.AdaptBlock(response.Block, sig)
			require.NoError(t, err)
			adaptedStateUpdate, err := sn2core.AdaptStateUpdate(response.StateUpdate)
			require.NoError(t, err)
			assert.Equal(t, block, &adaptedBlock)
			assert.Equal(t, stateUpdate, adaptedStateUpdate)
		})
	}
}

func TestStateUpdatePendingWithBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	response, err := client.StateUpdateWithBlock(ctx, "pending")
	require.NoError(t, err)
	adaptedBlock, err := sn2core.AdaptBlock(response.Block, nil)
	require.NoError(t, err)
	adaptedStateUpdate, err := sn2core.AdaptStateUpdate(response.StateUpdate)
	require.NoError(t, err)
	stateUpdate, block, err := adapter.StateUpdatePendingWithBlock(ctx)
	require.NoError(t, err)
	assert.Equal(t, block, &adaptedBlock)
	assert.Equal(t, stateUpdate, adaptedStateUpdate)
}

func TestPreConfirmedBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.SepoliaIntegration)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()
	blockNumber := uint64(1204672)
	blockNumberStr := strconv.FormatUint(blockNumber, 10)
	response, err := client.PreConfirmedBlock(ctx, blockNumberStr)
	require.NoError(t, err)
	adaptedPreConfirmed, err := sn2core.AdaptPreConfirmedBlock(response, blockNumber)
	require.NoError(t, err)

	preConfirmed, err := adapter.PreConfirmedBlockByNumber(ctx, blockNumber)
	require.NoError(t, err)
	assert.Equal(t, preConfirmed, adaptedPreConfirmed)
}
