package cache

import "crypto/sha1"

type chacheKey [sha1.Size]byte

func newCacheKey(v []byte) chacheKey {
	return sha1.Sum(v)
}
