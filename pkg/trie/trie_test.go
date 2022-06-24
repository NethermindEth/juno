package trie

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/NethermindEth/juno/pkg/common"
	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/store"
)

const testHeight = 3

var tests = [...]struct {
	key, val types.Felt
}{
	{types.BigToFelt(big.NewInt(2)) /* 0b010 */, types.BigToFelt(big.NewInt(1))},
	{types.BigToFelt(big.NewInt(3)) /* 0b011 */, types.BigToFelt(big.NewInt(1))},
	{types.BigToFelt(big.NewInt(5)) /* 0b101 */, types.BigToFelt(big.NewInt(1))},
	{types.BigToFelt(big.NewInt(7)) /* 0b111 */, types.BigToFelt(big.NewInt(0))},
}

func init() {
	rand.Seed(0)
}

func TestExample(t *testing.T) {
	pairs := [...]struct {
		key, val types.Felt
	}{
		{types.BigToFelt(big.NewInt(2)) /* 0b010 */, types.BigToFelt(big.NewInt(1))},
		{types.BigToFelt(big.NewInt(5)) /* 0b101 */, types.BigToFelt(big.NewInt(1))},
	}

	// Provide the storage that the trie will use to persist data.
	db := store.New()

	// Initialise trie with storage and provide the key length (height of
	// the tree).
	trie, _ := New(db, EmptyNode.Hash(), testHeight)

	// Insert items into the trie.
	for _, pair := range pairs {
		t.Logf("put(key=%d, val=%d)\n", pair.key, pair.val)
		err := trie.Put(&pair.key, &pair.val)
		if err != nil {
			return
		}
	}

	// Retrieve items from the trie.
	for _, pair := range pairs {
		val, _ := trie.Get(&pair.key)
		t.Logf("get(key=%d) = %d\n", pair.key, val)
	}

	// Remove items from the trie.
	for _, pair := range pairs {
		t.Logf("delete(key=%d)\n", pair.key)
		trie.Del(&pair.key)
	}

	// Output:
	// put(key=2, val=1)
	// put(key=5, val=1)
	// get(key=2) = 1
	// get(key=5) = 1
	// delete(key=2)
	// delete(key=5)
}

func TestDelete(t *testing.T) {
	db := store.New()
	trie, _ := New(db, EmptyNode.Hash(), testHeight)
	for _, test := range tests {
		trie.Put(&test.key, &test.val)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("delete(%s)", test.key.Hex()), func(t *testing.T) {
			if err := trie.Del(&test.key); err != nil {
				t.Fatal(err)
			}
			val, err := trie.Get(&test.key)
			if err != nil {
				t.Fatal(err)
			}
			if val.Cmp(&types.Felt0) != 0 {
				t.Errorf("key %s not successfully removed from storage. returned %s", test.key.Hex(), val.Hex())
			}
		})
	}
}

// TestEmptyTrie asserts that the commitment of an empty trie is zero.
func TestEmptyTrie(t *testing.T) {
	trie, _ := New(store.New(), EmptyNode.Hash(), testHeight)
	if trie.RootHash().Cmp(&types.Felt0) != 0 {
		t.Error("trie.RootHash() != 0 for empty trie")
	}
}

func TestGet(t *testing.T) {
	db := store.New()
	trie, _ := New(db, EmptyNode.Hash(), testHeight)
	for _, test := range tests {
		err := trie.Put(&test.key, &test.val)
		if err != nil {
			t.Fatalf("error inserting key (%v) on the trie: %v", test.key.Hex(), err)
		}
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("get(%#v) = %#v", test.key.Hex(), test.val.Hex()), func(t *testing.T) {
			got, err := trie.Get(&test.key)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Errorf("got = nil, want = %x", test.val.Hex())
			}
			if test.val.Big().Cmp(new(big.Int)) != 0 && got.Big().Cmp(test.val.Big()) != 0 {
				t.Errorf("get(%#v) = %#v, want %#v", test.key.Hex(), got.Hex(), test.val.Hex())
			}
		})
	}
}

