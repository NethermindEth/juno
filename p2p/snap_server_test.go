package p2p

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/stretchr/testify/require"
	"maps"
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
	shouldFetchAllClasses := false
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
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
	t.Skip("DB snapshot is needed for this test")
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

func TestContractRangeByOneContract(t *testing.T) {
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
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

	tests := []struct {
		address             *felt.Felt
		expectedStorageRoot *felt.Felt
		expectedClassHash   *felt.Felt
		expectedNonce       uint64
	}{
		{
			address:             feltFromString("0x27b0a1ba755185b8d05126a1e00ca687e6680e51d634b5218760b716b8d06"),
			expectedStorageRoot: feltFromString("0xa8d7943793ddd09e49b8650a71755ed04d0de087b28ad5967b519864f9844"),
			expectedClassHash:   feltFromString("0x772164c9d6179a89e7f1167f099219f47d752304b16ed01f081b6e0b45c93c3"),
			expectedNonce:       0,
		},
		{
			address:             feltFromString("0x292854fdd7653f65d8adc66739866567c212f2ef15ad8616e713eafc97e0a"),
			expectedStorageRoot: feltFromString("0x718f57f8cd2950a0f240941876eafdffe86c84bd2601de4ea244956d96d85b6"),
			expectedClassHash:   feltFromString("0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b"),
			expectedNonce:       2,
		},
		{
			address:             feltFromString("0xf3c569521d6ca43a0e2b86fd251031e2158aae502bf199f7eec986fe348f"),
			expectedStorageRoot: feltFromString("0x41c2705457dfa3872cbc862ac86c85d118259154f08408c3cd350a15646d596"),
			expectedClassHash:   feltFromString("0x29927c8af6bccf3f6fda035981e765a7bdbf18a2dc0d630494f8758aa908e2b"),
			expectedNonce:       1,
		},
		{
			address:             feltFromString("0x5edd7ece0dc39633f9825e16e39e17a0bded7bf540a685876ceb75cdfd9eb"),
			expectedStorageRoot: feltFromString("0x26f6e269f9462b6bf1649394aa4080d40ca4f4bc792bd0c1a72a8b524a93a9d"),
			expectedClassHash:   feltFromString("0x66559c86e66214ba1bc5d6512f6411aa066493e6086ff5d54f41a970d47fc5a"),
			expectedNonce:       0,
		},
		{
			address:             feltFromString("0x4c04ec7c3c5a82df2d194095f090af83a9f26e22544d968c3d67c1b320d43"),
			expectedStorageRoot: feltFromString("0x0"),
			expectedNonce:       0,
		},
		{
			address:             feltFromString("0x4ccd60176f9e757031f04691beb09832a0ea583eeb5158b05277547957514"),
			expectedStorageRoot: feltFromString("0x0"),
			expectedNonce:       1,
		},
		{
			address:             feltFromString("0xd94fd19a7730f84df43999562cbbf5cf8d48a6cb92f5bc5d6795f34c15f72"),
			expectedStorageRoot: feltFromString("0x0"),
			expectedNonce:       0,
		},
		{
			address:             feltFromString("0xdd92645559c6dca08c6e947b4a40a55142a0a8b65552be8b31c885f37ef87"),
			expectedStorageRoot: feltFromString("0x0"),
			expectedNonce:       1,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%.7s...", test.address), func(t *testing.T) {
			chunksPerProof := 5
			ctrIter, err := server.GetContractRange(
				&spec.ContractRangeRequest{
					StateRoot:      core2p2p.AdaptHash(stateRoot),
					Start:          core2p2p.AdaptAddress(test.address),
					End:            core2p2p.AdaptAddress(test.address),
					ChunksPerProof: uint32(chunksPerProof),
				})
			assert.NoError(t, err)

			finReceived := false
			chunksReceived := 0

			for res := range ctrIter {
				assert.NotNil(t, res)

				resT, ok := res.(*spec.ContractRangeResponse)
				assert.True(t, ok)
				assert.NotNil(t, resT)

				switch v := resT.GetResponses().(type) {
				case *spec.ContractRangeResponse_Range:
					crctStates := v.Range.State
					assert.Len(t, crctStates, 1)

					crct := crctStates[0]
					address := p2p2core.AdaptAddress(crct.Address)
					assert.Equal(t, test.address, address)

					storageRoot := p2p2core.AdaptHash(crct.Storage)
					assert.Equal(t, test.expectedStorageRoot, storageRoot)

					//classHash := p2p2core.AdaptHash(crct.Class)
					//assert.Equal(t, test.expectedClassHash, classHash, "classHash", classHash)

					assert.Equal(t, test.expectedNonce, crct.Nonce)

					chunksReceived++
				case *spec.ContractRangeResponse_Fin:
					assert.Equal(t, 1, chunksReceived)
					finReceived = true
				default:
					// we expect no any other message only just one range because we break the iteration
					t.Fatal("received unexpected message", "type", v)
				}
			}

			assert.True(t, finReceived)
		})
	}
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
	t.Skip("DB snapshot is needed for this test")
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
			address:        feltFromString("0x5eb8d1bc5aaf2f323f2a807d429686ac012ca16f90740071d2f3a160dc231"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 0,
		},
		{
			address:        feltFromString("0x614a5e0519963324acb5640321240827c0cd6a9f7cf5f17a80c1596e607d0"),
			storageRoot:    feltFromString("0x55ee7fd57d0aa3da8b89ea2feda16f9435186988a8b00b6f22f5ba39f3cf172"),
			expectedLeaves: 1,
		},
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
	t.Skip("DB snapshot is needed for this test")
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

