package trie

import (
	"fmt"
	"testing"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/bits-and-blooms/bitset"
	"github.com/dgraph-io/badger/v3"
	"github.com/stretchr/testify/assert"
)

// Todo: Refactor:
//   - Test names should not have "_"
//   - Table test are being used incorrectly: they should be separated into subsets, see node_test.go
//   - Functions such as Path and FindCommonKey don't need to be public. Thus,
//     they don't need to be tested explicitly.
//   - There are warning which ignore returned errors, returned errors should not be ignored.
//   - Add more test cases with different heights
//   - Add more complicated Put and Delete scenarios
func TestPathFromKey(t *testing.T) {
	trie := NewTrie(nil, 251, nil)
	key, _ := new(felt.Felt).SetRandom()
	path := trie.FeltToBitSet(key)
	keyRegular := key.ToRegular()
	for bit := 0; bit < felt.Bits; bit++ {
		if keyRegular.Bit(uint64(bit)) > 0 != path.Test(uint(bit)) {
			t.Error("TestPathFromKey failed")
			break
		}
	}

	// Make sure they dont share the same underlying memory
	key.Halve()
	keyRegular = key.ToRegular()
	for bit := 0; bit < felt.Bits; bit++ {
		if keyRegular.Bit(uint64(bit)) > 0 != path.Test(uint(bit)) {
			return
		}
	}
	t.Error("TestPathFromKey failed")
}

func TestFindCommonPath(t *testing.T) {
	tests := [...]struct {
		path1  *bitset.BitSet
		path2  *bitset.BitSet
		common *bitset.BitSet
		subset bool
	}{
		{
			path1:  bitset.New(16).Set(4).Set(3),
			path2:  bitset.New(16).Set(4),
			common: bitset.New(12).Set(0),
			subset: false,
		},
		{
			path1:  bitset.New(2).Set(1),
			path2:  bitset.New(2),
			common: bitset.New(0),
			subset: false,
		},
		{
			path1:  bitset.New(2).Set(1),
			path2:  bitset.New(2).Set(1),
			common: bitset.New(2).Set(1),
			subset: true,
		},
		{
			path1:  bitset.New(10),
			path2:  bitset.New(8),
			common: bitset.New(8),
			subset: true,
		},
	}

	for _, test := range tests {
		if common, subset := FindCommonKey(test.path1, test.path2); !test.common.Equal(common) || subset != test.subset {
			t.Errorf("TestFindCommonPath: Expected %s (%d) Got %s (%d)", test.common.DumpAsBits(),
				test.common.Len(), common.DumpAsBits(), common.Len())
		}
	}
}

func TestTriePut(t *testing.T) {
	tests := [...]struct {
		key   *felt.Felt
		value *felt.Felt
		root  *bitset.BitSet
	}{
		{
			key:   new(felt.Felt).SetUint64(2),
			value: new(felt.Felt).SetUint64(2),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(1),
			value: new(felt.Felt).SetUint64(1),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(3),
			value: new(felt.Felt).SetUint64(3),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(3),
			value: new(felt.Felt).SetUint64(4),
			root:  nil,
		},
		{
			key:   new(felt.Felt).SetUint64(0),
			value: new(felt.Felt).SetUint64(5),
			root:  nil,
		},
	}
	RunOnTempTrie(251, func(trie *Trie) error {
		for idx, test := range tests {
			if _, err := trie.Put(test.key, test.value); err != nil {
				t.Errorf("TestTriePut: Put() failed at test #%d", idx)
			}
			if value, err := trie.Get(test.key); err != nil || !value.Equal(test.value) {
				t.Errorf("TestTriePut: Get() failed at test #%d", idx)
			}
			if test.root != nil && !test.root.Equal(trie.rootKey) {
				t.Errorf("TestTriePut: Unexpected rootKey at test #%d", idx)
			}
		}

		return nil
	})
}

