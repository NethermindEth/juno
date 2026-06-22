package starknet

import "github.com/NethermindEth/juno/core/felt"

// Signature object returned by the feeder in JSON format for "get_signature" endpoint
type Signature struct {
	BlockNumber    uint64       `json:"block_number"`
	Signature      []*felt.Felt `json:"signature"`
	SignatureInput struct {
		BlockHash           *felt.Felt `json:"block_hash"`
		StateDiffCommitment *felt.Felt `json:"state_diff_commitment"`
	} `json:"signature_input"`
}

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.
func (val *Signature) Validate() error {
	return nil
}
