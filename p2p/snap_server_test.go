package p2p

import (
	"context"
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestClassRange(t *testing.T) {
	var d db.DB
	//t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot

	server := &snapServer{
		blockchain: bc,
	}

	startRange := (&felt.Felt{}).SetUint64(0)

	chunksPerProof := 150
	var classResult *ClassRangeStreamingResult
	iter, err := server.GetClassRange(context.Background(),
		&spec.ClassRangeRequest{
			Root:           core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptHash(startRange),
			ChunksPerProof: uint32(chunksPerProof),
		})
	if err != nil {
		fmt.Printf("err %s\n", err)
		t.Fatal(err)
	}
	iter(func(result *ClassRangeStreamingResult) bool {
		if result != nil {
			classResult = result
		}

		return false
	})

	assert.NotNil(t, classResult)
	assert.Equal(t, chunksPerProof, len(classResult.Range.Classes))
	verifyErr := VerifyGlobalStateRoot(stateRoot, classResult.ClassesRoot, classResult.ContractsRoot)
	assert.NoError(t, verifyErr)
}

func TestContractRange(t *testing.T) {
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot

	server := &snapServer{
		blockchain: bc,
	}

	startRange := (&felt.Felt{}).SetUint64(0)

	chunksPerProof := 150
	var contractResult *ContractRangeStreamingResult
	ctrIter, err := server.GetContractRange(context.Background(),
		&spec.ContractRangeRequest{
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptAddress(startRange),
			ChunksPerProof: uint32(chunksPerProof),
		})
	assert.NoError(t, err)

	ctrIter(func(result *ContractRangeStreamingResult) bool {
		if err != nil {
			fmt.Printf("err %s\n", err)
			t.Fatal(err)
		}

		if result != nil {
			contractResult = result
		}

		return false
	})

	assert.NotNil(t, contractResult)
	assert.Equal(t, chunksPerProof, len(contractResult.Range))
	verifyErr := VerifyGlobalStateRoot(stateRoot, contractResult.ClassesRoot, contractResult.ContractsRoot)
	assert.NoError(t, verifyErr)
}

func TestContractStorageRange(t *testing.T) {
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot

	server := &snapServer{
		blockchain: bc,
	}

	startRange := (&felt.Felt{}).SetUint64(0)

	tests := []struct {
		address        *felt.Felt
		storageRoot    *felt.Felt
		expectedLeaves int
	}{
		{
			address:        feltFromString("0x3deecdb26a60e4c062d5bd98ab37f72ea2acc37f28dae6923359627ebde9"),
			storageRoot:    feltFromString("0x276edbc91a945d11645ba0b8298c7d657e554d06ab2bb765cbc44d61fa01fd5"),
			expectedLeaves: 1,
		},
		{
			address:        feltFromString("0x5de00d3720421ab00fdbc47d33d253605c1ac226ab1a0d267f7d57e23305"),
			storageRoot:    feltFromString("0x5eebb2c6722d321469cb662260c5171c9f6f67b9a625c9c9ab56b0a4631b0fe"),
			expectedLeaves: 2,
		},
		{
			address:        feltFromString("0x1ee60ed3c5abd9a08c61de5e8cbcf32b49646e681ee6e84da9d52f5c3099"),
			storageRoot:    feltFromString("0x60dccd54f4956147c6a499b71579820d181e22d5e9c430fd5953f861ca7727e"),
			expectedLeaves: 4,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%.7s...", test.address), func(t *testing.T) {
			request := &StorageRangeRequest{
				StateRoot:     stateRoot,
				ChunkPerProof: 100,
				Queries: []*spec.StorageRangeQuery{
					{
						Address: core2p2p.AdaptAddress(test.address),
						Start: &spec.StorageLeafQuery{
							ContractStorageRoot: core2p2p.AdaptHash(test.storageRoot),
							Key:                 core2p2p.AdaptFelt(startRange),
						},
						End: nil,
					},
				},
			}

			keys := make([]*felt.Felt, 0, test.expectedLeaves)
			vals := make([]*felt.Felt, 0, test.expectedLeaves)
			stoIter, err := server.GetStorageRange(context.Background(), request)
			assert.NoError(t, err)

			stoIter(func(result *StorageRangeStreamingResult) bool {
				if result != nil {
					for _, r := range result.Range {
						keys = append(keys, p2p2core.AdaptFelt(r.Key))
						vals = append(vals, p2p2core.AdaptFelt(r.Value))
					}
				}

				return true
			})

			fmt.Println("Address:", test.address, "storage length:", len(keys))
			assert.Equal(t, test.expectedLeaves, len(keys))

			hasMore, err := VerifyTrie(test.storageRoot, keys, vals, nil, core.ContractStorageTrieHeight, crypto.Pedersen)
			assert.NoError(t, err)
			assert.False(t, hasMore)
		})
	}
}

func feltFromString(str string) *felt.Felt {
	f, err := (&felt.Felt{}).SetString(str)
	if err != nil {
		panic(err)
	}
	return f
}
