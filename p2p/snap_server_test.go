package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"testing"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/adapters/p2p2core"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"github.com/stretchr/testify/assert"
)

func TestClassRange(t *testing.T) {
	// Note: set to true to make test super long to complete
	shouldFetchAllClasses := true
	var d db.DB
	//t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot
	logger, _ := utils.NewZapLogger(utils.DEBUG, false)
	server := &snapServer{
		log:        logger,
		blockchain: bc,
	}

	startRange := (&felt.Felt{}).SetUint64(0)
	finMsgReceived := false
	chunksReceived := 0

	chunksPerProof := 150
	if shouldFetchAllClasses {
		// decrease iteration count and hence speed up a bit
		chunksPerProof *= 4
	}
	iter, err := server.GetClassRange(
		&spec.ClassRangeRequest{
			Root:           core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptHash(startRange),
			ChunksPerProof: uint32(chunksPerProof),
		})
	assert.NoError(t, err)

	for res := range iter {
		assert.NotNil(t, res)

		resT, ok := res.(*spec.ClassRangeResponse)
		assert.True(t, ok)
		assert.NotNil(t, resT)

		switch v := resT.GetResponses().(type) {
		case *spec.ClassRangeResponse_Classes:
			assert.True(t, chunksPerProof >= len(v.Classes.Classes))
			classesRoot := p2p2core.AdaptHash(resT.ClassesRoot)
			contractsRoot := p2p2core.AdaptHash(resT.ContractsRoot)
			verifyErr := VerifyGlobalStateRoot(stateRoot, classesRoot, contractsRoot)
			assert.NoError(t, verifyErr)
			chunksReceived++
		case *spec.ClassRangeResponse_Fin:
			finMsgReceived = true
		}

		if !shouldFetchAllClasses {
			break
		}
	}

	if !shouldFetchAllClasses {
		assert.Equal(t, 1, chunksReceived)
		assert.False(t, finMsgReceived)
	} else {
		fmt.Printf("ClassesReceived: \t%d\n", chunksReceived)
		assert.True(t, finMsgReceived)
		assert.True(t, chunksReceived > 1)
	}
}

func TestContractRange(t *testing.T) {
	var d db.DB
	//t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot

	logger, _ := utils.NewZapLogger(utils.DEBUG, false)
	server := &snapServer{
		log:        logger,
		blockchain: bc,
	}

	startRange := (&felt.Felt{}).SetUint64(0)
	chunksReceived := 0

	chunksPerProof := 150
	ctrIter, err := server.GetContractRange(
		&spec.ContractRangeRequest{
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			Start:          core2p2p.AdaptAddress(startRange),
			ChunksPerProof: uint32(chunksPerProof),
		})
	assert.NoError(t, err)

	for res := range ctrIter {
		assert.NotNil(t, res)

		resT, ok := res.(*spec.ContractRangeResponse)
		assert.True(t, ok)
		assert.NotNil(t, resT)

		switch v := resT.GetResponses().(type) {
		case *spec.ContractRangeResponse_Range:
			assert.True(t, chunksPerProof == len(v.Range.State))
			classesRoot := p2p2core.AdaptHash(resT.ClassesRoot)
			contractsRoot := p2p2core.AdaptHash(resT.ContractsRoot)
			verifyErr := VerifyGlobalStateRoot(stateRoot, classesRoot, contractsRoot)
			assert.NoError(t, verifyErr)
			chunksReceived++
		default:
			// we expect no any other message only just one range because we break the iteration
			t.Fatal("received unexpected message", "type", v)
		}

		// we don't need to fetch all contracts
		break
	}

	assert.Equal(t, 1, chunksReceived)
}

func TestContractRange_FinMsg_Received(t *testing.T) {
	// TODO: Fix the test so it demonstrated FinMsg is returned at the iteration end
	t.Skip("Fix me")
	var d db.DB = pebble.NewMemTest(t)
	bc := blockchain.New(d, &utils.Sepolia)
	defer bc.Close()
	server := &snapServer{blockchain: bc}

	zero := new(felt.Felt).SetUint64(0)
	iter, err := server.GetContractRange(
		&spec.ContractRangeRequest{
			StateRoot:      core2p2p.AdaptHash(zero),
			Start:          core2p2p.AdaptAddress(zero),
			ChunksPerProof: uint32(10),
		})
	assert.NoError(t, err)
	fmt.Printf("All Good!\n")

	finMsgReceived := false
	for res := range iter {
		assert.NotNil(t, res)
		resT, ok := res.(*spec.ContractRangeResponse)
		assert.True(t, ok)
		assert.NotNil(t, resT)
		assert.IsType(t, spec.ContractRangeResponse_Fin{}, resT)
		finMsgReceived = true
	}
	assert.True(t, finMsgReceived)
}