func Test__Finding_Storage_Heavy_Contract(t *testing.T) {
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
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

	ctso := make(map[felt.Felt]*felt.Felt)
	request := &spec.ContractRangeRequest{
		ChunksPerProof: 100,
		Start:          core2p2p.AdaptAddress(felt.Zero.Clone()),
		End:            nil, //core2p2p.AdaptAddress(test.address),
		StateRoot:      core2p2p.AdaptHash(stateRoot),
	}

	iter, err := server.GetContractRange(request)
	assert.NoError(t, err)

	contracts := 0
	for res := range iter {
		assert.NotNil(t, res)
		resT, ok := res.(*spec.ContractRangeResponse)
		assert.True(t, ok)
		assert.NotNil(t, resT)

		switch v := resT.GetResponses().(type) {
		case *spec.ContractRangeResponse_Range:
			for _, contract := range v.Range.State {
				addr := p2p2core.AdaptAddress(contract.Address)
				strt := p2p2core.AdaptHash(contract.Storage)
				//assert.Equal(t, test.address, addr)
				//assert.Equal(t, test.storageRoot, strt)
				if !(strt.IsZero() || addr.IsOne()) {
					ctso[*addr] = strt
					contracts++
				}
			}
		case *spec.ContractRangeResponse_Fin:
			fmt.Println("Contract iteration ends", "contracts", contracts)
		default:
			// we expect no any other message only just one range because we break the iteration
			t.Fatal("received unexpected message", "type", v)
		}

		if contracts > 100 {
			break
		}
	}

	keys := make([]*felt.Felt, 0, len(ctso))
	stoCnt := make(map[felt.Felt]int)
	for k := range maps.Keys(ctso) {
		keys = append(keys, k.Clone())
	}

	for len(keys) > 10 {
		var queries []*spec.StorageRangeQuery
		for i := range 10 {
			addr := keys[i]
			queries = append(queries, &spec.StorageRangeQuery{
				Address: core2p2p.AdaptAddress(addr),
				Start: &spec.StorageLeafQuery{
					ContractStorageRoot: core2p2p.AdaptHash(ctso[*addr]),
					Key:                 core2p2p.AdaptFelt(felt.Zero.Clone()),
				},
				End: nil,
			})
		}
		keys = keys[10:]

		sreq := &spec.ContractStorageRequest{
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			ChunksPerProof: uint32(500),
			Query:          queries,
		}

		iter, err := server.GetStorageRange(sreq)
		require.NoError(t, err)

		for res := range iter {
			assert.NotNil(t, res)
			resT, ok := res.(*spec.ContractStorageResponse)
			assert.True(t, ok)
			assert.NotNil(t, resT)

			addr := p2p2core.AdaptAddress(resT.ContractAddress)

			switch v := resT.GetResponses().(type) {
			case *spec.ContractStorageResponse_Storage:
				vl := stoCnt[*addr]
				//if !ok { stoCnt[*addr] = 0 }
				stoCnt[*addr] = vl + len(v.Storage.KeyValue)
			case *spec.ContractStorageResponse_Fin:
				// we expect just one fin message at the iteration end
				fmt.Println("End of iter", "no addr")
			}
		}
	}

	for addr, cnt := range stoCnt {
		if cnt <= 3 {
			fmt.Printf("[%5d]: address %s, storageRoot %s\n", cnt, &addr, ctso[addr])
		}
	}
}

