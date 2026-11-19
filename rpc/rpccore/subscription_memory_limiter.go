package rpccore

import "sync/atomic"

// SubscriptionMemoryLimiter bounds the memory, in bytes, that subscriptions hold together.
// A zero limit counts it without rejecting.
type SubscriptionMemoryLimiter struct {
	limit    int64
	used     atomic.Int64
	rejected atomic.Uint64
}

func NewSubscriptionMemoryLimiter(limit int64) *SubscriptionMemoryLimiter {
	return &SubscriptionMemoryLimiter{limit: limit}
}

// TryAcquire reserves size bytes if they fit under the limit
func (l *SubscriptionMemoryLimiter) TryAcquire(size int64) bool {
	for {
		used := l.used.Load()
		if l.limit > 0 && used+size > l.limit {
			l.rejected.Add(1)
			return false
		}
		if l.used.CompareAndSwap(used, used+size) {
			return true
		}
	}
}

func (l *SubscriptionMemoryLimiter) Release(size int64) { l.used.Add(-size) }

func (l *SubscriptionMemoryLimiter) Limit() int64 { return l.limit }

func (l *SubscriptionMemoryLimiter) Used() int64 { return l.used.Load() }

func (l *SubscriptionMemoryLimiter) Rejected() uint64 { return l.rejected.Load() }