func TestContractStorageRange(t *testing.T) {
	var d db.DB
	//t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	stateRoot := b.GlobalStateRoot

	logger, _ := utils.NewZapLogger(utils.DEBUG, false)
	server := &snapServer{
		log:        logger,
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
			request := &spec.ContractStorageRequest{
				StateRoot:      core2p2p.AdaptHash(stateRoot),
				ChunksPerProof: 100,
				Query: []*spec.StorageRangeQuery{
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
			stoIter, err := server.GetStorageRange(request)
			assert.NoError(t, err)

			finMsgReceived := false
			for res := range stoIter {
				assert.NotNil(t, res)
				resT, ok := res.(*spec.ContractStorageResponse)
				assert.True(t, ok)
				assert.NotNil(t, resT)

				switch v := resT.GetResponses().(type) {
				case *spec.ContractStorageResponse_Storage:
					assert.False(t, finMsgReceived)
					for _, r := range v.Storage.KeyValue {
						keys = append(keys, p2p2core.AdaptFelt(r.Key))
						vals = append(vals, p2p2core.AdaptFelt(r.Value))
					}
				case *spec.ContractStorageResponse_Fin:
					// we expect just one fin message at the iteration end
					finMsgReceived = true
				}
			}
			assert.True(t, finMsgReceived)

			fmt.Println("Address:", test.address, "storage length:", len(keys))
			assert.Equal(t, test.expectedLeaves, len(keys))

			hasMore, err := VerifyTrie(test.storageRoot, keys, vals, nil, core.ContractStorageTrieHeight, crypto.Pedersen)
			assert.NoError(t, err)
			assert.False(t, hasMore)
		})
	}
}

func TestGetClassesByHash(t *testing.T) {
	var d db.DB
	//t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/juno-sepolia", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia) // Needed because class loader need encoder to be registered

	b, err := bc.Head()
	assert.NoError(t, err)

	fmt.Printf("headblock %d\n", b.Number)

	logger, _ := utils.NewZapLogger(utils.DEBUG, false)
	server := &snapServer{
		log:        logger,
		blockchain: bc,
	}

	hashes := []*spec.Hash{
		// Block <number>, type v0
		core2p2p.AdaptHash(feltFromString("0x7db5c2c2676c2a5bfc892ee4f596b49514e3056a0eee8ad125870b4fb1dd909")),
		// Block <number>, type v0
		core2p2p.AdaptHash(feltFromString("0x28d1671fb74ecb54d848d463cefccffaef6df3ae40db52130e19fe8299a7b43")),
		// Block <number>, type v0
		core2p2p.AdaptHash(feltFromString("0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3")),
		// Block <number>, type v0
		core2p2p.AdaptHash(feltFromString("0x78401746828463e2c3f92ebb261fc82f7d4d4c8d9a80a356c44580dab124cb0")),
	}

	finMsgReceived := false
	iter, err := server.GetClasses(
		&spec.ClassHashesRequest{
			ClassHashes: hashes,
		})
	assert.NoError(t, err)

	i := 0
	for res := range iter {
		assert.NotNil(t, res)

		resT, ok := res.(*spec.ClassesResponse)
		assert.True(t, ok)
		assert.NotNil(t, resT)

		switch v := resT.GetClassMessage().(type) {
		case *spec.ClassesResponse_Class:
			assert.True(t, i < len(hashes))
			assert.Equal(t, v.Class.GetClassHash(), hashes[i])
		case *spec.ClassesResponse_Fin:
			assert.Equal(t, len(hashes), i)
			finMsgReceived = true
		}

		i++
	}
	assert.True(t, finMsgReceived)
}

func feltFromString(str string) *felt.Felt {
	f, err := (&felt.Felt{}).SetString(str)
	if err != nil {
		panic(err)
	}
	return f
}
