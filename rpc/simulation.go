package rpc

import (
	"encoding/json"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
)

type SimulationFlag int

const (
	SkipValidateFlag SimulationFlag = iota + 1
	SkipFeeChargeFlag
)

func (s *SimulationFlag) UnmarshalJSON(bytes []byte) (err error) {
	switch flag := string(bytes); flag {
	case `"SKIP_VALIDATE"`:
		*s = SkipValidateFlag
	case `"SKIP_FEE_CHARGE"`:
		*s = SkipFeeChargeFlag
	default:
		err = fmt.Errorf("unknown simulation flag %q", flag)
	}

	return
}

type SimulatedTransaction struct {
	TransactionTrace json.RawMessage `json:"transaction_trace,omitempty"`
	FeeEstimation    FeeEstimate     `json:"fee_estimation,omitempty"`
}

type TracedBlockTransaction struct {
	TraceRoot       json.RawMessage `json:"trace_root,omitempty"`
	TransactionHash *felt.Felt      `json:"transaction_hash,omitempty"`
}
