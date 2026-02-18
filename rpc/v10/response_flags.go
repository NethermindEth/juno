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
	*r = ResponseFlags{}

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

type SubscriptionTags struct {
	IncludeProofFacts bool
}

func (r *SubscriptionTags) UnmarshalJSON(data []byte) error {
	var flags []string
	if err := json.Unmarshal(data, &flags); err != nil {
		return err
	}

	*r = SubscriptionTags{}

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
