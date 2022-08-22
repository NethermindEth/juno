package starknet

type StarkNetError struct {
	Code    int
	Message string
}

func (s StarkNetError) Error() string {
	return s.Message
}

var (
	UnexpectedError = StarkNetError{
		Code:    -1,
		Message: "Unexpected error",
	}
	NotImplementedError = StarkNetError{
		Code:    -2,
		Message: "Not implemented",
	}
	InvalidStorageKey = StarkNetError{
		Code:    -3,
		Message: "Invalid storage key",
	}
	ContractNotFound = StarkNetError{
		Code:    20,
		Message: "Contract not found",
	}
	InvalidMessageSelector = StarkNetError{
		Code:    21,
		Message: "Invalid message selector",
	}
	InvalidCallData = StarkNetError{
		Code:    22,
		Message: "Invalid call data",
	}
	InvalidBlockId = StarkNetError{
		Code:    24,
		Message: "Invalid block id",
	}
	InvalidTxnHash = StarkNetError{
		Code:    25,
		Message: "Invalid txn hash",
	}
	InvalidTxnIndex = StarkNetError{
		Code:    27,
		Message: "Invalid transaction index in a block",
	}
	InvalidContractClassHash = StarkNetError{
		Code:    28,
		Message: "The supplied contract class hash is invalid or unknown",
	}
	NoBlocks = StarkNetError{
		Code:    32,
		Message: "There are no blocks",
	}
)
