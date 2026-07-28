package feeder_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/adapters/sn2core"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknet"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockByNumber(t *testing.T) {
	numbers := []uint64{147, 11817}

	client := feeder.NewTestClient(t, &networks.Mainnet)
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
			adaptedResponse, err := sn2core.AdaptBlock(&response, sig.Signature)
			require.NoError(t, err)
			assert.Equal(t, adaptedResponse, block)
		})
	}
}

func TestBlockLatest(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	response, err := client.Block(ctx, "latest")
	require.NoError(t, err)
	sig, err := client.Signature(ctx, "latest")
	require.NoError(t, err)
	block, err := adapter.BlockLatest(ctx)
	require.NoError(t, err)
	adaptedResponse, err := sn2core.AdaptBlock(&response, sig.Signature)
	require.NoError(t, err)
	assert.Equal(t, adaptedResponse, block)
}

func TestBlockHeaderLatest(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	block, err := client.BlockHeader(ctx, "latest")
	require.NoError(t, err)

	header, err := adapter.BlockHeaderLatest(ctx)
	require.NoError(t, err)

	assert.Equal(t, block.Hash, header.Hash)
	assert.Equal(t, block.Number, header.Number)
}

func TestStateUpdate(t *testing.T) {
	numbers := []uint64{0, 1, 2, 21656}

	client := feeder.NewTestClient(t, &networks.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	for _, number := range numbers {
		numberStr := strconv.FormatUint(number, 10)
		t.Run("number "+numberStr, func(t *testing.T) {
			response, err := client.StateUpdate(ctx, numberStr)
			require.NoError(t, err)
			feederUpdate, err := adapter.StateUpdate(ctx, number)
			require.NoError(t, err)

			adaptedResponse, err := sn2core.AdaptStateUpdate(&response)
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

	client := feeder.NewTestClient(t, &networks.Sepolia)
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

func TestClassV1(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Integration)
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
		var compiledClass *starknet.CasmClass
		if test.hasCompiledClass {
			require.NoError(t, err)
			compiledClass = &casmClass
		} else {
			require.EqualError(t, err, "deprecated compiled class")
		}

		adaptedResponse, err := sn2core.AdaptSierraClass(feederClass.Sierra, compiledClass)
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

	client := feeder.NewTestClient(t, &networks.SepoliaIntegration)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()

	for _, number := range numbers {
		numberStr := strconv.FormatUint(number, 10)
		t.Run("integration block number "+numberStr, func(t *testing.T) {
			response, err := client.StateUpdateWithBlockAndSignature(ctx, numberStr)
			require.NoError(t, err)
			sig, err := client.Signature(ctx, numberStr)
			require.NoError(t, err)
			stateUpdate, block, err := adapter.StateUpdateWithBlock(ctx, number)
			require.NoError(t, err)
			adaptedBlock, err := sn2core.AdaptBlock(response.Block, sig.Signature)
			require.NoError(t, err)
			adaptedStateUpdate, err := sn2core.AdaptStateUpdate(response.StateUpdate)
			require.NoError(t, err)
			assert.Equal(t, block, adaptedBlock)
			assert.Equal(t, stateUpdate, adaptedStateUpdate)
		})
	}
}

func TestAdapterErrorPaths(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Mainnet)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()
	missing := uint64(99999999)
	missingHash := felt.NewUnsafeFromString[felt.Felt]("0xdeadbeef")

	t.Run("BlockByNumber error", func(t *testing.T) {
		block, err := adapter.BlockByNumber(ctx, missing)
		assert.Error(t, err)
		assert.Nil(t, block)
	})

	t.Run("StateUpdate error", func(t *testing.T) {
		su, err := adapter.StateUpdate(ctx, missing)
		assert.Error(t, err)
		assert.Nil(t, su)
	})

	t.Run("StateUpdateWithBlock error", func(t *testing.T) {
		su, blk, err := adapter.StateUpdateWithBlock(ctx, missing)
		assert.Error(t, err)
		assert.Nil(t, su)
		assert.Nil(t, blk)
	})

	t.Run("Class error", func(t *testing.T) {
		cls, err := adapter.Class(ctx, missingHash)
		assert.Error(t, err)
		assert.Nil(t, cls)
	})

	t.Run("PreConfirmedBlockByNumber error", func(t *testing.T) {
		preConfirmed, err := adapter.PreConfirmedBlockByNumber(ctx, missing, "", 0)
		assert.Zero(t, preConfirmed)
		assert.Error(t, err)
	})

	t.Run("BlockHeaderLatest error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(srv.Close)
		feederURL, err := url.Parse(srv.URL)
		require.NoError(t, err)
		errClient := feeder.NewClient(
			feederURL,
			feeder.WithBackoff(feeder.NopBackoff),
			feeder.WithMaxRetries(0),
		)
		errAdapter := adaptfeeder.New(errClient)
		hdr, err := errAdapter.BlockHeaderLatest(ctx)
		assert.Error(t, err)
		assert.Zero(t, hdr)
	})
}

func TestPreConfirmedBlock(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.SepoliaIntegration)
	adapter := adaptfeeder.New(client)
	ctx := t.Context()
	blockNumber := uint64(11252240)

	update, err := adapter.PreConfirmedBlockByNumber(ctx, blockNumber, "", 0)
	require.NoError(t, err)
	full, ok := update.(starknet.PreConfirmedBlock)
	require.True(t, ok, "expected PreConfirmedBlock, got %T", update)
	assert.NotEmpty(t, full.BlockIdentifier)
}
