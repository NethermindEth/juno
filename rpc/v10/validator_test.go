package rpcv10_test

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	rpc "github.com/NethermindEth/juno/rpc/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateFeltMaxBits(t *testing.T) {
	t.Parallel()
	v := rpc.Validator()
	const maxFeltSize = 251

	maxHexOfBits := func(bits int) string {
		if bits < 1 {
			return "0x0"
		}
		// Create a uint64 or big.Int depending on size
		if bits <= 64 {
			val := (uint64(1) << bits) - 1
			return "0x" + strconv.FormatUint(val, 16)
		}
		// For bits > 64, use big.Int
		val := new(big.Int).Lsh(big.NewInt(1), uint(bits))
		val.Sub(val, big.NewInt(1))
		return "0x" + val.Text(16)
	}

	testData := make(map[int]string)
	for i := 1; i <= maxFeltSize; i++ {
		testData[i] = maxHexOfBits(i)
	}

	for maxBits := 1; maxBits <= maxFeltSize; maxBits++ {
		t.Run("max bits = "+strconv.Itoa(maxBits), func(t *testing.T) {
			t.Parallel()

			for i := 1; i <= maxFeltSize; i++ {
				f := felt.NewUnsafeFromString[felt.Felt](testData[i])
				err := v.Var(f, "felt_max_bits="+strconv.Itoa(maxBits))
				if i <= maxBits {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			}
		})
	}
}

func TestValidateFeltMaxBitsInStruct(t *testing.T) {
	t.Parallel()
	v := rpc.Validator()

	// boundsStruct exercises the felt_max_bits tag in isolation.
	type boundsStruct struct {
		Amount *felt.Felt `validate:"felt_max_bits=64"`
	}

	assert.NoError(
		t, v.Struct(boundsStruct{Amount: felt.NewUnsafeFromString[felt.Felt]("0xffffffffffffffff")}),
	)
	assert.Error(
		t, v.Struct(boundsStruct{Amount: felt.NewUnsafeFromString[felt.Felt]("0x10000000000000000")}),
	)
}

func TestVersion0x3Validation(t *testing.T) {
	t.Parallel()
	v := rpc.Validator()

	emptySlice := []*felt.Felt{}

	txVersions := []struct {
		name    string
		version string
		valid   bool
	}{
		{name: "v3", version: "0x3", valid: true},
		{name: "v3 query", version: "0x100000000000000000000000000000003", valid: true},
		{name: "v0", version: "0x0", valid: false},
		{name: "v1", version: "0x1", valid: false},
		{name: "v2", version: "0x2", valid: false},
	}

	for _, txVer := range txVersions {
		t.Run(txVer.name, func(t *testing.T) {
			t.Parallel()
			daMode := rpc.DAModeL1
			tx := rpc.Transaction{
				Type:    rpc.TxnInvoke,
				Version: felt.NewUnsafeFromString[felt.Felt](txVer.version),
				Nonce:   &felt.Zero,
				ResourceBounds: &rpc.ResourceBoundsMap{
					L1Gas:     rpc.ResourceBounds{MaxAmount: &felt.Zero, MaxPricePerUnit: &felt.Zero},
					L2Gas:     rpc.ResourceBounds{MaxAmount: &felt.Zero, MaxPricePerUnit: &felt.Zero},
					L1DataGas: rpc.ResourceBounds{MaxAmount: &felt.Zero, MaxPricePerUnit: &felt.Zero},
				},
				SenderAddress:         &felt.Zero,
				Signature:             &emptySlice,
				CallData:              &emptySlice,
				Tip:                   &felt.Zero,
				PaymasterData:         &emptySlice,
				AccountDeploymentData: &emptySlice,
				NonceDAMode:           &daMode,
				FeeDAMode:             &daMode,
			}
			err := v.Struct(tx)
			if txVer.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), "version_0x3")
			}
		})
	}
}
