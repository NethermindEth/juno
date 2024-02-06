package vm

import "fmt"

type TransactionExecutionError struct {
	Index uint64
	Cause error
}

func (e TransactionExecutionError) Unwrap() error {
	return e.Cause
}

func (e TransactionExecutionError) Error() string {
	return fmt.Sprintf("execute transaction #%d: %s", e.Index, e.Cause)
}
