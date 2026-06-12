package core_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/clients/feeder"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/encoder"
	adaptfeeder "github.com/NethermindEth/juno/starknetdata/feeder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassV0Hash(t *testing.T) {
	tests := []struct {
		name    string
		network *networks.Network
	}{
		{name: "Mainnet", network: &networks.Mainnet},
		{name: "Sepolia", network: &networks.Sepolia},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := feeder.NewTestClient(t, tt.network)
			gw := adaptfeeder.New(client)

			for _, fixture := range loadDeprecatedCairoHashFixtures(t, tt.network.Name) {
				t.Run(fixture.fixtureKey, func(t *testing.T) {
					hash := felt.UnsafeFromString[felt.Felt](fixture.fixtureKey)
					class, err := gw.Class(t.Context(), &hash)
					require.NoError(t, err)

					_, ok := class.(*core.DeprecatedCairoClass)
					require.True(
						t,
						ok,
						"fixture %s on %s should be a DeprecatedCairoClass",
						fixture.fixtureKey,
						tt.network.Name,
					)

					got, err := class.Hash()
					require.NoError(t, err)
					assert.Equal(t, felt.UnsafeFromString[felt.Felt](fixture.expectedClassHash), got)
				})
			}
		})
	}
}

type deprecatedCairoHashFixture struct {
	fixtureKey        string
	expectedClassHash string
}

// deprecatedCairoHashFixturesByNetwork lists the frozen V0 class hashes
// captured from the pre-migration Rust/FFI implementation or a trusted gateway
// response. Some checked-in fixture filenames no longer match the hash their
// current bytes produce, so fixtureKey and expectedClassHash can differ.
var deprecatedCairoHashFixturesByNetwork = map[string][]deprecatedCairoHashFixture{
	"mainnet": {
		{
			fixtureKey:        "0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8",
			expectedClassHash: "0x2220884df4e83cf7ba6e968416ccd44f31c9846f47a021e2a52b6da29a1781e",
		},
		{
			fixtureKey:        "0x1efa8f84fd4dff9e2902ec88717cf0dafc8c188f80c3450615944a469428f7f",
			expectedClassHash: "0x694716ddb975811b5a016e849475851996eb0d581d7cdc9332950f699f7e022",
		},
		{
			fixtureKey:        "0x3297a93c52357144b7da71296d7e8231c3e0959f0a1d37222204f2f7712010e",
			expectedClassHash: "0x4ee0b2964f380cb35c72346531ff1d518bb4ba0fff17f6d6bb669730860b88d",
		},
	},
	"sepolia": {
		{
			fixtureKey:        "0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43",
			expectedClassHash: "0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43",
		},
		{
			fixtureKey:        "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
			expectedClassHash: "0x5c478ee27f2112411f86f207605b2e2c58cdb647bac0df27f660ef2252359c6",
		},
		{
			fixtureKey:        "0x5f18f9cdc05da87f04e8e7685bd346fc029f977167d5b1b2b59f69a7dacbfc8",
			expectedClassHash: "0x22a4f508608ca25a4f8de87b6c12fc9279060f8f13246c09d631e207ed1b9c",
		},
		{
			fixtureKey:        "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
			expectedClassHash: "0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
		},
		{
			fixtureKey:        "0x78401746828463e2c3f92ebb261fc82f7d4d4c8d9a80a356c44580dab124cb0",
			expectedClassHash: "0x78401746828463e2c3f92ebb261fc82f7d4d4c8d9a80a356c44580dab124cb0",
		},
		{
			fixtureKey:        "0x7db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909",
			expectedClassHash: "0x7db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909",
		},
		{
			fixtureKey:        "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3",
			expectedClassHash: "0xd0e183745e9dae3e4e78a8ffedcce0903fc4900beace4e0abf192d4c202da3",
		},
	},
}

