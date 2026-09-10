package rpcv10_test

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	statetestutils "github.com/NethermindEth/juno/core/state/testutils"
	"github.com/NethermindEth/juno/core/trie"
	"github.com/NethermindEth/juno/core/trie2"
	"github.com/NethermindEth/juno/core/trie2/trienode"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db/memory"
	"github.com/NethermindEth/juno/mocks"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func BenchmarkStorageProof(b *testing.B) {
	fixture := newStorageProofBenchmarkFixture()
	cases := fixture.cases()
	for i := range cases {
		tc := &cases[i]
		b.Run(tc.name, func(b *testing.B) {
			benchmarkStorageProofCase(b, fixture, tc)
		})
	}
}

type storageProofBenchmarkFixture struct {
	blkHash         *felt.Felt
	nonce           *felt.Felt
	classHash       *felt.Felt
	blockLatest     rpc.BlockID
	blockNumber     uint64
	smallClasses    []felt.Felt
	smallContracts  []felt.Felt
	storageTrieKeys []felt.Felt
	manyClasses     []felt.Felt
	manyContracts   []felt.Felt
}

type storageProofBenchmarkCase struct {
	name             string
	classTrieKeys    []felt.Felt
	contractTrieKeys []felt.Felt
	storageTrieKeys  []felt.Felt
	classes          []felt.Felt
	contracts        []felt.Felt
	blockID          rpc.BlockID
}

func newStorageProofBenchmarkFixture() *storageProofBenchmarkFixture {
	key := felt.NewFromUint64[felt.Felt](1)
	key2 := felt.NewFromUint64[felt.Felt](8)
	contract := felt.NewFromUint64[felt.Felt](0xadd0)

	return &storageProofBenchmarkFixture{
		blkHash:         felt.NewFromUint64[felt.Felt](0x11ead),
		nonce:           felt.NewFromUint64[felt.Felt](121),
		classHash:       felt.NewFromUint64[felt.Felt](1234),
		blockLatest:     rpc.BlockIDLatest(),
		blockNumber:     1313,
		smallClasses:    []felt.Felt{*key, *key2},
		smallContracts:  []felt.Felt{*contract},
		storageTrieKeys: []felt.Felt{*key, *key2},
		manyClasses:     benchmarkUnsortedFelts(0x1000, 128),
		manyContracts:   benchmarkUnsortedFelts(0x2000, 128),
	}
}

func (f *storageProofBenchmarkFixture) cases() []storageProofBenchmarkCase {
	return []storageProofBenchmarkCase{
		{
			name:             "many classes",
			classTrieKeys:    f.manyClasses,
			contractTrieKeys: f.smallContracts,
			storageTrieKeys:  f.storageTrieKeys,
			classes:          f.manyClasses,
			blockID:          f.blockLatest,
		},
		{
			name:             "many contracts",
			classTrieKeys:    f.smallClasses,
			contractTrieKeys: f.manyContracts,
			storageTrieKeys:  f.storageTrieKeys,
			contracts:        f.manyContracts,
			blockID:          f.blockLatest,
		},
		{
			name:             "many classes and contracts",
			classTrieKeys:    f.manyClasses,
			contractTrieKeys: f.manyContracts,
			storageTrieKeys:  f.storageTrieKeys,
			classes:          f.manyClasses,
			contracts:        f.manyContracts,
			blockID:          f.blockLatest,
		},
	}
}

func benchmarkStorageProofCase(
	b *testing.B,
	fixture *storageProofBenchmarkFixture,
	tc *storageProofBenchmarkCase,
) {
	b.Helper()

	handler, finish := fixture.setup(
		b,
		tc.classTrieKeys,
		tc.contractTrieKeys,
		tc.storageTrieKeys,
	)
	defer finish()

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		proof, rpcErr := handler.StorageProof(&tc.blockID, tc.classes, tc.contracts, nil)
		if rpcErr != nil {
			b.Fatal(rpcErr)
		}
		if proof == nil {
			b.Fatal("expected proof")
		}
	}
}

