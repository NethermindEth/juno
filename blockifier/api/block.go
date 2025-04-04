package api

import (
	"errors"

	"github.com/NethermindEth/juno/core/felt"
)

///////////////////////////
//// BlockNumber
///////////////////////////

// BlockNumber represents a block number in the blockchain.
type BlockNumber uint64

// UncheckedNext returns the next block number without checking if it's in range.
func (bn BlockNumber) UncheckedNext() BlockNumber {
	return BlockNumber(bn + 1)
}

// Next returns the next block number, or nil if the next block number is out of range.
func (bn BlockNumber) Next() *BlockNumber {
	if bn == ^BlockNumber(0) { // Check for overflow
		return nil
	}
	next := bn + 1
	return &next
}

// Prev returns the previous block number, or nil if the previous block number is out of range.
func (bn BlockNumber) Prev() *BlockNumber {
	if bn == 0 {
		return nil
	}
	prev := bn - 1
	return &prev
}

// IterUpTo returns a slice of block numbers from self to up_to (exclusive).
func (bn BlockNumber) IterUpTo(upTo BlockNumber) []BlockNumber {
	if bn >= upTo {
		return nil
	}
	var blocks []BlockNumber
	for i := bn; i < upTo; i++ {
		blocks = append(blocks, i)
	}
	return blocks
}

///////////////////////////
//// FeeType
///////////////////////////

// FeeType represents the type of fee.
type FeeType int

const (
	FeeTypeStrk FeeType = iota
	FeeTypeEth
)

// String returns the string representation of the FeeType.
func (ft FeeType) String() string {
	switch ft {
	case FeeTypeStrk:
		return "Strk"
	case FeeTypeEth:
		return "Eth"
	default:
		return "Unknown"
	}
}

///////////////////////////
//// GasPrice
///////////////////////////

// GasPrice represents a gas price using felt.Felt to accommodate large values.
type GasPrice struct {
	value *felt.Felt
}

// NewGasPrice creates a new GasPrice from a uint64 value.
func NewGasPrice(val uint64) *GasPrice {
	return &GasPrice{value: new(felt.Felt).SetUint64(val)}
}

// NewGasPriceFromFelt creates a new GasPrice from a felt.Felt value.
func NewGasPriceFromFelt(val *felt.Felt) *GasPrice {
	return &GasPrice{value: new(felt.Felt).Set(val)}
}

// Value returns the underlying felt.Felt value of the GasPrice.
func (gp *GasPrice) Value() *felt.Felt {
	return new(felt.Felt).Set(gp.value)
}

// Add adds another GasPrice to this one and returns a new GasPrice.
func (gp *GasPrice) Add(other *GasPrice) *GasPrice {
	return &GasPrice{value: new(felt.Felt).Add(gp.value, other.value)}
}

// Mul multiplies the GasPrice by a GasAmount and returns a new Fee.
func (gp *GasPrice) Mul(amount GasAmount) *felt.Felt {
	return new(felt.Felt).Mul(gp.value, amount.Value())
}

// NonzeroGasPrice represents a non-zero gas price.
type NonzeroGasPrice struct {
	price GasPrice
}

// StarknetApiError represents errors related to Starknet API.
var (
	ErrZeroGasPrice = errors.New("zero gas price")
)

// NewNonzeroGasPrice creates a new NonzeroGasPrice, returning an error if the price is zero.
func NewNonzeroGasPrice(price GasPrice) (*NonzeroGasPrice, error) {
	if price.Value().IsZero() {
		return nil, ErrZeroGasPrice
	}
	return &NonzeroGasPrice{price: price}, nil
}

// Get returns the gas price.
func (ngp *NonzeroGasPrice) Get() GasPrice {
	return ngp.price
}

// SaturatingMul returns the product of the gas price and a gas amount, saturating at the maximum value.
func (ngp *NonzeroGasPrice) SaturatingMul(rhs GasAmount) Fee {
	product := new(felt.Felt).Mul(ngp.price.Value(), rhs.value)
	maxUint64 := new(felt.Felt).SetUint64(^uint64(0))
	if product.Cmp(maxUint64) > 0 {
		return Fee{value: maxUint64}
	}
	return Fee{value: product}
}

// NewUncheckedNonzeroGasPrice creates a new NonzeroGasPrice without checking the value.
func NewUncheckedNonzeroGasPrice(price GasPrice) *NonzeroGasPrice {
	return &NonzeroGasPrice{price: price}
}

// DefaultNonzeroGasPrice returns the default NonzeroGasPrice.
func DefaultNonzeroGasPrice() *NonzeroGasPrice {
	return &NonzeroGasPrice{price: *NewGasPrice(1)}
}