func loadDeprecatedCairoHashFixtures(t *testing.T, network string) []deprecatedCairoHashFixture {
	t.Helper()

	pattern := filepath.Join("..", "clients", "feeder", "testdata", network, "class", "*.json")
	paths, err := filepath.Glob(pattern)
	require.NoError(t, err)

	expectedFixtures, ok := deprecatedCairoHashFixturesByNetwork[network]
	require.True(t, ok, "missing expected hashes for network %s", network)

	expectedClassHashes := make(map[string]string, len(expectedFixtures))
	for _, fixture := range expectedFixtures {
		expectedClassHashes[fixture.fixtureKey] = fixture.expectedClassHash
	}

	fixtures := make([]deprecatedCairoHashFixture, 0, len(paths))
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		require.NoError(t, err)

		var probe struct {
			ContractClassVersion *string `json:"contract_class_version"`
		}
		require.NoError(t, json.Unmarshal(contents, &probe))
		if probe.ContractClassVersion != nil {
			continue
		}

		fixtureKey := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		expectedClassHash, ok := expectedClassHashes[fixtureKey]
		require.True(t, ok, "missing expected hash for deprecated fixture %s on %s", fixtureKey, network)
		fixtures = append(fixtures, deprecatedCairoHashFixture{
			fixtureKey:        fixtureKey,
			expectedClassHash: expectedClassHash,
		})
	}

	require.Len(t, fixtures, len(expectedFixtures))

	return fixtures
}

func TestClassV1Hash(t *testing.T) {
	client := feeder.NewTestClient(t, &networks.Integration)
	gw := adaptfeeder.New(client)
	tests := []struct {
		classHash string
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
			hash := felt.UnsafeFromString[felt.Felt](tt.classHash)
			class, err := gw.Class(t.Context(), &hash)
			require.NoError(t, err)

			got, err := class.Hash()
			require.NoError(t, err)
			assert.Equal(t, hash, got)
		})
	}
}

func TestCompiledClassHash(t *testing.T) {
	tests := []struct {
		network                   networks.Network
		classHash                 string
		expectedCompiledClassHash string
		hashVersion               core.CasmHashVersion
	}{
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc
			classHash:                 "0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc",
			expectedCompiledClassHash: "0x18f95714044fd5408d3bf812bcd249ddec098ab3cd201b7916170cfbfa59e05",
			hashVersion:               core.HashVersionV1,
			network:                   networks.Integration,
		},
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x6b3da05b352f93912df0593a703f1884c4c607523bb33feaff4940635ef050d
			classHash:                 "0x6b3da05b352f93912df0593a703f1884c4c607523bb33feaff4940635ef050d",
			expectedCompiledClassHash: "0x603dd72504d8b0bc54df4f1102fdcf87fc3b2b94750a9083a5876913eec08e4",
			hashVersion:               core.HashVersionV1,
			network:                   networks.Integration,
		},
		{
			// https://external.integration.starknet.io/feeder_gateway/get_class_by_hash?classHash=0x1fb5f6adb94dd3c0bfda71f7f73957691619ab9fe8f6b9b675da13877086f89
			classHash:                 "0x1fb5f6adb94dd3c0bfda71f7f73957691619ab9fe8f6b9b675da13877086f89",
			expectedCompiledClassHash: "0x260f0d9862f0dd76ac1f9c93e6ce0c2536f7c0275c87061e73abce321bfd4ad",
			hashVersion:               core.HashVersionV1,
			network:                   networks.Integration,
		},
		{
			classHash:                 "0x941a2dc3ab607819fdc929bea95831a2e0c1aab2f2f34b3a23c55cebc8a040",
			expectedCompiledClassHash: "0x6c1f99f23865abe822bd9690f8d6cd181d43b1ff5535842aa973363aa7c7bb3",
			hashVersion:               core.HashVersionV2,
			network:                   networks.SepoliaIntegration,
		},
	}

	for _, tt := range tests {
		t.Run("ClassHash", func(t *testing.T) {
			client := feeder.NewTestClient(t, &tt.network)
			gw := adaptfeeder.New(client)
			hash := felt.NewUnsafeFromString[felt.Felt](tt.classHash)
			class, err := gw.Class(t.Context(), hash)
			require.NoError(t, err)
			got := class.(*core.SierraClass).Compiled.Hash(tt.hashVersion)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCompiledClassHash, got.String())
		})
	}
}

