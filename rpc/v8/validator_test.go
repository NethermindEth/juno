package rpcv8_test

import (
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	"github.com/stretchr/testify/require"
)

func TestVersion0x3Validation(t *testing.T) {
	t.Parallel()
	v := rpcv8.Validator()

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
			daMode := rpcv8.DAModeL1
			tx := rpcv8.Transaction{
				Type:    rpcv8.TxnInvoke,
				Version: felt.NewUnsafeFromString[felt.Felt](txVer.version),
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
