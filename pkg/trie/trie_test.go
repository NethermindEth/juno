package trie

import (
	"fmt"
	"math/big"
	"testing"

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

func TestPut(t *testing.T) {
	db := store.New()
	trie := New(db, testKeyLen)
	for _, test := range tests {
		t.Run(fmt.Sprintf("put(%#v, %#v)", test.key, test.val), func(t *testing.T) {
			trie.Put(test.key, test.val)
		})
	}
}
