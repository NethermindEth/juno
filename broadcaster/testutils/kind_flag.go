package testutils

import (
	"os"
	"strconv"

	"github.com/NethermindEth/juno/broadcaster"
)

// Kind selects the broadcaster backend for a test run from the JUNO_BROADCAST
// environment variable, mirroring core/state/testutils.UseNewState. Unset or
// falsey yields broadcaster.KindFeed (the default backend); truthy yields
// broadcaster.KindBroadcast so the whole suite can be exercised on the ring.
var Kind = func() broadcaster.Kind {
	val := os.Getenv("JUNO_BROADCAST")
	parsed, err := strconv.ParseBool(val)
	if err == nil && parsed {
		return broadcaster.KindBroadcast
	}
	return broadcaster.KindFeed
}
