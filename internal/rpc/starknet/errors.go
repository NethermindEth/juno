package starknet

type StarkNetError struct {
	Code    int
	Message string
}

func (s *StarkNetError) Error() string {
	return s.Message
}

func NewInvalidBlockHash() *StarkNetError {
	return &StarkNetError{
		Code:    24,
		Message: "Invalid block hash",
	}
}
