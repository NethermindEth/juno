package lru

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNewCache(t *testing.T) {
	tc := []int{1, 5, 10, 100, 500, 1000, 5000, 10000}

	t.Run("cannot create lru cache with cap=0", func(t *testing.T) {
		c := NewCache(0)
		if c != nil {
			t.Fatalf("expected c to be nil but go \"%v\"", c)
		}
	})

	for _, i := range tc {
		t.Run(fmt.Sprintf("create cache with cap=%d", i), func(t *testing.T) {
			c := NewCache(i)
			if c == nil {
				t.Error("did not expect cache to be nil")
			}
			if c.Cap() != i || c.Len() != 0 {
				t.Fatalf("expected cap: \"%d\" and len: \"%d\" but got cap: \"%d\" and len: \"%d\"", i, 0, c.Cap(), c.Len())
			}
		})
	}
}

func TestCache_Put(t *testing.T) {
	t.Run("single key value pair in empty cache", func(t *testing.T) {
		wantCap, wantLen := 5, 1
		k, v := keyValuePair(1)
		c := NewCache(wantCap)
		c.Put(k, v)
		checkCache(t, c, wantCap, wantLen, v, v)
	})
	t.Run("multiple distinct key value pairs in empty cache", func(t *testing.T) {
		wantCap := 5
		var b []byte
		c := NewCache(wantCap)

		for i := 0; i < wantCap; i++ {
			wantLen := i + 1
			k, v := keyValuePair(i)
			if i == 0 {
				b = v
			}
			c.Put(k, v)
			checkCache(t, c, wantCap, wantLen, v, b)
		}
	})
	t.Run("multiple distinct key value pairs in full cache", func(t *testing.T) {
		wantCap := 5
		c := createFullCache(wantCap)

		for i := 0; i < wantCap; i++ {
			j := i + wantCap
			k, v := keyValuePair(j)
			c.Put(k, v)

			_, wantV := keyValuePair(i + 1)
			deletedK, _ := keyValuePair(i)

			checkCache(t, c, wantCap, wantCap, v, wantV)
			if v := c.Get(deletedK); v != nil {
				t.Errorf("expected value to be nil, got %s", v)
			}
		}
	})
	t.Run("node with same key but different value at the front of the full cache", func(t *testing.T) {
		wantCap := 5
		c := createFullCache(wantCap)

		k, _ := keyValuePair(4)
		_, wantV := keyValuePair(5)
		_, wantB := keyValuePair(0)
		c.Put(k, wantV)

		checkCache(t, c, wantCap, wantCap, wantV, wantB)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}

	})
	t.Run("node with same key but different value at the back of the full cache", func(t *testing.T) {
		wantCap := 5
		c := createFullCache(wantCap)

		k, _ := keyValuePair(0)
		_, wantV := keyValuePair(5)
		_, wantB := keyValuePair(1)
		c.Put(k, wantV)

		checkCache(t, c, wantCap, wantCap, wantV, wantB)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
	})
	t.Run("node with same key but different value in the middle of the full cache", func(t *testing.T) {
		wantCap := 5
		c := createFullCache(wantCap)

		k, _ := keyValuePair(3)
		_, wantV := keyValuePair(5)
		_, wantB := keyValuePair(0)
		c.Put(k, wantV)

		checkCache(t, c, wantCap, wantCap, wantV, wantB)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
	})
	t.Run("node with same key but different value at the front of the sparse cache", func(t *testing.T) {
		wantCap, wantLen := 10, 5
		c := createSparseCache(wantCap, wantLen)

		k, _ := keyValuePair(4)
		_, wantV := keyValuePair(5)
		_, wantB := keyValuePair(0)
		c.Put(k, wantV)

		checkCache(t, c, wantCap, wantLen, wantV, wantB)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}

	})
	t.Run("node with same key but different value at the back of the sparse cache", func(t *testing.T) {
		wantCap, wantLen := 10, 5
		c := createSparseCache(wantCap, wantLen)

		k, _ := keyValuePair(0)
		_, wantV := keyValuePair(5)
		_, wantB := keyValuePair(1)
		c.Put(k, wantV)

		checkCache(t, c, wantCap, wantLen, wantV, wantB)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
	})
	t.Run("node with same key but different value in the middle of the sparse cache", func(t *testing.T) {
		wantCap, wantLen := 10, 5
		c := createSparseCache(wantCap, wantLen)

		k, _ := keyValuePair(3)
		_, wantV := keyValuePair(5)
		_, wantB := keyValuePair(0)
		c.Put(k, wantV)

		checkCache(t, c, wantCap, wantLen, wantV, wantB)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
	})
}

