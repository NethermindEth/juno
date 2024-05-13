package core_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	client := feeder.NewTestClient(t, &utils.Sepolia)

	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash string
	}{
		{
			// https://alpha-sepolia.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x07db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909
			classHash: "0x07db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909",
		},
		{
			// https://alpha-sepolia.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x0772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3
			classHash: "0x0772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
		},
		{
			// https://alpha-sepolia.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x028d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43
			classHash: "0x028d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			require.NoError(t, err)

			got, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, hash, got)
		})
	}
}

func TestClassV1Hash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash       string
		checkNoCompiled bool
	}{
		{
			// https://alpha-mainnet.starknet.io/feeder_gateway/get_class_by_hash?classHash=<calss_hash>
			classHash: "0x1efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f",
		},
		{
			classHash: "0x1338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292",
		},
		{
			classHash: "0x3297a93c52357144b7da71296d7e8231c3e0959f0a1d37222204f2f7712010e",
		},
		{
			classHash: "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			require.NoError(t, err)

			got, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, hash, got)
		})
	}
}

func TestCompiledClassHash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash                 string
		expectedCompiledClassHash string
	}{
		{
			classHash:                 "0x1338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292",
			expectedCompiledClassHash: "0xf2056a217cc9cabef54d4b1bceea5a3e8625457cb393698ba507259ed6f3c",
		},
		{
			classHash:                 "0x21c2e8a87c431e8d3e89ecd1a40a0674ef533cce5a1f6c44ba9e60d804ecad2",
			expectedCompiledClassHash: "0x1199c4832cfea48f452b8ddebaf7e4ceb77bd0e27704efd06753b4878631e39",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash "+tt.classHash[:7], func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(context.Background(), hash)
			require.NoError(t, err)
			got := class.(*core.Cairo1Class).Compiled.Hash()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCompiledClassHash, got.String())
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
				Abi: json.RawMessage("abi"),
				Externals: []core.EntryPoint{
					{Selector: utils.HexToFelt(t, "0x44"), Offset: utils.HexToFelt(t, "0x37")},
				},
				L1Handlers:   []core.EntryPoint{},
				Constructors: []core.EntryPoint{},
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
		assert.Fail(t, "not a class")
	}
}

func TestVerifyClassHash(t *testing.T) {
	type Tests struct {
		name      string
		classHash *felt.Felt
		class     core.Class
		wantErr   error
	}

	client := feeder.NewTestClient(t, &utils.Mainnet)
	gw := adaptfeeder.New(client)

	cairo1ClassHash := utils.HexToFelt(t, "0x1338d85d3e579f6944ba06c005238d145920afeb32f94e3a1e234d21e1e9292")
	cairo1Class, err := gw.Class(context.Background(), cairo1ClassHash)
	require.NoError(t, err)

	t.Run("class(es) with error", func(t *testing.T) {
		tests := []Tests{
			{
				name:      "error if expected hash is not equal to gotten hash",
				classHash: utils.HexToFelt(t, "0xab"),
				class:     cairo1Class,
				wantErr: fmt.Errorf("cannot verify class hash: calculated hash %v, received hash %v", cairo1ClassHash.String(),
					utils.HexToFelt(t, "0xab").String()),
			},
			{
				name:      "no error if expected hash is equal to gotten hash",
				classHash: cairo1ClassHash,
				class:     cairo1Class,
				wantErr:   nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err = core.VerifyClassHashes(map[felt.Felt]core.Class{
					*tt.classHash: tt.class,
				})
				require.Equal(t, tt.wantErr, err)
			})
		}
	})

	cairo0ClassHash := utils.HexToFelt(t, "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	cairo0Class, err := gw.Class(context.Background(), cairo0ClassHash)
	require.NoError(t, err)

	t.Run("class(es) with no error", func(t *testing.T) {
		classMap := map[felt.Felt]core.Class{
			*cairo1ClassHash: cairo1Class,
			*cairo0ClassHash: cairo0Class,
		}

		assert.NoError(t, core.VerifyClassHashes(classMap))
	})
}

func TestSegmentedBytecodeHash(t *testing.T) {
	// nested case that is not covered by class hash tests
	require.Equal(t, "0x7cdd91b70b76e3deb1d334d76ba08eebd26f8c06af82117b79bcf1386c8e736",
		core.SegmentedBytecodeHash([]*felt.Felt{
			new(felt.Felt).SetUint64(1),
			new(felt.Felt).SetUint64(2),
			new(felt.Felt).SetUint64(3),
		}, []core.SegmentLengths{
			{
				Length: 1,
			},
			{
				Children: []core.SegmentLengths{
					{
						Length: 1,
					},
					{
						Length: 1,
					},
				},
			},
		}).String())
}
