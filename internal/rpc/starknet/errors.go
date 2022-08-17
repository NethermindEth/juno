package starknet

type StarkNetError struct {
	Code    int
	Message string
}

func (s *StarkNetError) Error() string {
	return s.Message
}

func NewInvalidStorageKey() *StarkNetError {
    // NOTE: this error is not documented in the RPC specification
    return &StarkNetError{
        Code:    -1,
        Message: "Invalid storage key",
    }
}

func NewContractNotFound() *StarkNetError {
	return &StarkNetError{
		Code:    20,
		Message: "Contract not found",
	}
}

func NewInvalidMessageSelector() *StarkNetError {
	return &StarkNetError{
		Code:    21,
		Message: "Invalid message selector",
	}
}

func NewInvalidCallData() *StarkNetError {
	return &StarkNetError{
		Code:    22,
		Message: "Invalid call data",
	}
}

func NewInvalidBlockId() *StarkNetError {
	return &StarkNetError{
		Code:    24,
		Message: "Invalid block id",
	}
}

func NewInvalidTxnHash() *StarkNetError {
	return &StarkNetError{
		Code:    25,
		Message: "Invalid txn hash",
	}
}

func NewInvalidTxnIndex() *StarkNetError {
	return &StarkNetError{
		Code:    26,
		Message: "Invalid transaction index in a block",
	}
}

func NewInvalidContractClassHash() *StarkNetError {
	return &StarkNetError{
		Code:    28,
		Message: "The supplied contract class hash is invalid or unknown",
	}
}

func NewNoBlocks() *StarkNetError {
	return &StarkNetError{
		Code:    32,
		Message: "There are no blocks",
	}
}
