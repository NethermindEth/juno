package statetestutils

import (
	"os"
	"strconv"
	"sync"
)

var (
	useNewState bool
	once        sync.Once
)

func UseNewState() bool {
	once.Do(func() {
		val := os.Getenv("USE_NEW_STATE")
		parsed, err := strconv.ParseBool(val)
		useNewState = err == nil && parsed
	})
	return useNewState
}
