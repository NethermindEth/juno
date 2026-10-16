package records

import (
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils/cbor"
	"github.com/NethermindEth/juno/utils/cbor/fast"
	"github.com/stretchr/testify/require"
)

// The readers expect each key where the encoder that writes the records puts it. If the
// encoder's options or a struct change the order, reads stay correct but take the slow path for
// every key; this test fails first.
func TestFieldsFollowTheEncoder(t *testing.T) {
	for name, tt := range map[string]struct {
		fields *fast.Fields
		value  any
	}{
		"receipt":             {receiptFields, core.TransactionReceipt{}},
		"event":               {eventFields, core.Event{}},
		"l1 to l2 message":    {l1ToL2MessageFields, core.L1ToL2Message{}},
		"l2 to l1 message":    {l2ToL1MessageFields, core.L2ToL1Message{}},
		"execution resources": {executionResourcesFields, core.ExecutionResources{}},
		"data availability":   {dataAvailabilityFields, core.DataAvailability{}},
		"gas consumed":        {gasConsumedFields, core.GasConsumed{}},
		"builtins":            {builtinInstanceCounterFields, core.BuiltinInstanceCounter{}},
		"header":              {headerFields, core.Header{}},
		"gas price":           {gasPriceFields, core.GasPrice{}},
		"state update":        {stateUpdateFields, core.StateUpdate{}},
		"state diff":          {stateDiffFields, core.StateDiff{}},
		"sierra class":        {sierraClassFields, core.SierraClass{}},
		"sierra entry points": {sierraEntryPointsFields, core.SierraEntryPointsByType{}},
		"sierra entry point":  {sierraEntryPointFields, core.SierraEntryPoint{}},
		"casm class":          {casmClassFields, core.CasmClass{}},
		"casm entry point":    {casmEntryPointFields, core.CasmEntryPoint{}},
		"segment lengths":     {segmentLengthsFields, core.SegmentLengths{}},
		"invoke":              {invokeFields, core.InvokeTransaction{ProofFacts: []felt.Felt{{}}}},
		"declare":             {declareFields, core.DeclareTransaction{}},
		"deploy":              {deployFields, core.DeployTransaction{}},
		"deploy account":      {deployAccountFields, core.DeployAccountTransaction{}},
		"l1 handler":          {l1HandlerFields, core.L1HandlerTransaction{}},
		"resource bounds":     {boundsFields, core.ResourceBounds{}},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := cbor.Marshal(tt.value)
			require.NoError(t, err)

			r := fast.NewReader(data)
			if _, err = r.Tag(); err != nil { // the tag of a registered type, if it has one
				require.ErrorIs(t, err, fast.ErrType)
			}
			count, err := r.MapLen()
			require.NoError(t, err)
			keys := make([]string, count)
			for i := range keys {
				keys[i], err = r.String()
				require.NoError(t, err)
				require.NoError(t, r.Skip())
			}
			require.Equal(t, keys, tt.fields.Order())
		})
	}
}