func TestGetContractStorageRoot(t *testing.T) {
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
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

	tests := []struct {
		address        *felt.Felt
		storageRoot    *felt.Felt
		expectedLeaves int
	}{
		{
			address:        feltFromString("0x2375219f8c73b77eef29d7cfd3749d64d2cca6ad7a776ed85bd33ff09201ea"),
			storageRoot:    feltFromString("0x24e11cc263e8d4c37519a95dc5cc4bc2627a991da6048b9a41f011e586cd3cc"),
			expectedLeaves: 140,
		},
		{
			address:        feltFromString("0xec1131fe035c235c03e0ad43646d8cbfd59d048b1825b0a36a167c468d5bf"),
			storageRoot:    feltFromString("0x21c409ca7f7d064d5e580e756cc945d3b266ab852e6d982697177e57d4c96a0"),
			expectedLeaves: 193,
		},
		{
			address:        feltFromString("0xc41025be6d90828b1af119d384cecf1a57da8190ce79a2ffd925f02b59df"),
			storageRoot:    feltFromString("0x3f6b341ce4fb8441a0b350932a89cb19e195479e43ade9eb9e2fcde31a64680"),
			expectedLeaves: 187,
		},
		{
			address:        feltFromString("0xb5a14ddd6d1a6b33a10411e45bbff54f92265ede856cd0e12fee4a638c389"),
			storageRoot:    feltFromString("0x331a990cf32f6eedf01ca9b577cb75b53d937333729f48091945e255fff4a3d"),
			expectedLeaves: 137,
		},
		{
			address:        feltFromString("0x130b5a3035eef0470cff2f9a450a7a6856a3c5a4ea3f5b7886c2d03a50d2bf"),
			storageRoot:    feltFromString("0x4cf3afb0828518a24c0f2a5cc6e87d188df58b5faf40c6b81d6d476cf7897f6"),
			expectedLeaves: 338,
		},
		{
			address:        feltFromString("0x267311365224e8d4eb4dd580f1b737f990dfc81112ca71ecce147e774bcecb"),
			storageRoot:    feltFromString("0x71b0e71d2b69bfbfa25fd54e0cd3673f27f07110c3be72cb81c20aa0c6df4b0"),
			expectedLeaves: 701,
		},
		{
			address:        feltFromString("0x28f61c91275111e8c5af6febfa5c9d2191442b4fe48d30a84e51a09e8f18b5"),
			storageRoot:    feltFromString("0x1de1bf792cd9221c8d6ba2953671d6a4f0890375eca3718989082225dccb7eb"),
			expectedLeaves: 540,
		},
		{
			address:        feltFromString("0x4c04ec7c3c5a82df2d194095f090af83a9f26e22544d968c3d67c1b320d43"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 0,
		},
		{
			address:        feltFromString("0x4ccd60176f9e757031f04691beb09832a0ea583eeb5158b05277547957514"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 1,
		},
		{
			address:        feltFromString("0xd94fd19a7730f84df43999562cbbf5cf8d48a6cb92f5bc5d6795f34c15f72"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 0,
		},
		{
			address:        feltFromString("0xdd92645559c6dca08c6e947b4a40a55142a0a8b65552be8b31c885f37ef87"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 1,
		},
		{
			address:        feltFromString("0x807dd1766d3c833ac82290e38a000b7d48acce3cf125cffde15ea9e583a95"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 0,
		},
		{
			address:        feltFromString("0x8502b172cb17395511c4bfabb7ded748cd930fec8201ad6b444f9d6a9df7a"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 1,
		},
		{
			address:        feltFromString("0x5788cee963fe68a76b1ef9b04f1ba404043d853ad593c366c723322381b14"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 0,
		},
		{
			address:        feltFromString("0x5e8dab06aaf28538be2077fddd679cac934c5e082303f892235fb989e800c"),
			storageRoot:    feltFromString("0x0"),
			expectedLeaves: 1,
		},
	}

	ctso := make(map[felt.Felt]*felt.Felt)
	stoCnt := make(map[felt.Felt]int)

	t.Run("Storage validation ", func(t *testing.T) {
		queries := []*spec.StorageRangeQuery{}
		for _, dt := range tests {
			queries = append(queries, &spec.StorageRangeQuery{
				Address: core2p2p.AdaptAddress(dt.address),
				Start: &spec.StorageLeafQuery{
					ContractStorageRoot: core2p2p.AdaptHash(dt.storageRoot),
					Key:                 core2p2p.AdaptFelt(felt.Zero.Clone()),
				},
				End: nil,
			})
			ctso[*dt.address] = dt.storageRoot
		}

		// Contract Storage request contains queries for each tests' contract
		// also note we specify `chainsPerProof` to be 200, which will be not enough
		// to get all contract's keys in one iteration
		sreq := &spec.ContractStorageRequest{
			StateRoot:      core2p2p.AdaptHash(stateRoot),
			ChunksPerProof: uint32(200),
			Query:          queries,
		}

		iter, err := server.GetStorageRange(sreq)
		require.NoError(t, err)

		// There will be one iteration for contracts where `expectedLeaves` < `chunksPerProof`
		// and several iterations otherwise
		// only at the end (all queries are responded) we will get `Fin` message
		for res := range iter {
			assert.NotNil(t, res)
			resT, ok := res.(*spec.ContractStorageResponse)
			assert.True(t, ok)
			assert.NotNil(t, resT)

			switch v := resT.GetResponses().(type) {
			case *spec.ContractStorageResponse_Storage:
				addr := p2p2core.AdaptAddress(resT.ContractAddress)
				vl := stoCnt[*addr]
				stoCnt[*addr] = vl + len(v.Storage.KeyValue)
				fmt.Printf("Response for %s: %d\n", addr, stoCnt[*addr])

			case *spec.ContractStorageResponse_Fin:
				// we expect just one fin message at the iteration end
				fmt.Println("End of iter", "no addr")
			}
		}
	})

	for _, test := range tests {
		t.Run(fmt.Sprintf("%.7s...", test.address), func(t *testing.T) {
			// validate storage leaves count
			assert.Equal(t, test.expectedLeaves, stoCnt[*test.address])
		})
	}
}

func TestReadAndVerifySnapshot(t *testing.T) {
	var d db.DB
	t.Skip("DB snapshot is needed for this test")
	d, _ = pebble.NewWithOptions("/Users/pnowosie/juno/snapshots/node1", 128000000, 128, false)
	defer func() { _ = d.Close() }()
	bc := blockchain.New(d, &utils.Sepolia)

	logger, _ := utils.NewZapLogger(utils.DEBUG, false)
	syncer := SnapSyncer{
		log:                    logger,
		blockchain:             bc,
		currentGlobalStateRoot: feltFromString("0x472e84b65d387c9364b5117f4afaba3fb88897db1f28867b398506e2af89f25"),
	}

	err := syncer.PhraseVerify(context.Background())
	assert.NoError(t, err)
}

func TestPercentageCalculation(t *testing.T) {
	tests := []struct {
		actual  *felt.Felt
		percent uint64
	}{
		{
			actual:  feltFromString("0x0"),
			percent: 0,
		},
		{
			// actual felt.MaxValue:2^251 + 17 * 2^192
			actual:  feltFromString("0x800000000000011000000000000000000000000000000000000000000000000"),
			percent: 100,
		},
		{
			actual:  feltFromString("0x400000000000008800000000000000000000000000000000000000000000000"),
			percent: 50,
		},
		{
			actual:  feltFromString("0x0ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"),
			percent: 12,
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%d%%", test.percent), func(t *testing.T) {
			percent := CalculatePercentage(test.actual)
			assert.Equal(t, test.percent, percent)
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
