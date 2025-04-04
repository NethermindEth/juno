package transaction

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

// TransactionReceipt contains all receipt information for a transaction.
type TransactionReceipt struct {
	// Fee represents the transaction fee that was charged (in units of the relevant fee token).
	Fee felt.Felt // Todo: Can we just use a felt here?

	// Gas represents the actual gas consumption the transaction is charged for execution.
	Gas GasVector

	// DaGas represents the actual gas consumption the transaction is charged for data availability.
	DaGas GasVector

	// Resources represents the actual execution resources the transaction
	// is charged for (including L1 gas and additional OS resources estimation).
	Resources TransactionResources
}

// FeeToken represents the token used for fees
type FeeToken uint8

const (
	ETH FeeToken = iota // Todo: will ETH fees be turned off??
	STRK
)

// String returns the string representation of the fee token
func (ft FeeToken) String() string {
	switch ft {
	case ETH:
		return "ETH"
	case STRK:
		return "STRK"
	default:
		return "UNKNOWN"
	}
}

// FeeTokenFromString converts a string to a FeeToken
func FeeTokenFromString(s string) (FeeToken, error) {
	switch s {
	case "ETH":
		return ETH, nil
	case "STRK":
		return STRK, nil
	default:
		return 0, fmt.Errorf("invalid fee token: %s", s)
	}
}

// Fee represents the transaction fee in units of the relevant fee token.
type Fee struct {
	// Amount is the fee amount in wei
	Amount felt.Felt
	// Unit is the token unit (e.g., ETH, STRK)
	Unit FeeToken
}

// GasAmount represents an amount of gas
// Todo: why can't we just replace with a felt?
type GasAmount struct {
	Amount uint64
}

// CheckedAdd performs addition with overflow checking
func (g GasAmount) CheckedAdd(other GasAmount) (GasAmount, error) {
	if g.Amount > (^uint64(0) - other.Amount) {
		return GasAmount{}, errors.New("gas amount addition overflow")
	}
	return GasAmount{Amount: g.Amount + other.Amount}, nil
}

// CheckedMul performs multiplication with overflow checking
func (g GasAmount) CheckedMul(factor uint64) (GasAmount, error) {
	if factor != 0 && g.Amount > (^uint64(0)/factor) {
		return GasAmount{}, errors.New("gas amount multiplication overflow")
	}
	return GasAmount{Amount: g.Amount * factor}, nil
}

// GasVector represents different types of gas consumption
type GasVector struct {
	L1Gas     GasAmount
	L1DataGas GasAmount
	L2Gas     GasAmount
}

// ZeroGasVector returns a GasVector with all values set to zero
func ZeroGasVector() GasVector {
	return GasVector{
		L1Gas:     GasAmount{Amount: 0},
		L1DataGas: GasAmount{Amount: 0},
		L2Gas:     GasAmount{Amount: 0},
	}
}

// FromL1Gas creates a GasVector with only L1Gas set
func FromL1Gas(l1Gas GasAmount) GasVector {
	return GasVector{
		L1Gas:     l1Gas,
		L1DataGas: GasAmount{Amount: 0},
		L2Gas:     GasAmount{Amount: 0},
	}
}

// FromL1DataGas creates a GasVector with only L1DataGas set
func FromL1DataGas(l1DataGas GasAmount) GasVector {
	return GasVector{
		L1Gas:     GasAmount{Amount: 0},
		L1DataGas: l1DataGas,
		L2Gas:     GasAmount{Amount: 0},
	}
}

// FromL2Gas creates a GasVector with only L2Gas set
func FromL2Gas(l2Gas GasAmount) GasVector {
	return GasVector{
		L1Gas:     GasAmount{Amount: 0},
		L1DataGas: GasAmount{Amount: 0},
		L2Gas:     l2Gas,
	}
}

// CheckedAdd adds two GasVectors with overflow checking
func (gv GasVector) CheckedAdd(other GasVector) (GasVector, error) {
	l1Gas, err := gv.L1Gas.CheckedAdd(other.L1Gas)
	if err != nil {
		return GasVector{}, err
	}

	l1DataGas, err := gv.L1DataGas.CheckedAdd(other.L1DataGas)
	if err != nil {
		return GasVector{}, err
	}

	l2Gas, err := gv.L2Gas.CheckedAdd(other.L2Gas)
	if err != nil {
		return GasVector{}, err
	}

	return GasVector{
		L1Gas:     l1Gas,
		L1DataGas: l1DataGas,
		L2Gas:     l2Gas,
	}, nil
}

// CheckedScalarMul multiplies all components by a factor with overflow checking
func (gv GasVector) CheckedScalarMul(factor uint64) (GasVector, error) {
	l1Gas, err := gv.L1Gas.CheckedMul(factor)
	if err != nil {
		return GasVector{}, err
	}

	l1DataGas, err := gv.L1DataGas.CheckedMul(factor)
	if err != nil {
		return GasVector{}, err
	}

	l2Gas, err := gv.L2Gas.CheckedMul(factor)
	if err != nil {
		return GasVector{}, err
	}

	return GasVector{
		L1Gas:     l1Gas,
		L1DataGas: l1DataGas,
		L2Gas:     l2Gas,
	}, nil
}

// Todo: Big
type TransactionResources struct {
	// StarknetResources
	// ComputationResources
}