func TestPath(t *testing.T) {
	tests := [...]struct {
		parent *bitset.BitSet
		child  *bitset.BitSet
		want   *bitset.BitSet
	}{
		{
			parent: bitset.New(0),
			child:  bitset.New(251).Set(250).Set(249),
			want:   bitset.New(250).Set(249),
		},
		{
			parent: bitset.New(0),
			child:  bitset.New(251).Set(249),
			want:   bitset.New(250).Set(249),
		},
		{
			parent: bitset.New(1).Set(0),
			child:  bitset.New(251).Set(250).Set(249),
			want:   bitset.New(249),
		},
	}

	for idx, test := range tests {
		if got := Path(test.child, test.parent); !got.Equal(test.want) {
			t.Error("TestPath failing #", idx)
		}
	}
}

func TestPathOnTrie(t *testing.T) {
	RunOnTempTrie(251, func(trie *Trie) error {
		// build example trie from https://docs.starknet.io/documentation/develop/State/starknet-state/
		// and check paths
		var two felt.Felt
		two.SetUint64(2)
		var five felt.Felt
		five.SetUint64(5)
		var one felt.Felt
		one.SetUint64(1)
		trie.Put(&two, &one)
		assert.Equal(t, true, Path(trie.rootKey, nil).Equal(trie.FeltToBitSet(&two)))

		trie.Put(&five, &one)
		expectedRoot, _ := FindCommonKey(trie.FeltToBitSet(&two), trie.FeltToBitSet(&five))
		assert.Equal(t, true, Path(trie.rootKey, nil).Equal(expectedRoot))

		rootNode, err := trie.storage.Get(trie.rootKey)
		if err != nil {
			t.Error()
		}

		assert.Equal(t, true, rootNode.left != nil && rootNode.right != nil)

		expectedLeftPath := bitset.New(2).Set(1)
		expectedRightPath := bitset.New(2).Set(0)
		assert.Equal(t, true, Path(rootNode.left, trie.rootKey).Equal(expectedLeftPath))
		assert.Equal(t, true, Path(rootNode.right, trie.rootKey).Equal(expectedRightPath))
		return nil
	})
}

func TestGetPath_ZeroRoot(t *testing.T) {
	RunOnTempTrie(251, func(trie *Trie) error {
		var zero felt.Felt
		msbOne, _ := new(felt.Felt).SetString("0x400000000000000000000000000000000000000000000000000000000000000")
		var one felt.Felt
		one.SetUint64(1)
		trie.Put(&zero, &one)
		trie.Put(msbOne, &one)

		zeroPath := bitset.New(0)
		assert.Equal(t, true, trie.rootKey.Equal(zeroPath))
		assert.Equal(t, true, Path(trie.rootKey, nil).Equal(zeroPath))
		return nil
	})
}

