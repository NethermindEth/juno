package api

// ExecutionFlags controls transaction execution behavior
type ExecutionFlags struct {
	OnlyQuery bool
	ChargeFee bool
	Validate  bool
}

// DefaultExecutionFlags returns execution flags with standard settings
func DefaultExecutionFlags() ExecutionFlags {
	return ExecutionFlags{
		OnlyQuery: false,
		ChargeFee: true,
		Validate:  true,
	}
}
