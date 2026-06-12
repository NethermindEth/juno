package propeller

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
)

const payloadLen = 95

// buildSignPayload constructs the byte sequence that the publisher signs.
// Does it in constant time without heap allocations
func buildSignPayload(
	root *MessageRoot, committeeID *CommitteeID, nonce Nonce,
) [payloadLen]byte {
	// The tags domain-separate propeller signatures from any other protocol
	// that might use the same key, preventing cross-protocol signature reuse.
	const prefix = "<propeller>"
	const suffix = "<propeller/>"

	// cumulative lengths denoting the ranges in where each bytes of data should be stored
	const prefixLen = len(prefix)
	const rootLen = prefixLen + 32
	const committeeIDLen = rootLen + 32
	const nonceLen = committeeIDLen + 8
	const suffixLen = nonceLen + len(suffix)

	var payload [payloadLen]byte

	copy(payload[0:prefixLen], prefix)
	copy(payload[prefixLen:rootLen], root[:])
	copy(payload[rootLen:committeeIDLen], committeeID[:])
	binary.BigEndian.PutUint64(payload[committeeIDLen:nonceLen], uint64(nonce))
	copy(payload[nonceLen:suffixLen], suffix)

	return payload
}

func SignMessage(
	privKey crypto.PrivKey,
	root *MessageRoot,
	committeeID *CommitteeID,
	nonce Nonce,
) (Signature, error) {
	payload := buildSignPayload(root, committeeID, nonce)
	sig, err := privKey.Sign(payload[:])
	if err != nil {
		return nil, fmt.Errorf("signing message root: %w", err)
	}
	return sig, nil
}

func VerifyMessageSignature(
	pubKey crypto.PubKey,
	root *MessageRoot,
	committeeID *CommitteeID,
	nonce Nonce,
	signature Signature,
) error {
	if len(signature) == 0 {
		return errors.New("empty signature")
	}

	payload := buildSignPayload(root, committeeID, nonce)
	valid, err := pubKey.Verify(payload[:], signature)
	if err != nil {
		return fmt.Errorf("failed pub key verification: %w", err)
	}

	if !valid {
		return errors.New("signature is invalid")
	}

	return nil
}