// TestState tests whether the trie produces the same state root as in
// Block 0 of the StarkNet protocol mainnet.
func TestState(t *testing.T) {
	// See https://alpha-mainnet.starknet.io/feeder_gateway/get_state_update?blockNumber=0.
	type (
		diff  struct{ key, val string }
		diffs map[string][]diff
	)

	var (
		addresses = diffs{
			"0x735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c": {
				{"0x5", "0x64"},
				{
					"0x2f50710449a06a9fa789b3c029a63bd0b1f722f46505828a9f815cf91b31d8",
					"0x2a222e62eabe91abdb6838fa8b267ffe81a6eb575f61e96ec9aa4460c0925a2",
				},
			},
			"0x20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": {
				{"0x5", "0x22b"},
				{
					"0x5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5",
					"0x7e5",
				},
				{
					"0x313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300",
					"0x4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
				},
				{
					"0x313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620301",
					"0x453ae0c9610197b18b13645c44d3d0a407083d96562e8752aab3fab616cecb0",
				},
				{
					"0x6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0",
					"0x7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
				},
			},
			"0x6ee3440b08a9c805305449ec7f7003f27e9f7e287b83610952ec36bdc5a6bae": {
				{
					"0x1e2cd4b3588e8f6f9c4e89fb0e293bf92018c96d7a93ee367d29a284223b6ff",
					"0x71d1e9d188c784a0bde95c1d508877a0d93e9102b37213d1e13f3ebc54a7751",
				},
				{
					"0x5f750dc13ed239fa6fc43ff6e10ae9125a33bd05ec034fc3bb4dd168df3505f",
					"0x7e5",
				},
				{
					"0x48cba68d4e86764105adcdcf641ab67b581a55a4f367203647549c8bf1feea2",
					"0x362d24a3b030998ac75e838955dfee19ec5b6eceb235b9bfbeccf51b6304d0b",
				},
				{
					"0x449908c349e90f81ab13042b1e49dc251eb6e3e51092d9a40f86859f7f415b0",
					"0x6cb6104279e754967a721b52bcf5be525fdc11fa6db6ef5c3a4db832acf7804",
				},
				{
					"0x5bdaf1d47b176bfcd1114809af85a46b9c4376e87e361d86536f0288a284b65",
					"0x28dff6722aa73281b2cf84cac09950b71fa90512db294d2042119abdd9f4b87",
				},
				{
					"0x5bdaf1d47b176bfcd1114809af85a46b9c4376e87e361d86536f0288a284b66",
					"0x57a8f8a019ccab5bfc6ff86c96b1392257abb8d5d110c01d326b94247af161c",
				},
			},
			"0x31c887d82502ceb218c06ebb46198da3f7b92864a8223746bc836dda3e34b52": {
				{
					"0x5f750dc13ed239fa6fc43ff6e10ae9125a33bd05ec034fc3bb4dd168df3505f",
					"0x7c7",
				},
				{
					"0xdf28e613c065616a2e79ca72f9c1908e17b8c913972a9993da77588dc9cae9",
					"0x1432126ac23c7028200e443169c2286f99cdb5a7bf22e607bcd724efa059040",
				},
			},
			"0x31c9cdb9b00cb35cf31c05855c0ec3ecf6f7952a1ce6e3c53c3455fcd75a280": {
				{"0x5", "0x65"},
				{
					"0x5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5",
					"0x7c7",
				},
				{
					"0xcfc2e2866fd08bfb4ac73b70e0c136e326ae18fc797a2c090c8811c695577e",
					"0x5f1dd5a5aef88e0498eeca4e7b2ea0fa7110608c11531278742f0b5499af4b3",
				},
				{
					"0x5fac6815fddf6af1ca5e592359862ede14f171e1544fd9e792288164097c35d",
					"0x299e2f4b5a873e95e65eb03d31e532ea2cde43b498b50cd3161145db5542a5",
				},
				{
					"0x5fac6815fddf6af1ca5e592359862ede14f171e1544fd9e792288164097c35e",
					"0x3d6897cf23da3bf4fd35cc7a43ccaf7c5eaf8f7c5b9031ac9b09a929204175f",
				},
			},
		}

		want, _         = new(felt.Felt).SetString("0x021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6")
		contractHash, _ = new(felt.Felt).SetString("0x10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	)

	RunOnTempTrie(251, func(state *Trie) error {
		for addr, dif := range addresses {
			RunOnTempTrie(251, func(contractState *Trie) error {
				for _, slot := range dif {
					key, _ := new(felt.Felt).SetString(slot.key)
					val, _ := new(felt.Felt).SetString(slot.val)
					if _, err := contractState.Put(key, val); err != nil {
						t.Fatal(err)
					}
				}
				/*
				   735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c  :  15c52969f4ae2ad48bf324e21b8c06ce8abcbc492263072a8de9c7f0bfa3c81
				   20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6  :  4532b9a656bd6074c2ddb1b884fb976eb055cd4d37e093448ce3f223864ccc4
				   6ee3440b08a9c805305449ec7f7003f27e9f7e287b83610952ec36bdc5a6bae  :  51c6b823cbf53c47ab7b34cddf1d9c0286fbb9d72ab29f2b577da0308cb1a07
				   31c887d82502ceb218c06ebb46198da3f7b92864a8223746bc836dda3e34b52  :  2eb33f71cbf096ea6b3a55ba19fb31efc31184caca6482bc89c7708c2cbb420
				   31c9cdb9b00cb35cf31c05855c0ec3ecf6f7952a1ce6e3c53c3455fcd75a280  :  6fe0662f4be66647b4508a53a08e13e7d1ffb2b19e93fa9dc991153f3a447d
				*/

				key, _ := new(felt.Felt).SetString(addr)
				contractRoot, _ := contractState.Root()
				fmt.Println(addr, " : ", contractRoot.Text(16))

				val := crypto.Pedersen(contractHash, contractRoot)
				val = crypto.Pedersen(val, new(felt.Felt))
				val = crypto.Pedersen(val, new(felt.Felt))

				if _, err := state.Put(key, val); err != nil {
					t.Fatal(err)
				}

				return nil
			})
		}

		got, _ := state.Root()
		if !want.Equal(got) {
			t.Errorf("state.RootHash() = %s, want = %s", got.Text(16), want.Text(16))
		}

		return nil
	})
}

func TestPutZero(t *testing.T) {
	db, _ := db.NewInMemoryDb()
	defer db.Close()

	txn := db.NewTransaction(true)
	defer txn.Discard()

	trieTxn := NewTrieTxn(txn, nil)
	trie := NewTrie(trieTxn, 251, nil)
	emptyRoot, err := trie.Root()
	if err != nil {
		t.Error(err)
	}

	roots := []*felt.Felt{}
	keys := []*felt.Felt{}
	// put random 64 keys and record roots
	for i := 0; i < 64; i++ {
		key, err := new(felt.Felt).SetRandom()
		if err != nil {
			t.Error(err)
		}
		value, err := new(felt.Felt).SetRandom()
		if err != nil {
			t.Error(err)
		}

		if _, err = trie.Put(key, value); err != nil {
			t.Error(err)
		}

		keys = append(keys, key)
		root, err := trie.Root()
		if err != nil {
			t.Error(err)
		}

		roots = append(roots, root)
	}

	key, err := new(felt.Felt).SetRandom()
	if err != nil {
		t.Error(err)
	}
	// adding a zero value should not change Trie
	if _, err = trie.Put(key, new(felt.Felt)); err != nil {
		t.Error(err)
	}
	root, err := trie.Root()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, true, root.Equal(roots[len(roots)-1]))

	// put zero in reverse order and check roots still match
	for i := 0; i < 64; i++ {
		root := roots[len(roots)-1-i]
		actual, err := trie.Root()
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, true, actual.Equal(root))

		key := keys[len(keys)-1-i]
		trie.Put(key, new(felt.Felt))
	}

	actualEmptyRoot, err := trie.Root()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, true, actualEmptyRoot.Equal(emptyRoot))

	it := txn.Impl().(*badger.Txn).NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()
	it.Rewind()
	assert.Equal(t, false, it.Valid()) // storage should be empty
}

func TestOldData(t *testing.T) {
	RunOnTempTrie(251, func(trie *Trie) error {
		key := new(felt.Felt).SetUint64(12)
		old := new(felt.Felt)

		was, err := trie.Put(key, old)
		assert.NoError(t, err)
		assert.Nil(t, was) // no change

		was, err = trie.Put(key, new(felt.Felt).SetUint64(1))
		assert.NoError(t, err)
		assert.Equal(t, old, was)

		old.SetUint64(1)

		was, err = trie.Put(key, new(felt.Felt).SetUint64(2))
		assert.NoError(t, err)
		assert.Equal(t, old, was)

		old.SetUint64(2)

		was, err = trie.Put(key, new(felt.Felt))
		assert.NoError(t, err)
		assert.Equal(t, old, was)

		was, err = trie.Put(key, new(felt.Felt))
		assert.NoError(t, err)
		assert.Nil(t, was) // no change

		return nil
	})
}
