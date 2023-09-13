package rpc

import (
	"encoding/json"
	"fmt"
	"regexp"

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

// regex for finding json key "revert_reason" and it's value of type string.
var r = regexp.MustCompile(`"revert_reason"\s*:\s*"(.+?)"`)

// findRevertReason tries to find `revert_reason` key within provided message.
// Returns empty string when not found.
func findRevertReason(msg json.RawMessage) string {
	sub := r.FindStringSubmatch(string(msg))
	if sub == nil || len(sub) < 2 {
		return ""
	}
	return sub[1]
}
