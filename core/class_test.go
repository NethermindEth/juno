package core_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassV0Hash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.GOERLI)
	t.Cleanup(closeFn)

	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash string
	}{
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8
			classHash: "0x010455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3
			classHash: "0x056b96c1d1bbfa01af44b465763d1b71150fa00c6c9d54c3947f57e979ff68c3",
		},
		{
			// https://alpha4.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118
			classHash: "0x0079e2d211e70594e687f9f788f71302e6eecb61d98efce48fbe8514948c8118",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			assert.NoError(t, err)
			got := class.Hash()
			assert.Equal(t, hash, got)
		})
	}
}

func TestClassV1Hash(t *testing.T) {
	client, closeFn := feeder.NewTestClient(utils.INTEGRATION)
	t.Cleanup(closeFn)

	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash string
	}{
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c
			classHash: "0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			require.NoError(t, err)
			got := class.Hash()
			assert.Equal(t, hash, got)
		})
	}
}

func TestClassEncoding(t *testing.T) {
	tests := []struct {
		name  string
		class core.Class
	}{
		{
			name: "V0",
			class: &core.Cairo0Class{
				Abi: "abi",
				Externals: []core.EntryPoint{
					{Selector: utils.HexToFelt(t, "0x44"), Offset: utils.HexToFelt(t, "0x37")},
				},
				L1Handlers:   []core.EntryPoint{},
				Constructors: []core.EntryPoint{},
				Builtins:     []*felt.Felt{utils.HexToFelt(t, "0xDEADBEEF")},
				ProgramHash:  utils.HexToFelt(t, "0xBEEFDEAD"),
				Bytecode:     []*felt.Felt{utils.HexToFelt(t, "0xDEAD"), utils.HexToFelt(t, "0xBEEF")},
			},
		},
		{
			name: "V1",
			class: &core.Cairo1Class{
				Abi:     "abi",
				AbiHash: utils.HexToFelt(t, "0xDEADBEEF"),
				EntryPoints: struct {
					Constructor []core.SierraEntryPoint
					External    []core.SierraEntryPoint
					L1Handler   []core.SierraEntryPoint
				}{
					Constructor: []core.SierraEntryPoint{},
					External: []core.SierraEntryPoint{
						{
							Index:    1,
							Selector: utils.HexToFelt(t, "0xDEADBEEF"),
						},
					},
					L1Handler: []core.SierraEntryPoint{},
				},
				Program:         []*felt.Felt{utils.HexToFelt(t, "0xDEAD"), utils.HexToFelt(t, "0xBEEF")},
				ProgramHash:     utils.HexToFelt(t, "0xBEEFDEAD"),
				SemanticVersion: "0.1.0",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkClassSymmetry(t, test.class)
		})
	}
}

func checkClassSymmetry(t *testing.T, input core.Class) {
	t.Helper()
	require.NoError(t, encoder.RegisterType(reflect.TypeOf(input)))

	data, err := encoder.Marshal(input)
	require.NoError(t, err)

	var class core.Class
	require.NoError(t, encoder.Unmarshal(data, &class))

	switch v := class.(type) {
	case *core.Cairo0Class:
		assert.Equal(t, input, v)
	case *core.Cairo1Class:
		assert.Equal(t, input, v)
	default:
		t.Error("not a class")
	}
}
