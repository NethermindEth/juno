package utils

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
)

// SignPayload constructs the byte sequence that the publisher signs:
//
//	"<propeller>" || root[0:32] || "</propeller>"
//
// The tags domain-separate propeller signatures from any other protocol
// that might use the same key, preventing cross-protocol signature reuse.
func SignPayload[T ~[32]byte](root T) []byte {
	payload := make([]byte, 0, len("<propeller>")+32+len("</propeller>"))
	payload = append(payload, []byte("<propeller>")...)
	payload = append(payload, root[:]...)
	payload = append(payload, []byte("</propeller>")...)
	return payload
}

// todo(rdr): verify this is correct
// SignRoot signs the Merkle root with the given private key, producing the
// signature that goes into every PropellerUnit for this message.
func SignRoot[T ~[32]byte](root T, privKey crypto.PrivKey) ([]byte, error) {
	payload := SignPayload(root)
	sig, err := privKey.Sign(payload)
	if err != nil {
		return nil, fmt.Errorf("signing message root: %w", err)
	}
	return sig, nil
}
