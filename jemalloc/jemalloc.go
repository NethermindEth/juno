package jemalloc

// Does it mean that Jemalloc will be used as default allocator in Rust?
/*
// This cgo directive is what actually causes jemalloc to be linked in to the
// final Go executable
#cgo pkg-config: jemalloc

#include <jemalloc/jemalloc.h>

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

// So jemalloc is turned on by default? do we need to specify somewhere that jemalloc library is required
// And could you explain shortly what benefits we found by using jemalloc
func GetActive() C.ulonglong {
	C._refresh_jemalloc_stats()
	return C._get_jemalloc_active()
}
