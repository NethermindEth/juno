package core_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/types/felt"
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
			class, err := gw.Class(t.Context(), hash)
			require.NoError(t, err)

			got, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, hash, got)
		})
	}
}

func TestClassV1Hash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash       string
		checkNoCompiled bool
	}{
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5
			classHash: "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
		},
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c
			classHash: "0x4e70b19333ae94bd958625f7b61ce9eec631653597e68645e13780061b2136c",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(t.Context(), hash)
			require.NoError(t, err)

			got, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, hash, got)
		})
	}
}

func TestCompiledClassHash(t *testing.T) {
	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash                 string
		expectedCompiledClassHash string
	}{
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc
			classHash:                 "0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc",
			expectedCompiledClassHash: "0x18f95714044fd5408d3bf812bcd249ddec098ab3cd201b7916170cfbfa59e05",
		},
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x6b3da05b352f93912df0593a703f1884c4c607523bb33feaff4940635ef050d
			classHash:                 "0x6b3da05b352f93912df0593a703f1884c4c607523bb33feaff4940635ef050d",
			expectedCompiledClassHash: "0x603dd72504d8b0bc54df4f1102fdcf87fc3b2b94750a9083a5876913eec08e4",
		},
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x1fb5f6adb94dd3c0bfda71f7f73957691619ab9fe8f6b9b675da13877086f89
			classHash:                 "0x1fb5f6adb94dd3c0bfda71f7f73957691619ab9fe8f6b9b675da13877086f89",
			expectedCompiledClassHash: "0x260f0d9862f0dd76ac1f9c93e6ce0c2536f7c0275c87061e73abce321bfd4ad",
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			hash := utils.HexToFelt(t, tt.classHash)
			class, err := gw.Class(t.Context(), hash)
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

	client := feeder.NewTestClient(t, &utils.Integration)
	gw := adaptfeeder.New(client)

	cairo1ClassHash := utils.HexToFelt(t, "0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5")
	cairo1Class, err := gw.Class(t.Context(), cairo1ClassHash)
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

	cairo0ClassHash := utils.HexToFelt(t, "0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04")
	cairo0Class, err := gw.Class(t.Context(), cairo0ClassHash)
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

func TestSierraVersion(t *testing.T) {
	t.Run("cairo zero should return 0.0.0 by default", func(t *testing.T) {
		class := core.Cairo0Class{}
		sierraVersion := class.SierraVersion()
		require.Equal(t, "0.0.0", sierraVersion)
	})

	t.Run("cairo one should return 0.1.0 when only one felt", func(t *testing.T) {
		sierraVersion010 := felt.Felt(
			[4]uint64{
				18446737451840584193,
				18446744073709551615,
				18446744073709551615,
				576348180530977296,
			})
		class := core.Cairo1Class{
			Program: []*felt.Felt{
				&sierraVersion010,
			},
		}
		sierraVersion := class.SierraVersion()
		require.Equal(t, "0.1.0", sierraVersion)
	})

	t.Run("cairo one should return based on the program data", func(t *testing.T) {
		class := core.Cairo1Class{
			Program: []*felt.Felt{
				new(felt.Felt).SetUint64(7),
				new(felt.Felt).SetUint64(3),
				new(felt.Felt).SetUint64(11),
			},
		}
		sierraVersion := class.SierraVersion()
		require.Equal(t, "7.3.11", sierraVersion)
	})
}