// FromNonzeroGasPrice converts a NonzeroGasPrice to a GasPrice.
func FromNonzeroGasPrice(val NonzeroGasPrice) GasPrice {
	return val.price
}

// TryFromGasPrice attempts to create a NonzeroGasPrice from a GasPrice.
func TryFromGasPrice(price GasPrice) (*NonzeroGasPrice, error) {
	return NewNonzeroGasPrice(price)
}

// GasPriceVector holds gas price information for L1 and L2.
type GasPriceVector struct {
	L1GasPrice     NonzeroGasPrice
	L1DataGasPrice NonzeroGasPrice
	L2GasPrice     NonzeroGasPrice
}

// Fee represents a fee using felt.Felt to accommodate large values.
type Fee struct {
	value *felt.Felt
}

// NewFee creates a new Fee from a uint64 value.
func NewFee(val uint64) *Fee {
	return &Fee{value: new(felt.Felt).SetUint64(val)}
}

// NewFeeFromFelt creates a new Fee from a felt.Felt value.
func NewFeeFromFelt(val *felt.Felt) *Fee {
	return &Fee{value: new(felt.Felt).Set(val)}
}

// CheckedAdd adds another Fee to this one and returns a new Fee, or nil if overflow occurs.
func (f *Fee) CheckedAdd(rhs *Fee) (*Fee, error) {
	sum := new(felt.Felt).Add(f.value, rhs.value)
	if sum.Cmp(f.value) < 0 || sum.Cmp(rhs.value) < 0 {
		return nil, errors.New("overflow occurred")
	}
	return &Fee{value: sum}, nil
}

// SaturatingAdd adds another Fee to this one, saturating at the maximum value.
func (f *Fee) SaturatingAdd(rhs *Fee) *Fee {
	return &Fee{value: new(felt.Felt).Add(f.value, rhs.value)}
}

// CheckedDivCeil divides the Fee by a NonzeroGasPrice, rounding up.
func (f *Fee) CheckedDivCeil(rhs *NonzeroGasPrice) (*GasAmount, error) {
	div, err := f.CheckedDiv(rhs)
	if err != nil {
		return nil, err
	}
	rhsGP := rhs.Get()
	mul := new(felt.Felt).Mul(div.Value(), rhsGP.value)
	if mul.Cmp(f.value) < 0 {
		div.value.Add(div.Value(), new(felt.Felt).SetUint64(1))
	}
	return div, nil
}

// CheckedDiv divides the Fee by a NonzeroGasPrice, returning a GasAmount or an error if division fails.
func (f *Fee) CheckedDiv(rhs *NonzeroGasPrice) (*GasAmount, error) {
	rhsGP := rhs.Get()
	if rhsGP.Value().IsZero() {
		return nil, errors.New("division by zero")
	}
	quotient := new(felt.Felt).Div(f.value, rhsGP.Value())
	return &GasAmount{value: quotient}, nil
}

// SaturatingDiv divides the Fee by a NonzeroGasPrice, saturating at the maximum value.
func (f *Fee) SaturatingDiv(rhs *NonzeroGasPrice) *GasAmount {
	div, err := f.CheckedDiv(rhs)
	if err != nil {
		return &GasAmount{value: new(felt.Felt).SetUint64(^uint64(0))}
	}
	return div
}

// GasAmount represents an amount of gas using felt.Felt.
type GasAmount struct {
	value *felt.Felt
}

// Value returns the underlying felt.Felt value of the GasAmount.
func (ga *GasAmount) Value() *felt.Felt {
	return new(felt.Felt).Set(ga.value)
}

// GasPrices holds gas prices for different fee types.
type GasPrices struct {
	EthGasPrices  GasPriceVector // In wei.
	StrkGasPrices GasPriceVector // In fri.
}

// L1GasPrice returns the L1 gas price for the given fee type.
func (gp *GasPrices) L1GasPrice(feeType FeeType) NonzeroGasPrice {
	return gp.GasPriceVector(feeType).L1GasPrice
}

// L1DataGasPrice returns the L1 data gas price for the given fee type.
func (gp *GasPrices) L1DataGasPrice(feeType FeeType) NonzeroGasPrice {
	return gp.GasPriceVector(feeType).L1DataGasPrice
}

// L2GasPrice returns the L2 gas price for the given fee type.
func (gp *GasPrices) L2GasPrice(feeType FeeType) NonzeroGasPrice {
	return gp.GasPriceVector(feeType).L2GasPrice
}

// GasPriceVector returns the GasPriceVector for the given fee type.
func (gp *GasPrices) GasPriceVector(feeType FeeType) *GasPriceVector {
	switch feeType {
	case FeeTypeStrk:
		return &gp.StrkGasPrices
	case FeeTypeEth:
		return &gp.EthGasPrices
	default:
		return nil
	}
}
