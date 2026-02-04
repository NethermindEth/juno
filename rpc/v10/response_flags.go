package rpcv10

import (
	"encoding/json"
	"fmt"
)

type ResponseFlags struct {
	IncludeProofFacts bool
}

func (r *ResponseFlags) UnmarshalJSON(data []byte) error {
	var flags []string
	if err := json.Unmarshal(data, &flags); err != nil {
		return err
	}

	r.IncludeProofFacts = false

	for _, flag := range flags {
		switch flag {
		case "INCLUDE_PROOF_FACTS":
			r.IncludeProofFacts = true
		default:
			return fmt.Errorf("unknown flag: %s", flag)
		}
	}

	return nil
}
