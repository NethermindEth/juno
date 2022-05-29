package store

import (
	"bytes"
	"fmt"
	"testing"
)

var tests = []struct {
	key, val []byte
}{
	{[]byte{2}, []byte{1}},
	{[]byte{3}, []byte{1}},
	{[]byte{5}, []byte{1}},
}

func TestDelete(t *testing.T) {
	store := New()
	for _, test := range tests {
		store.Put(test.key, test.val)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("delete(%#v)", test.key), func(t *testing.T) {
			store.Delete(test.key)
			_, ok := store.Get(test.key)
			if ok {
				t.Errorf("key %#v not successfully removed from storage", test.key)
			}
		})
	}
}

func TestGet(t *testing.T) {
	store := New()
	for _, test := range tests {
		store.Put(test.key, test.val)
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("get(%#v) = %#v", test.key, test.val), func(t *testing.T) {
			got, _ := store.Get(test.key)
			if !bytes.Equal(got, test.val) {
				t.Errorf("get(%#v) = %#v, want %#v", test.key, got, test.val)
			}
		})
	}
}

func TestPut(t *testing.T) {
	store := New()
	for _, test := range tests {
		t.Run(fmt.Sprintf("put(%#v, %#v)", test.key, test.val), func(t *testing.T) {
			store.Put(test.key, test.val)
		})
	}
}
