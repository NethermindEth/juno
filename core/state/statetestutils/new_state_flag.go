package statetestutils

import (
	"os"
	"strconv"
)

var UseNewState = func() bool {
	val := os.Getenv("JUNO_NEW_STATE")
	parsed, err := strconv.ParseBool(val)
	return err == nil && parsed
}
