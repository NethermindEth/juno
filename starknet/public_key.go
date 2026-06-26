package starknet

// PublicKey is the sequencer's constant STARK curve public key in hex format.
type PublicKey string

// TODO: placeholder for now to avoid compiler errors. A proper validation
// should be implemented in a follow-up PR.
func (val *PublicKey) Validate() error {
	return nil
}