func TestCache_Get(t *testing.T) {
	t.Run("value from front of a cache", func(t *testing.T) {
		wantCap, wantLen := 10, 8
		c := createSparseCache(wantCap, wantLen)

		k, wantV := keyValuePair(7)
		_, wantB := keyValuePair(0)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
		checkCache(t, c, wantCap, wantLen, wantV, wantB)
	})
	t.Run("value from back of a cache", func(t *testing.T) {
		wantCap, wantLen := 10, 8
		c := createSparseCache(wantCap, wantLen)

		k, wantV := keyValuePair(0)
		_, wantB := keyValuePair(1)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
		checkCache(t, c, wantCap, wantLen, wantV, wantB)
	})
	t.Run("value from middle of a cache", func(t *testing.T) {
		wantCap, wantLen := 10, 8
		c := createSparseCache(wantCap, wantLen)

		k, wantV := keyValuePair(4)
		_, wantB := keyValuePair(0)
		gotV := c.Get(k)
		if res := bytes.Compare(wantV, gotV); res != 0 {
			t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
		}
		checkCache(t, c, wantCap, wantLen, wantV, wantB)
	})
	t.Run("multiple values from a back of cache", func(t *testing.T) {
		wantCap, wantLen := 10, 8
		c := createSparseCache(wantCap, wantLen)

		for i := 0; i < wantLen; i++ {
			k, wantV := keyValuePair(i)
			_, wantB := keyValuePair((i + 1) % wantLen)
			gotV := c.Get(k)
			if res := bytes.Compare(wantV, gotV); res != 0 {
				t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
			}
			checkCache(t, c, wantCap, wantLen, wantV, wantB)
		}
	})
	t.Run("multiple values from a front of cache", func(t *testing.T) {
		wantCap, wantLen := 10, 8
		c := createSparseCache(wantCap, wantLen)

		for i := wantLen - 1; i >= 0; i-- {
			k, wantV := keyValuePair(i)
			_, wantB := keyValuePair(0)
			gotV := c.Get(k)
			if res := bytes.Compare(wantV, gotV); res != 0 {
				t.Fatalf("expected value \"%s\" for key \"%s\" but got \"%s\"", wantV, k, gotV)
			}
			if i == 0 {
				_, wantB = keyValuePair(7)
			}
			checkCache(t, c, wantCap, wantLen, wantV, wantB)
		}
	})
}

func TestCache_Clear(t *testing.T) {
	wantCap := 10
	c := NewCache(wantCap)
	for i := 0; i < wantCap; i++ {
		k, v := keyValuePair(i)
		c.Put(k, v)
	}
	c.Clear()
	checkCache(t, c, wantCap, 0, nil, nil)
}

func checkCache(t *testing.T, c *Cache, wantCap, wantLen int, f, b []byte) {
	if c.Cap() != wantCap || c.Len() != wantLen {
		t.Fatalf("expected Cap: \"%d\" and len: \"%d\" but got Cap: \"%d\" and len: \"%d\"", wantCap, wantLen,
			c.Cap(), c.Len())
	}

	if res := bytes.Compare(f, c.Front()); res != 0 {
		t.Fatalf("expected front of cache: \"%s\" but got \"%s\"", f, c.Front())
	}

	if res := bytes.Compare(b, c.Back()); res != 0 {
		t.Fatalf("expected back of cache: \"%s\" but got \"%s\"", b, c.Back())
	}
}

func createFullCache(cap int) *Cache {
	c := NewCache(cap)
	for i := 0; i < cap; i++ {
		k, v := keyValuePair(i)
		c.Put(k, v)
	}
	return c
}

func createSparseCache(cap int, len int) *Cache {
	c := NewCache(cap)
	for i := 0; i < len; i++ {
		k, v := keyValuePair(i)
		c.Put(k, v)
	}
	return c
}

func keyValuePair(i int) ([]byte, []byte) {
	return []byte(fmt.Sprintf("key%d", i)), []byte(fmt.Sprintf("value%d", i))
}
