package trie

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/store"
)

const testKeyLen = 3

var tests = [...]struct {
	key, val *big.Int
}{
	{big.NewInt(2) /* 0b010 */, big.NewInt(1)},
	{big.NewInt(3) /* 0b011 */, big.NewInt(1)},
	{big.NewInt(5) /* 0b101 */, big.NewInt(1)},
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Example() {
	var pairs = [...]struct {
		key, val *big.Int
	}{
		{big.NewInt(2) /* 0b010 */, big.NewInt(1)},
		{big.NewInt(5) /* 0b101 */, big.NewInt(1)},
	}

	// Provide the storage that the trie will use to persist data.
	db := store.New()

	// Initialise trie with storage and provide the key length (height of
	// the tree).
	t := New(db, 3)

	// Insert items into the trie.
	for _, pair := range pairs {
		fmt.Printf("put(key=%d, val=%d)\n", pair.key, pair.val)
		t.Put(pair.key, pair.val)
	}

	// Retrieve items from the trie.
	for _, pair := range pairs {
		val, _ := t.Get(pair.key)
		fmt.Printf("get(key=%d) = %d\n", pair.key, val)
	}

	// Remove items from the trie.
	for _, pair := range pairs {
		fmt.Printf("delete(key=%d)\n", pair.key)
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

func TestGet(t *testing.T) {
	db := store.New()
	trie := New(db, testKeyLen)
	for _, test := range tests {
		trie.Put(test.key, test.val)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("get(%#v) = %#v", test.key, test.val), func(t *testing.T) {
			got, _ := trie.Get(test.key)
			if got.Cmp(test.val) != 0 {
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

	t.Run("insert: t0.Commitment().Cmp(t1.Commitment()) == 0", func(t *testing.T) {
		if t0.Commitment().Cmp(t1.Commitment()) != 0 {
			t.Errorf("tries with the same values have diverging root hashes")
		}
	})

	t.Run("delete: t0.Commitment().Cmp(t1.Commitment()) == 0", func(t *testing.T) {
		t0.Delete(tests[1].key)
		t1.Delete(tests[1].key)
		if t0.Commitment().Cmp(t1.Commitment()) != 0 {
			t.Errorf("tries with the same values have diverging root hashes")
		}
	})

	t.Run("different: t0.Commitment().Cmp(t1.Commitment()) != 0", func(t *testing.T) {
		t0.Put(tests[1].key, tests[1].val)
		if t0.Commitment().Cmp(t1.Commitment()) == 0 {
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

	t.Run("oldTrie.Commitment().Cmp(newTrie.Commitment()) == 0", func(t *testing.T) {
		if oldTrie.Commitment().Cmp(newTrie.Commitment()) != 0 {
			t.Errorf("new trie produced a different commitment from the same store")
		}
	})

	randKey := tests[rand.Int()%len(tests)].key
	t.Run(
		fmt.Sprintf("oldTrie.Get(%#v) == newTrie.Get(%#v)", randKey, randKey),
		func(t *testing.T) {
			got, _ := oldTrie.Get(randKey)
			want, _ := newTrie.Get(randKey)
			if got.Cmp(want) != 0 {
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
			pre := prefix(reversed(test.key, testKeyLen), testKeyLen)
			got, ok := db.Get(pre)
			if !ok {
				t.Fatalf("failed to retrieve value with key %s from database", pre)
			}
			var n node
			if err := json.Unmarshal(got, &n); err != nil {
				t.Fatal("failed to unmarshal value from database")
			}
			if test.val.Cmp(n.Bottom) != 0 {
				t.Errorf("failed to put value %#v at key %#v", test.key, test.val)
			}
		})
	}
}

// TODO: Test for a valid commitment value for a given set of
// insertions.
