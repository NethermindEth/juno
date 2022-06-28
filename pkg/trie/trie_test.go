package trie

import (
	"encoding/json"
	"fmt"
	//"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/crypto/pedersen"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/store"
)

const testKeyLen = 3

var tests = [...]struct {
	key, val *felt.Felt
}{
	{new(felt.Felt).Set(felt.Felt2) /* 0b010 */, new(felt.Felt).Set(felt.Felt1)},
	{new(felt.Felt).Set(felt.Felt3) /* 0b011 */, new(felt.Felt).Set(felt.Felt1)},
	{new(felt.Felt).SetUint64(5) /* 0b101 */, new(felt.Felt).Set(felt.Felt1)},
	{new(felt.Felt).SetUint64(7) /* 0b111 */, new(felt.Felt).Set(felt.Felt0)},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Example() {
	pairs := [...]struct {
		key, val *felt.Felt
	}{
		{new(felt.Felt).Set(felt.Felt2) /* 0b010 */, new(felt.Felt).Set(felt.Felt1)},
		{new(felt.Felt).SetUint64(5) /* 0b101 */, new(felt.Felt).Set(felt.Felt1)},
	}

	// Provide the storage that the trie will use to persist data.
	db := store.New()

	// Initialise trie with storage and provide the key length (height of
	// the tree).
	t := New(db, 3)

	// Insert items into the trie.
	for _, pair := range pairs {
		fmt.Printf("put(key=%d, val=%d)\n", pair.key.Uint64(), pair.val.Uint64())
		t.Put(pair.key, pair.val)
	}

	// Retrieve items from the trie.
	for _, pair := range pairs {
		val, _ := t.Get(pair.key)
		fmt.Printf("get(key=%d) = %d\n", pair.key.Uint64(), val.Uint64())
	}

	// Remove items from the trie.
	for _, pair := range pairs {
		fmt.Printf("delete(key=%d)\n", pair.key.Uint64())
		t.Delete(pair.key)
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
	trie := New(db, testKeyLen)
	for _, test := range tests {
		trie.Put(test.key, test.val)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("delete(%#v)", test.key), func(t *testing.T) {
			trie.Delete(test.key)
			_, ok := trie.Get(test.key)
			if ok {
				t.Errorf("key %#v not successfully removed from storage", test.key)
			}
		})
	}
}

// TestEmptyTrie asserts that the commitment of an empty trie is zero.
func TestEmptyTrie(t *testing.T) {
	trie := New(store.New(), testKeyLen)
	if trie.Commitment().CmpCompat(felt.Felt0) != 0 {
		t.Error("trie.Commitment() != 0 for empty trie")
	}
}

func TestGet(t *testing.T) {
	db := store.New()
	trie := New(db, testKeyLen)
	for _, test := range tests {
		trie.Put(test.key, test.val)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("get(%#v) = %#v", test.key, test.val), func(t *testing.T) {
			got, _ := trie.Get(test.key)
			if test.val.CmpCompat(new(felt.Felt)) != 0 && got.CmpCompat(test.val) != 0 {
				t.Errorf("get(%#v) = %#v, want %#v", test.key, got, test.val)
			}
		})
	}
}

// TestInvariant checks that the root hash is independent of the
// insertion and deletion order.
func TestInvariant(t *testing.T) {
	t0 := New(store.New(), testKeyLen)
	t1 := New(store.New(), testKeyLen)

	for _, test := range tests {
		t0.Put(test.key, test.val)
	}

	// Insertion in reverse order.
	for i := len(tests) - 1; i >= 0; i-- {
		t1.Put(tests[i].key, tests[i].val)
	}

	t.Run("insert: t0.Commitment().CmpCompat(t1.Commitment()) == 0", func(t *testing.T) {
		if t0.Commitment().CmpCompat(t1.Commitment()) != 0 {
			t.Errorf("tries with the same values have diverging root hashes")
		}
	})

	t.Run("delete: t0.Commitment().CmpCompat(t1.Commitment()) == 0", func(t *testing.T) {
		t0.Delete(tests[1].key)
		t1.Delete(tests[1].key)
		if t0.Commitment().CmpCompat(t1.Commitment()) != 0 {
			t.Errorf("tries with the same values have diverging root hashes")
		}
	})

	t.Run("different: t0.Commitment().CmpCompat(t1.Commitment()) != 0", func(t *testing.T) {
		t0.Put(tests[1].key, tests[1].val)
		if t0.Commitment().CmpCompat(t1.Commitment()) == 0 {
			t.Errorf("tries with different values have the same root hashes")
		}
	})
}

// TestRebuild tests that the trie can be reconstructed from storage.
func TestRebuild(t *testing.T) {
	db := store.New()
	oldTrie := New(db, testKeyLen)

	for _, test := range tests {
		oldTrie.Put(test.key, test.val)
	}

	// New trie using the same storage.
	newTrie := New(db, testKeyLen)

	t.Run("oldTrie.Commitment().CmpCompat(newTrie.Commitment()) == 0", func(t *testing.T) {
		if oldTrie.Commitment().CmpCompat(newTrie.Commitment()) != 0 {
			t.Errorf("new trie produced a different commitment from the same store")
		}
	})

	randKey := tests[rand.Int()%len(tests)].key
	t.Run(
		fmt.Sprintf("oldTrie.Get(%#v) == newTrie.Get(%#v)", randKey, randKey),
		func(t *testing.T) {
			got, _ := oldTrie.Get(randKey)
			want, _ := newTrie.Get(randKey)
			if got.CmpCompat(want) != 0 {
				t.Errorf("oldTrie.Get(%#v) = %#v != newTrie.Get(%#v) = %#v", randKey, got, randKey, want)
			}
		})
}

func TestPut(t *testing.T) {
	db := store.New()
	trie := New(db, testKeyLen)
	for _, test := range tests {
		t.Run(fmt.Sprintf("put(%#v, %#v)", test.key, test.val), func(t *testing.T) {
			trie.Put(test.key, test.val)
			pre := Prefix(Reversed(test.key, testKeyLen), testKeyLen)
			got, ok := db.Get(pre)
			if !ok {
				// A key with a value 0 is deleted.
				if test.val.CmpCompat(new(felt.Felt)) == 0 {
					t.Skip()
				}
				t.Fatalf("failed to retrieve value with key %s from database", pre)
			}
			var n Node
			if err := json.Unmarshal(got, &n); err != nil {
				t.Fatal("failed to unmarshal value from database")
			}
			if test.val.CmpCompat(n.Bottom) != 0 {
				t.Errorf("failed to put value %#v at key %#v", test.key, test.val)
			}
		})
	}
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

		want         = new(felt.Felt).SetHex("021870ba80540e7831fb21c591ee93481f5ae1bb71ff85a86ddd465be4eddee6")
		contractHash = new(felt.Felt).SetHex("10455c752b86932ce552f2b0fe81a880746649b9aee7e0d842bf3f52378f9f8")
	)

	height := 251
	state := New(store.New(), height)
	for addr, diff := range addresses {
		storage := New(store.New(), height)
		for _, slot := range diff {
			key := new(felt.Felt).SetHex(slot.key)
			val := new(felt.Felt).SetHex(slot.val)
			storage.Put(key, val)
		}

		key := new(felt.Felt).SetHex(addr)
		val := pedersen.Digest(pedersen.Digest(pedersen.Digest(contractHash, storage.Commitment()), new(felt.Felt)), new(felt.Felt))
		state.Put(key, val)
	}

	got := state.Commitment()
	if want.CmpCompat(got) != 0 {
		t.Errorf("state.Commitment() = %x, want = %x", got, want)
	}
}
