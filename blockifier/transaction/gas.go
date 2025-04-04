package transaction

// Todo: move to api??

import (
	"errors"
)

// GasCounter tracks the gas usage
type GasCounter struct {
	SpentGas     GasAmount
	RemainingGas GasAmount
}

// NewGasCounter initializes a new GasCounter with the given initial gas
func NewGasCounter(initialGas GasAmount) *GasCounter {
	return &GasCounter{
		SpentGas:     GasAmount{},
		RemainingGas: initialGas,
	}
}

// Spend deducts the specified amount of gas, updating spent and remaining gas
func (gc *GasCounter) Spend(amount GasAmount) error {
	if gc.RemainingGas.Amount < amount.Amount {
		return errors.New("overuse of gas; should have been caught earlier")
	}
	gc.SpentGas.Amount += amount.Amount
	gc.RemainingGas.Amount -= amount.Amount
	return nil
}

// LimitUsage limits the amount of gas that can be used by the given global limit
func (gc *GasCounter) LimitUsage(amount GasAmount) GasAmount {
	if gc.RemainingGas.Amount < amount.Amount {
		return gc.RemainingGas
	}
	return amount
}

// SubtractUsedGas subtracts the gas used by a call operation
func (gc *GasCounter) SubtractUsedGas(callInfo *CallInfo) error {
	return gc.Spend(GasAmount{callInfo.Execution.GasConsumed})
}
