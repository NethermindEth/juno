package jemalloc

/*
// This cgo directive is what actually causes jemalloc to be linked in to the
// final Go executable
#cgo pkg-config: jemalloc

#include <jemalloc/jemalloc.h>

// jemalloc defaults to 4 arenas per CPU. The default assumes a thread frees what it allocated and
// allocates again soon, so a private arena per thread avoids locks and cross-thread traffic.
// Juno breaks that assumption. The Go scheduler runs the next request on a different thread, and
// the VM's allocations are far above the 32 KB thread-cache limit. Memory freed on one thread is
// never reused by another: it is purged to the OS and page-faulted back in on the next request.
// Pebble's block cache allocates through C.calloc and is affected the same way.
// Two shared arenas let freed memory be reused across threads. That lowers latency and RSS.
// The cost is more threads per arena lock. It stays small here: VM concurrency is capped by
// max-vms and each request makes tens of large allocations, so two arenas are far from saturation,
// while the purge and page-fault cost would be paid on every request.
const char *malloc_conf = "narenas:2";

void _refresh_jemalloc_stats() {
	// You just need to pass something not-null into the "epoch" mallctl.
	size_t random_something = 1;
	mallctl("epoch", NULL, NULL, &random_something, sizeof(random_something));
}
unsigned long long _get_jemalloc_active() {
	size_t stat, stat_size;
	stat = 0;
	stat_size = sizeof(stat);
	mallctl("stats.active", &stat, &stat_size, NULL, 0);
	return (unsigned long long)stat;
}
*/
import "C"

func GetActive() C.ulonglong {
	C._refresh_jemalloc_stats()
	return C._get_jemalloc_active()
}
