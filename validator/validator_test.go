package validator_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	jvalidator "github.com/NethermindEth/juno/validator"
	"github.com/stretchr/testify/require"
)

func TestVersion0x3Validation(t *testing.T) {
	t.Parallel()
	v := jvalidator.Validator()

	emptySlice := []*felt.Felt{}

	rpcVersions := []struct {
		name     string
		invokeTx func(version string) any
	}{
		{ //nolint:dupl // Similar invokeTx between rpc versions
			name: "v8",
			invokeTx: func(version string) any {
				daMode := rpcv8.DAModeL1
				return rpcv8.Transaction{
					Type:    rpcv8.TxnInvoke,
					Version: felt.NewUnsafeFromString[felt.Felt](version),
					Nonce:   &felt.Zero,
					ResourceBounds: &rpcv8.ResourceBoundsMap{
						L1Gas:     &rpcv8.ResourceBounds{},
						L2Gas:     &rpcv8.ResourceBounds{},
						L1DataGas: &rpcv8.ResourceBounds{},
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
			},
		},
		{ //nolint:dupl // Similar invokeTx between rpc versions
			name: "v9",
			invokeTx: func(version string) any {
				daMode := rpcv9.DAModeL1
				return rpcv9.Transaction{
					Type:    rpcv9.TxnInvoke,
					Version: felt.NewUnsafeFromString[felt.Felt](version),
					Nonce:   &felt.Zero,
					ResourceBounds: &rpcv9.ResourceBoundsMap{
						L1Gas:     &rpcv9.ResourceBounds{},
						L2Gas:     &rpcv9.ResourceBounds{},
						L1DataGas: &rpcv9.ResourceBounds{},
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
			},
		},
		{ //nolint:dupl // Similar invokeTx between rpc versions
			name: "v10",
			invokeTx: func(version string) any {
				daMode := rpcv10.DAModeL1
				return rpcv10.Transaction{
					Type:    rpcv10.TxnInvoke,
					Version: felt.NewUnsafeFromString[felt.Felt](version),
					Nonce:   &felt.Zero,
					ResourceBounds: &rpcv10.ResourceBoundsMap{
						L1Gas:     &rpcv10.ResourceBounds{},
						L2Gas:     &rpcv10.ResourceBounds{},
						L1DataGas: &rpcv10.ResourceBounds{},
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
			},
		},
	}

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

	for _, rpcVer := range rpcVersions {
		t.Run(rpcVer.name, func(t *testing.T) {
			t.Parallel()
			for _, txVer := range txVersions {
				t.Run(txVer.name, func(t *testing.T) {
					t.Parallel()
					tx := rpcVer.invokeTx(txVer.version)
					err := v.Struct(tx)
					if txVer.valid {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
						require.Contains(t, err.Error(), "version_0x3")
					}
				})
			}
		})
	}
}
