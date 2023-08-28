package utils

var (
	InvalidContractClass            ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS"
	UndeclaredClass                 ErrorCode = "StarknetErrorCode.UNDECLARED_CLASS"
	ClassAlreadyDeclared            ErrorCode = "StarknetErrorCode.CLASS_ALREADY_DECLARED"
	InsufficientMaxFee              ErrorCode = "StarknetErrorCode.INSUFFICIENT_MAX_FEE"
	InsufficientAccountBalance      ErrorCode = "StarknetErrorCode.INSUFFICIENT_ACCOUNT_BALANCE"
	ValidateFailure                 ErrorCode = "StarknetErrorCode.VALIDATE_FAILURE"
	ContractBytecodeSizeTooLarge    ErrorCode = "StarknetErrorCode.CONTRACT_BYTECODE_SIZE_TOO_LARGE"
	DuplicatedTransaction           ErrorCode = "StarknetErrorCode.DUPLICATED_TRANSACTION"
	InvalidTransactionNonce         ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_NONCE"
	CompilationFailed               ErrorCode = "StarknetErrorCode.COMPILATION_FAILED"
	InvalidCompiledClassHash        ErrorCode = "StarknetErrorCode.INVALID_COMPILED_CLASS_HASH"
	ContractClassObjectSizeTooLarge ErrorCode = "StarknetErrorCode.CONTRACT_CLASS_OBJECT_SIZE_TOO_LARGE"
	InvalidTransactionVersion       ErrorCode = "StarknetErrorCode.INVALID_TRANSACTION_VERSION"
	InvalidContractClassVersion     ErrorCode = "StarknetErrorCode.INVALID_CONTRACT_CLASS_VERSION"
)

type ErrorCode string

type Error struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}
