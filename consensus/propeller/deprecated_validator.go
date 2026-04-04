package propeller

import (
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
)

// Validator checks incoming PropellerUnits for correctness. Each check
// serves a specific defensive purpose:
//
//   - Self-sending check: prevents reflection attacks.
//   - Self-published check: we already have all shards for our own messages.
//   - Duplicate check: avoids redundant work and state corruption.
//   - Origin check: ensures the sender is the peer assigned to this shard,
//     preventing sybil-like relay attacks.
//   - Merkle proof check: ensures the shard data is authentic (matches the
//     committed tree root).
//   - Signature check: ensures the publisher actually authored the message
//     (the root they committed to).
//
// These checks are ordered from cheapest to most expensive so we reject
// invalid units as early as possible.
type Validator struct {
	schedule  *Scheduler
	localPeer peer.ID
	verifier  SignatureVerifier
}

// NewValidator creates a validator for the given channel configuration.
func NewValidator(
	schedule *Scheduler,
	localPeer peer.ID,
	verifier SignatureVerifier,
) *Validator {
	return &Validator{
		schedule:  schedule,
		localPeer: localPeer,
		verifier:  verifier,
	}
}

// ValidateUnit checks an incoming unit against all validation rules.
//
// Parameters:
//   - unit: the incoming PropellerUnit to validate.
//   - sender: the peer.ID of the network peer that sent us this unit.
//   - seenShards: set of shard indices already received for this message
//     (used for duplicate detection).
//   - signatureVerified: true if we have already verified the publisher's
//     signature for this Merkle root. Allows skipping the expensive crypto
//     check after the first shard from the same message passes.
//
// Returns nil if valid, or a *ShardValidationError describing the failure.
func (v *Validator) ValidateUnit(
	unit *Unit,
	sender peer.ID,
	seenShards map[ShardIndex]bool,
	signatureVerified bool,
) error {
	// 1. Reject units from ourselves (should never happen in normal
	//    operation; indicates a routing bug or reflection attack).
	if sender == v.localPeer {
		return &ShardValidationError{
			Reason: ReasonSelfSending,
			Detail: "received unit from ourselves",
		}
	}

	// 2. Reject units for messages we published (we already have all
	//    shards and don't need them relayed back).
	if unit.Publisher == v.localPeer {
		return &ShardValidationError{
			Reason: ReasonReceivedSelfPublishedShard,
			Detail: "received shard for a message we published",
		}
	}

	// 3. Reject duplicate shards. A well-behaved peer sends each shard
	//    exactly once; duplicates waste bandwidth and could corrupt state.
	if seenShards[unit.ShardIndex] {
		return &ShardValidationError{
			Reason: ReasonDuplicateShard,
			Detail: fmt.Sprintf("already received shard %d", unit.ShardIndex),
		}
	}

	// 4. Verify the sender is either the peer assigned to broadcast this
	//    shard or the publisher itself (who initially distributes all shards).
	//    This prevents a Byzantine node from impersonating another peer's
	//    shard assignment while still allowing the publisher's initial send.
	expectedPeer, err := v.schedule.PeerForShard(unit.Publisher, unit.ShardIndex)
	if err != nil {
		return &ShardValidationError{
			Reason: ReasonScheduleError,
			Detail: fmt.Sprintf(
				"looking up peer for shard %d: %v", unit.ShardIndex, err,
			),
		}
	}
	if sender != expectedPeer && sender != unit.Publisher {
		return &ShardValidationError{
			Reason: ReasonUnexpectedSender,
			Detail: fmt.Sprintf(
				"shard %d should come from %s or publisher %s, got %s",
				unit.ShardIndex, expectedPeer, unit.Publisher, sender,
			),
		}
	}

	// 5. Verify the Merkle inclusion proof. This ensures the shard data
	//    is consistent with the tree root the publisher committed to.
	if !VerifyMerkleProof(
		unit.MerkleRoot, unit.ShardData, uint32(unit.ShardIndex), unit.MerkleProof,
	) {
		return &ShardValidationError{
			Reason: ReasonMerkleProofVerificationFailed,
			Detail: fmt.Sprintf("merkle proof invalid for shard %d", unit.ShardIndex),
		}
	}

	// 6. Verify the publisher's signature over the root. This is the most
	//    expensive check (public-key crypto), so we skip it if we've already
	//    verified the same root from this publisher.
	if !signatureVerified {
		payload := SignPayload(unit.MerkleRoot)
		ok, err := v.verifier.Verify(unit.Publisher, payload, unit.Signature)
		if err != nil {
			return &ShardValidationError{
				Reason: ReasonSignatureVerificationFailed,
				Detail: fmt.Sprintf(
					"verifying signature: %v", err,
				),
			}
		}
		if !ok {
			return &ShardValidationError{
				Reason: ReasonSignatureVerificationFailed,
				Detail: "signature does not match publisher's public key",
			}
		}
	}

	return nil
}