func (f *storageProofBenchmarkFixture) setup(
	b *testing.B,
	classTrieKeys []felt.Felt,
	contractTrieKeys []felt.Felt,
	storageTrieKeys []felt.Felt,
) (*rpc.Handler, func()) {
	b.Helper()

	classTrie, contractTrie, storageTrie := benchmarkStorageProofTries(
		b,
		classTrieKeys,
		contractTrieKeys,
		storageTrieKeys,
	)
	mockCtrl := gomock.NewController(b)
	mockReader := mocks.NewMockReader(mockCtrl)
	mockState := mocks.NewMockStateReader(mockCtrl)

	mockReader.EXPECT().Height().Return(f.blockNumber, nil).AnyTimes()
	mockReader.EXPECT().BlockHeaderHashByNumber(f.blockNumber).Return(f.blkHash, nil).AnyTimes()
	mockReader.EXPECT().HeadState().Return(mockState, func() error { return nil }, nil).AnyTimes()

	mockState.EXPECT().ClassTrie().Return(classTrie, nil).AnyTimes()
	mockState.EXPECT().ContractTrie().Return(contractTrie, nil).AnyTimes()
	mockState.EXPECT().ContractNonce(gomock.Any()).Return(*f.nonce, nil).AnyTimes()
	mockState.EXPECT().ContractClassHash(gomock.Any()).Return(*f.classHash, nil).AnyTimes()
	mockState.EXPECT().ContractStorageTrie(gomock.Any()).Return(storageTrie, nil).AnyTimes()

	return rpc.New(mockReader, nil, nil, log.NewNopZapLogger()), mockCtrl.Finish
}

func benchmarkUnsortedFelts(start, count uint64) []felt.Felt {
	if count == 0 {
		return nil
	}

	values := make([]felt.Felt, count)
	for i := range values {
		offset := (uint64(i)*73 + 19) % count
		values[i] = *felt.NewFromUint64[felt.Felt](start + offset)
	}
	return values
}

func benchmarkStorageProofTries(
	b *testing.B,
	classKeys []felt.Felt,
	contractKeys []felt.Felt,
	storageKeys []felt.Felt,
) (core.TrieReader, core.TrieReader, core.TrieReader) {
	b.Helper()

	if !statetestutils.UseNewState() {
		classTrie := benchmarkDeprecatedTrie(b, []byte{0}, classKeys)
		contractTrie := benchmarkDeprecatedTrie(b, []byte{1}, contractKeys)
		storageTrie := benchmarkDeprecatedTrie(b, []byte{2}, storageKeys)
		return classTrie, contractTrie, storageTrie
	}

	newComm := felt.FromUint64[felt.StateRootHash](1)
	trieDB := trie2.NewTestNodeDatabase(memory.New(), trie2.PathScheme)
	oldComm := felt.FromUint64[felt.StateRootHash](0)
	benchmarkTrie(
		b,
		&trieDB,
		trieutils.NewClassTrieID(oldComm),
		newComm,
		classKeys,
		crypto.Poseidon,
	)
	benchmarkTrie(
		b,
		&trieDB,
		trieutils.NewContractTrieID(oldComm),
		newComm,
		contractKeys,
		crypto.Pedersen,
	)
	benchmarkTrie(
		b,
		&trieDB,
		trieutils.NewContractStorageTrieID(oldComm, felt.Address{}),
		newComm,
		storageKeys,
		crypto.Pedersen,
	)

	classTrie, err := trie2.New(
		trieutils.NewClassTrieID(newComm),
		251,
		crypto.Poseidon,
		&trieDB,
	)
	require.NoError(b, err)

	contractTrie, err := trie2.New(
		trieutils.NewContractTrieID(newComm),
		251,
		crypto.Pedersen,
		&trieDB,
	)
	require.NoError(b, err)

	storageTrie, err := trie2.New(
		trieutils.NewContractStorageTrieID(newComm, felt.Address{}),
		251,
		crypto.Pedersen,
		&trieDB,
	)
	require.NoError(b, err)

	return classTrie, contractTrie, storageTrie
}

func benchmarkDeprecatedTrie(b *testing.B, bucket []byte, keys []felt.Felt) *trie.Trie {
	b.Helper()

	memdb := memory.New()
	txn := memdb.NewIndexedBatch()
	tempTrie, err := trie.NewTriePedersen(txn, bucket, 251)
	require.NoError(b, err)
	for i := range keys {
		value := felt.NewFromUint64[felt.Felt](uint64(i + 1))
		_, err = tempTrie.Put(&keys[i], value)
		require.NoError(b, err)
	}
	require.NoError(b, tempTrie.Commit())

	return tempTrie
}

func benchmarkTrie(
	b *testing.B,
	trieDB *trie2.TestNodeDatabase,
	id trieutils.TrieID,
	commitment felt.StateRootHash,
	keys []felt.Felt,
	hashFn crypto.HashFn,
) {
	b.Helper()

	tr, err := trie2.New(id, 251, hashFn, trieDB)
	require.NoError(b, err)
	for i := range keys {
		value := felt.NewFromUint64[felt.Felt](uint64(i + 1))
		require.NoError(b, tr.Update(&keys[i], value))
	}
	_, nodes := tr.Commit()
	err = trieDB.Update((*felt.Felt)(&commitment), &felt.Zero, trienode.NewMergeNodeSet(nodes))
	require.NoError(b, err)
}
