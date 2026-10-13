package rpcv10

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

type ResponseFlags struct {
	IncludeProofFacts bool
}

func (r *ResponseFlags) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	var flags []string
	if err := json.UnmarshalDecode(dec, &flags); err != nil {
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

func (r *SubscriptionTags) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	var flags []string
	if err := json.UnmarshalDecode(dec, &flags); err != nil {
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
