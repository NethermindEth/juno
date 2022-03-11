// Package errpkg provides convenience functions for checking and
// handling errors. Its main purpose is to reduce the verbosity of error
// checking code by providing default handling logic.
//
// See also:
//  - https://github.com/golang/proposal/blob/master/design/go2draft-error-handling-overview.md
//  - https://github.com/golang/proposal/blob/master/design/go2draft-error-handling.md
package errpkg

// notest
import (
	"os"

	"github.com/NethermindEth/juno/internal/log"
)

// CheckFatal checks whether an error occurred, logs it using the default
// logger, and then calls os.Exit(1).
func CheckFatal(err error, msg string) {
	if err != nil {
		log.Default.With("Error", err).Error(msg)
		os.Exit(1)
	}
}