// TestInvariant checks that the root hash is independent of the
// insertion and deletion order.
func TestInvariant(t *testing.T) {
	t0, _ := New(store.New(), EmptyNode.Hash(), testHeight)
	t1, _ := New(store.New(), EmptyNode.Hash(), testHeight)

	for i := 0; i < len(tests); i++ {
		t0.Put(&tests[i].key, &tests[i].val)
	}

	// Insertion in reverse order.
	for i := len(tests) - 1; i >= 0; i-- {
		t1.Put(&tests[i].key, &tests[i].val)
	}

	t.Run("insert: t0.RootHash().Cmp(t1.RootHash()) == 0", func(t *testing.T) {
		if t0.RootHash().Cmp(t1.RootHash()) != 0 {
			t.Errorf("tries with the same values have diverging root hashes")
		}
	})

	t.Run("delete: t0.RootHash().Cmp(t1.RootHash()) == 0", func(t *testing.T) {
		t0.Del(&tests[1].key)
		t1.Del(&tests[1].key)
		if t0.RootHash().Cmp(t1.RootHash()) != 0 {
			t.Errorf("tries with the same values have diverging root hashes")
		}
	})

	t.Run("different: t0.RootHash().Cmp(t1.RootHash()) != 0", func(t *testing.T) {
		t0.Put(&tests[1].key, &tests[1].val)
		if t0.RootHash().Cmp(t1.RootHash()) == 0 {
			t.Errorf("tries with different values have the same root hashes")
		}
	})
}

