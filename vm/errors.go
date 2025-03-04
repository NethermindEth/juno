package vm

import (
	"encoding/json"
	"fmt"
)

type TransactionExecutionError struct {
	Index uint64
	Cause json.RawMessage
}

func (e TransactionExecutionError) Error() string {
	return fmt.Sprintf("execute transaction #%d: %s", e.Index, e.Cause)
}