func TestClassEncoding(t *testing.T) {
	tests := []struct {
		name  string
		class core.ClassDefinition
	}{
		{
			name: "V0",
			class: &core.DeprecatedCairoClass{
				Abi: json.RawMessage("abi"),
				Externals: []core.DeprecatedEntryPoint{
					{Selector: felt.NewUnsafeFromString[felt.Felt]("0x44"), Offset: felt.NewUnsafeFromString[felt.Felt]("0x37")},
				},
				L1Handlers:   []core.DeprecatedEntryPoint{},
				Constructors: []core.DeprecatedEntryPoint{},
			},
		},
		{
			name: "V1",
			class: &core.SierraClass{
				Abi:     "abi",
				AbiHash: felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF"),
				EntryPoints: struct {
					Constructor []core.SierraEntryPoint
					External    []core.SierraEntryPoint
					L1Handler   []core.SierraEntryPoint
				}{
					Constructor: []core.SierraEntryPoint{},
					External: []core.SierraEntryPoint{
						{
							Index:    1,
							Selector: felt.NewUnsafeFromString[felt.Felt]("0xDEADBEEF"),
						},
					},
					L1Handler: []core.SierraEntryPoint{},
				},
				Program:         []*felt.Felt{felt.NewUnsafeFromString[felt.Felt]("0xDEAD"), felt.NewUnsafeFromString[felt.Felt]("0xBEEF")},
				ProgramHash:     felt.NewUnsafeFromString[felt.Felt]("0xBEEFDEAD"),
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

func checkClassSymmetry(t *testing.T, input core.ClassDefinition) {
	t.Helper()

	data, err := encoder.Marshal(input)
	require.NoError(t, err)

	var class core.ClassDefinition
	require.NoError(t, encoder.Unmarshal(data, &class))

	switch v := class.(type) {
	case *core.DeprecatedCairoClass:
		assert.Equal(t, input, v)
	case *core.SierraClass:
		assert.Equal(t, input, v)
	default:
		assert.Fail(t, "not a class")
	}
}

func TestVerifyClassHash(t *testing.T) {
	type Tests struct {
		name      string
		classHash *felt.Felt
		class     core.ClassDefinition
		wantErr   error
	}

	client := feeder.NewTestClient(t, &networks.Integration)
	gw := adaptfeeder.New(client)

	sierraClassHash := felt.NewUnsafeFromString[felt.Felt](
		"0x1cd2edfb485241c4403254d550de0a097fa76743cd30696f714a491a454bad5",
	)
	sierraClass, err := gw.Class(t.Context(), sierraClassHash)
	require.NoError(t, err)

	t.Run("class(es) with error", func(t *testing.T) {
		tests := []Tests{
			{
				name:      "error if expected hash is not equal to gotten hash",
				classHash: felt.NewUnsafeFromString[felt.Felt]("0xab"),
				class:     sierraClass,
				wantErr: fmt.Errorf(
					"cannot verify class hash: calculated hash %v, received hash %v",
					sierraClassHash.String(),
					felt.NewUnsafeFromString[felt.Felt]("0xab").String(),
				),
			},
			{
				name:      "no error if expected hash is equal to gotten hash",
				classHash: sierraClassHash,
				class:     sierraClass,
				wantErr:   nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err = core.VerifyClassHashes(map[felt.Felt]core.ClassDefinition{
					*tt.classHash: tt.class,
				})
				require.Equal(t, tt.wantErr, err)
			})
		}
	})

	deprecatedCairoClassHash := felt.NewUnsafeFromString[felt.Felt](
		"0x4631b6b3fa31e140524b7d21ba784cea223e618bffe60b5bbdca44a8b45be04",
	)
	deprecatedCairoClass, err := gw.Class(t.Context(), deprecatedCairoClassHash)
	require.NoError(t, err)

	t.Run("class(es) with no error", func(t *testing.T) {
		classMap := map[felt.Felt]core.ClassDefinition{
			*sierraClassHash:          sierraClass,
			*deprecatedCairoClassHash: deprecatedCairoClass,
		}

		assert.NoError(t, core.VerifyClassHashes(classMap))
	})
}

func TestSegmentedBytecodeHash(t *testing.T) {
	byteCode := []*felt.Felt{
		felt.NewFromUint64[felt.Felt](1),
		felt.NewFromUint64[felt.Felt](2),
		felt.NewFromUint64[felt.Felt](3),
	}
	segmentLengths := []core.SegmentLengths{
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
	}

	t.Run("hash version v1", func(t *testing.T) {
		hasher := core.NewCasmHasher(core.HashVersionV1)
		// nested case that is not covered by class hash tests
		segmentedByteCodeHash := core.SegmentedBytecodeHash(
			byteCode,
			segmentLengths,
			hasher,
		)
		require.Equal(
			t,
			"0x7cdd91b70b76e3deb1d334d76ba08eebd26f8c06af82117b79bcf1386c8e736",
			segmentedByteCodeHash.String(),
		)
	})

	t.Run("hash version v2", func(t *testing.T) {
		hasher := core.NewCasmHasher(core.HashVersionV2)
		// nested case that is not covered by class hash tests
		segmentedByteCodeHash := core.SegmentedBytecodeHash(
			byteCode,
			segmentLengths,
			hasher,
		)
		require.Equal(
			t,
			"0x6cbb48b353d958576794ae55e64046f150ebac350207c97bdfd3cd5cdfbe406",
			segmentedByteCodeHash.String(),
		)
	})
}

func TestSierraVersion(t *testing.T) {
	t.Run("cairo zero should return 0.0.0 by default", func(t *testing.T) {
		class := core.DeprecatedCairoClass{}
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
		class := core.SierraClass{
			Program: []*felt.Felt{
				&sierraVersion010,
			},
		}
		sierraVersion := class.SierraVersion()
		require.Equal(t, "0.1.0", sierraVersion)
	})

	t.Run("cairo one should return based on the program data", func(t *testing.T) {
		class := core.SierraClass{
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

func TestClassCasmHashMetadata(t *testing.T) {
	declaredAt := uint64(100)
	migratedAt := uint64(150)
	v1Hash := felt.UnsafeFromString[felt.CasmClassHash]("0x1")
	v2Hash := felt.UnsafeFromString[felt.CasmClassHash]("0x2")
	record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
	require.NoError(t, record.Migrate(migratedAt))

	t.Run("NewCasmHashMetadataDeclaredV1", func(t *testing.T) {
		t.Run("before declaration", func(t *testing.T) {
			_, err := record.CasmHashAt(declaredAt - 1)
			require.Error(t, err)
			require.ErrorIs(t, err, db.ErrKeyNotFound)
		})

		t.Run("at declaration", func(t *testing.T) {
			hash, err := record.CasmHashAt(declaredAt)
			require.NoError(t, err)
			require.Equal(t, v1Hash, hash)
		})

		t.Run("after declaration", func(t *testing.T) {
			hash, err := record.CasmHashAt(declaredAt + 1)
			require.NoError(t, err)
			require.Equal(t, v1Hash, hash)
		})
	})

	t.Run("NewCasmHashMetadataDeclaredV2", func(t *testing.T) {
		record := core.NewCasmHashMetadataDeclaredV2(declaredAt, &v2Hash)

		t.Run("CasmHash", func(t *testing.T) {
			assert.Equal(t, v2Hash, record.CasmHash())
		})

		t.Run("before declaration", func(t *testing.T) {
			_, err := record.CasmHashAt(declaredAt - 1)
			require.Error(t, err)
			require.ErrorIs(t, err, db.ErrKeyNotFound)
		})

		t.Run("at declaration", func(t *testing.T) {
			hash, err := record.CasmHashAt(declaredAt)
			require.NoError(t, err)
			require.Equal(t, v2Hash, hash)
		})

		t.Run("after declaration", func(t *testing.T) {
			hash, err := record.CasmHashAt(declaredAt + 1)
			require.NoError(t, err)
			require.Equal(t, v2Hash, hash)
		})
	})

	t.Run("Migrate V1 class", func(t *testing.T) {
		record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
		require.NoError(t, record.Migrate(150))

		t.Run("CasmHash", func(t *testing.T) {
			assert.Equal(t, v2Hash, record.CasmHash())
		})

		t.Run("CasmHashAt", func(t *testing.T) {
			t.Run("before migration", func(t *testing.T) {
				hash, err := record.CasmHashAt(149)
				require.NoError(t, err)
				assert.Equal(t, v1Hash, hash)
			})

			t.Run("at migration", func(t *testing.T) {
				hash, err := record.CasmHashAt(150)
				require.NoError(t, err)
				assert.Equal(t, v2Hash, hash)
			})

			t.Run("after migration", func(t *testing.T) {
				hash, err := record.CasmHashAt(200)
				require.NoError(t, err)
				assert.Equal(t, v2Hash, hash)
			})
		})
	})

	t.Run("Migrate error cases", func(t *testing.T) {
		t.Run("cannot migrate V2-declared class", func(t *testing.T) {
			record := core.NewCasmHashMetadataDeclaredV2(declaredAt, &v2Hash)
			err := record.Migrate(migratedAt)
			require.Error(t, err)
			require.ErrorIs(t, err, core.ErrCannotMigrateV2Declared)
		})

		t.Run("cannot migrate to block before declaration", func(t *testing.T) {
			record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)

			t.Run("migrate to same block as declaration", func(t *testing.T) {
				err := record.Migrate(declaredAt)
				require.Error(t, err)
				require.ErrorIs(t, err, core.ErrCannotMigrateBeforeDeclared)
			})

			t.Run("migrate to block before declaration", func(t *testing.T) {
				err := record.Migrate(declaredAt - 1)
				require.Error(t, err)
				require.ErrorIs(t, err, core.ErrCannotMigrateBeforeDeclared)
			})
		})

		t.Run("cannot migrate already migrated class", func(t *testing.T) {
			record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
			require.NoError(t, record.Migrate(migratedAt))

			err := record.Migrate(migratedAt + 1)
			require.Error(t, err)
			require.ErrorIs(t, err, core.ErrCannotMigrateAlreadyMigrated)
		})
	})

	t.Run("Unmigrate", func(t *testing.T) {
		t.Run("error: cannot unmigrate non-migrated class", func(t *testing.T) {
			record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
			err := record.Unmigrate()
			require.Error(t, err)
			require.ErrorIs(t, err, core.ErrCannotUnmigrateNotMigrated)
		})

		t.Run("success: unmigrate migrated class", func(t *testing.T) {
			record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
			require.NoError(t, record.Migrate(migratedAt))

			// Verify it's migrated
			require.True(t, record.IsMigrated())
			require.True(t, record.IsMigratedAt(migratedAt))
			require.True(t, record.IsMigratedAt(migratedAt+1))
			require.False(t, record.IsMigratedAt(migratedAt-1))
			assert.Equal(t, v2Hash, record.CasmHash())

			// Unmigrate
			require.NoError(t, record.Unmigrate())

			// Verify it's no longer migrated
			require.False(t, record.IsMigrated())
			require.False(t, record.IsMigratedAt(migratedAt))
			require.False(t, record.IsMigratedAt(migratedAt+1))
			assert.Equal(t, v1Hash, record.CasmHash())

			// Verify CasmHashAt returns V1 at all heights after declaration
			hash, err := record.CasmHashAt(migratedAt - 1)
			require.NoError(t, err)
			assert.Equal(t, v1Hash, hash)

			hash, err = record.CasmHashAt(migratedAt)
			require.NoError(t, err)
			assert.Equal(t, v1Hash, hash)

			hash, err = record.CasmHashAt(migratedAt + 1)
			require.NoError(t, err)
			assert.Equal(t, v1Hash, hash)
		})

		t.Run("unmigrate then remigrate", func(t *testing.T) {
			record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
			require.NoError(t, record.Migrate(migratedAt))
			require.NoError(t, record.Unmigrate())

			// Should be able to migrate again
			require.NoError(t, record.Migrate(migratedAt+1))
			require.True(t, record.IsMigrated())
			assert.Equal(t, v2Hash, record.CasmHash())

			// Verify migration happened at new block
			hash, err := record.CasmHashAt(migratedAt - 1)
			require.NoError(t, err)
			assert.Equal(t, v1Hash, hash)

			hash, err = record.CasmHashAt(migratedAt + 1)
			require.NoError(t, err)
			assert.Equal(t, v2Hash, hash)
		})
	})

	t.Run("IsDeclaredWithV2", func(t *testing.T) {
		v1Record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
		require.False(t, v1Record.IsDeclaredWithV2())

		v2Record := core.NewCasmHashMetadataDeclaredV2(declaredAt, &v2Hash)
		require.True(t, v2Record.IsDeclaredWithV2())
	})

	t.Run("IsMigrated", func(t *testing.T) {
		record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
		require.False(t, record.IsMigrated())

		require.NoError(t, record.Migrate(migratedAt))
		require.True(t, record.IsMigrated())
	})

	t.Run("IsMigratedAt", func(t *testing.T) {
		record := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
		require.False(t, record.IsMigratedAt(declaredAt))
		require.False(t, record.IsMigratedAt(migratedAt))
		require.False(t, record.IsMigratedAt(migratedAt+1))

		require.NoError(t, record.Migrate(migratedAt))
		require.False(t, record.IsMigratedAt(migratedAt-1))
		require.True(t, record.IsMigratedAt(migratedAt))
		require.True(t, record.IsMigratedAt(migratedAt+1))
	})

	t.Run("MarshalBinary and UnmarshalBinary not migrated V1 class", func(t *testing.T) {
		original := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)

		data, err := original.MarshalBinary()
		require.NoError(t, err)
		// 8 bytes for declaredAt
		// 32 bytes for casmHashV2
		// 1 byte for migratedAt flag
		// 1 byte for casmHashV1 flag, 32 bytes for casmHashV1
		require.Equal(t, 8+32+1+1+32, len(data))

		var unmarshaled core.ClassCasmHashMetadata
		err = unmarshaled.UnmarshalBinary(data)
		require.NoError(t, err)

		assert.Equal(t, v1Hash, unmarshaled.CasmHash())

		hash, err := unmarshaled.CasmHashAt(declaredAt)
		require.NoError(t, err)
		assert.Equal(t, v1Hash, hash)
	})

	t.Run("MarshalBinary and UnmarshalBinary migrated V1 class", func(t *testing.T) {
		original := core.NewCasmHashMetadataDeclaredV1(declaredAt, &v1Hash, &v2Hash)
		require.NoError(t, original.Migrate(migratedAt))

		data, err := original.MarshalBinary()
		require.NoError(t, err)
		// 8 bytes for declaredAt
		// 32 bytes for casmHashV2
		// 1 byte for migratedAt flag, 8 bytes for migratedAt
		// 1 byte for casmHashV1 flag, 32 bytes for casmHashV1
		require.Equal(t, 8+32+1+1+8+32, len(data))

		var unmarshaled core.ClassCasmHashMetadata
		err = unmarshaled.UnmarshalBinary(data)
		require.NoError(t, err)

		assert.Equal(t, v2Hash, unmarshaled.CasmHash()) // Should return V2 since migrated

		// Test at different heights
		hash, err := unmarshaled.CasmHashAt(declaredAt)
		require.NoError(t, err)
		assert.Equal(t, v1Hash, hash) // Before migration

		hash, err = unmarshaled.CasmHashAt(migratedAt)
		require.NoError(t, err)
		assert.Equal(t, v2Hash, hash) // At migration
	})

	t.Run("MarshalBinary and UnmarshalBinary for V2-only", func(t *testing.T) {
		original := core.NewCasmHashMetadataDeclaredV2(declaredAt, &v2Hash)

		data, err := original.MarshalBinary()
		require.NoError(t, err)
		// 8 bytes for declaredAt
		// 32 bytes for casmHashV2
		// 1 byte for migratedAt flag
		// 1 byte for casmHashV1 flag
		require.Equal(t, 8+32+1+1, len(data))
		var unmarshaled core.ClassCasmHashMetadata
		err = unmarshaled.UnmarshalBinary(data)
		require.NoError(t, err)

		hash := unmarshaled.CasmHash()
		assert.Equal(t, v2Hash, hash)

		hash, err = unmarshaled.CasmHashAt(declaredAt)
		require.NoError(t, err)
		assert.Equal(t, v2Hash, hash)
	})
}

var benchmarkClassV0Fixtures = []struct {
	name      string
	classHash string
}{
	{
		name:      "smallerDeprecatedClass",
		classHash: "0x07db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909",
	},
	{
		name:      "largerDeprecatedClass",
		classHash: "0x0772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3",
	},
}

func BenchmarkClassV0Hash(b *testing.B) {
	client := feeder.NewTestClient(b, &networks.Sepolia)
	gw := adaptfeeder.New(client)

	for _, benchmark := range benchmarkClassV0Fixtures {
		hash := felt.UnsafeFromString[felt.Felt](benchmark.classHash)
		class, err := gw.Class(b.Context(), &hash)
		require.NoError(b, err)

		deprecatedClass, ok := class.(*core.DeprecatedCairoClass)
		require.True(b, ok)

		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_, err := deprecatedClass.Hash()
				require.NoError(b, err)
			}
		})
	}
}

// BenchmarkSierraClassHash measures the Poseidon-based Sierra (Cairo 1) class hash.
func BenchmarkSierraClassHash(b *testing.B) {
	client := feeder.NewTestClient(b, &networks.Integration)
	gw := adaptfeeder.New(client)

	key := "0x6d8ede036bb4720e6f348643221d8672bf4f0895622c32c11e57460b3b7dffc"
	hash := felt.UnsafeFromString[felt.Felt](key)
	class, err := gw.Class(b.Context(), &hash)
	require.NoError(b, err)
	sierra, ok := class.(*core.SierraClass)
	require.True(b, ok)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_, err := sierra.Hash()
		require.NoError(b, err)
	}
}