// TestRebuild tests that the trie can be reconstructed from storage.
func TestRebuild(t *testing.T) {
	db := store.New()
	oldTrie, _ := New(db, EmptyNode.Hash(), testHeight)

	for _, test := range tests {
		oldTrie.Put(&test.key, &test.val)
	}

	// New trie using the same storage.
	newTrie, _ := New(db, oldTrie.RootHash(), testHeight)

	t.Run("oldTrie.RootHash().Cmp(newTrie.RootHash()) == 0", func(t *testing.T) {
		if oldTrie.RootHash().Cmp(newTrie.RootHash()) != 0 {
			t.Errorf("new trie produced a different commitment from the same store")
		}
	})

	randKey := tests[rand.Int()%len(tests)].key
	t.Run(
		fmt.Sprintf("oldTrie.Get(%#v) == newTrie.Get(%#v)", randKey, randKey),
		func(t *testing.T) {
			got, _ := oldTrie.Get(&randKey)
			want, _ := newTrie.Get(&randKey)
			if got.Cmp(want) != 0 {
				t.Errorf("oldTrie.Get(%#v) = %#v != newTrie.Get(%#v) = %#v", randKey, got, randKey, want)
			}
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
			"735596016a37ee972c42adef6a3cf628c19bb3794369c65d2c82ba034aecf2c": {
				{"5", "64"},
				{
					"2f50710449a06a9fa789b3c029a63bd0b1f722f46505828a9f815cf91b31d8",
					"2a222e62eabe91abdb6838fa8b267ffe81a6eb575f61e96ec9aa4460c0925a2",
				},
			},
			"20cfa74ee3564b4cd5435cdace0f9c4d43b939620e4a0bb5076105df0a626c6": {
				{"5", "22b"},
				{
					"5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5",
					"7e5",
				},
				{
					"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620300",
					"4e7e989d58a17cd279eca440c5eaa829efb6f9967aaad89022acbe644c39b36",
				},
				{
					"313ad57fdf765addc71329abf8d74ac2bce6d46da8c2b9b82255a5076620301",
					"453ae0c9610197b18b13645c44d3d0a407083d96562e8752aab3fab616cecb0",
				},
				{
					"6cf6c2f36d36b08e591e4489e92ca882bb67b9c39a3afccf011972a8de467f0",
					"7ab344d88124307c07b56f6c59c12f4543e9c96398727854a322dea82c73240",
				},
			},
			"6ee3440b08a9c805305449ec7f7003f27e9f7e287b83610952ec36bdc5a6bae": {
				{
					"1e2cd4b3588e8f6f9c4e89fb0e293bf92018c96d7a93ee367d29a284223b6ff",
					"71d1e9d188c784a0bde95c1d508877a0d93e9102b37213d1e13f3ebc54a7751",
				},
				{
					"5f750dc13ed239fa6fc43ff6e10ae9125a33bd05ec034fc3bb4dd168df3505f",
					"7e5",
				},
				{
					"48cba68d4e86764105adcdcf641ab67b581a55a4f367203647549c8bf1feea2",
					"362d24a3b030998ac75e838955dfee19ec5b6eceb235b9bfbeccf51b6304d0b",
				},
				{
					"449908c349e90f81ab13042b1e49dc251eb6e3e51092d9a40f86859f7f415b0",
					"6cb6104279e754967a721b52bcf5be525fdc11fa6db6ef5c3a4db832acf7804",
				},
				{
					"5bdaf1d47b176bfcd1114809af85a46b9c4376e87e361d86536f0288a284b65",
					"28dff6722aa73281b2cf84cac09950b71fa90512db294d2042119abdd9f4b87",
				},
				{
					"5bdaf1d47b176bfcd1114809af85a46b9c4376e87e361d86536f0288a284b66",
					"57a8f8a019ccab5bfc6ff86c96b1392257abb8d5d110c01d326b94247af161c",
				},
			},
			"31c887d82502ceb218c06ebb46198da3f7b92864a8223746bc836dda3e34b52": {
				{
					"5f750dc13ed239fa6fc43ff6e10ae9125a33bd05ec034fc3bb4dd168df3505f",
					"7c7",
				},
				{
					"df28e613c065616a2e79ca72f9c1908e17b8c913972a9993da77588dc9cae9",
					"1432126ac23c7028200e443169c2286f99cdb5a7bf22e607bcd724efa059040",
				},
			},
			"31c9cdb9b00cb35cf31c05855c0ec3ecf6f7952a1ce6e3c53c3455fcd75a280": {
				{"5", "65"},
				{
					"5aee31408163292105d875070f98cb48275b8c87e80380b78d30647e05854d5",
					"7c7",
				},
				{
					"cfc2e2866fd08bfb4ac73b70e0c136e326ae18fc797a2c090c8811c695577e",
					"5f1dd5a5aef88e0498eeca4e7b2ea0fa7110608c11531278742f0b5499af4b3",
				},
				{
					"5fac6815fddf6af1ca5e592359862ede14f171e1544fd9e792288164097c35d",
					"299e2f4b5a873e95e65eb03d31e532ea2cde43b498b50cd3161145db5542a5",
				},
				{
					"5fac6815fddf6af1ca5e592359862ede14f171e1544fd9e792288164097c35e",
					"3d6897cf23da3bf4fd35cc7a43ccaf7c5eaf8f7c5b9031ac9b09a929204175f",
				},
			},
		}

		want         = types.BytesToFelt(common.FromHex("021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6"))
		contractHash = types.BytesToFelt(common.FromHex("10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8"))
	)

	height := 251
	state, _ := New(store.New(), EmptyNode.Hash(), height)
	for addr, diff := range addresses {
		storage, _ := New(store.New(), EmptyNode.Hash(), height)
		for _, slot := range diff {
			key := types.BytesToFelt(common.FromHex(slot.key))
			val := types.BytesToFelt(common.FromHex(slot.val))
			storage.Put(&key, &val)
		}

		key := types.BytesToFelt(common.FromHex(addr))
		val := types.BigToFelt(
			pedersen.Digest(
				pedersen.Digest(
					pedersen.Digest(contractHash.Big(), storage.RootHash().Big()),
					types.Felt0.Big(),
				),
				types.Felt0.Big(),
			),
		)

		state.Put(&key, &val)
	}

	got := state.RootHash()
	if want.Cmp(got) != 0 {
		t.Errorf("state.RootHash() = %x, want = %x", got, want)
	}
}
