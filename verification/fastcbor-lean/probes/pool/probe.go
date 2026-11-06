package pool

import "sync"

var cache = sync.Pool{New: func() any { return new(uint64) }}

func Borrow() *uint64    { return cache.Get().(*uint64) }
func GiveBack(p *uint64) { *p = 0; cache.Put(p) }
