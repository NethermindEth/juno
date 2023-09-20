package utils

import "fmt"

func RunAndWrapOnError(runnable func() error, existingErr error) error {
	if runErr := runnable(); runErr != nil {
		if existingErr == nil {
			return runErr
		}
		return fmt.Errorf(`failed to run because "%v" with existing err "%w"`, runErr, existingErr)
	}
	return existingErr
}
