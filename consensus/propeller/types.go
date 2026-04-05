// Package propeller implements an erasure-coding based message broadcast protocol
// for Byzantine fault-tolerant consensus. A publisher splits a message into shards,
// erasure-encodes them via Reed-Solomon, and distributes one shard per peer.
// Any peer can reconstruct the full message from a threshold number of shards,
// then forwards its own assigned shard to all others.
//
// The protocol tolerates up to f = floor((N-1)/3) Byzantine faulty nodes.
package propeller

import (
	"fmt"
	"time"

	"github.com/NethermindEth/juno/consensus/propeller/merkle"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// CommitteeID identifies a committee or logical broadcast group. Multiple committees
// can operate concurrently within the same engine, each with its own peer set.
type CommitteeID [4]uint64

// ShardIndex is the position of a shard within the erasure-coded output.
// Valid range is [0, N-2] where N is the total number of peers.
type ShardIndex uint32

// MessageRoot is the SHA-256 Merkle root over all shard leaves. It uniquely
// identifies a message and is signed by the publisher to bind authenticity.
type MessageRoot merkle.Hash

// Config holds tunable parameters for the propeller engine. Sensible defaults
// are provided by DefaultConfig().
type Config struct {
	// StaleMessageTimeout is how long the engine waits for a message to
	// reach the receive threshold before giving up. This prevents memory
	// leaks from partially-received messages that will never complete
	// (e.g., due to a crashed publisher or network partition).
	StaleMessageTimeout time.Duration

	// StreamProtocol is the libp2p protocol identifier used for direct
	// shard transfers between peers.
	StreamProtocol protocol.ID

	// MaxWireMessageSize caps the size of a single serialised PropellerUnit
	// on the wire. Units exceeding this are rejected to prevent memory
	// exhaustion from malicious peers.
	MaxWireMessageSize int
}

// DefaultConfig returns production-ready defaults.
func DefaultConfig() Config {
	return Config{
		StaleMessageTimeout: 120 * time.Second,
		StreamProtocol:      "/propeller/0.1.0",
		MaxWireMessageSize:  1 << 20, // 1 MiB
	}
}

// ---------------------------------------------------------------------------
// Events: structured outputs from the engine to the application layer.
// Each event is emitted at most once per message lifecycle.
// ---------------------------------------------------------------------------

// EventMessageReceived signals that a message has been fully reconstructed
// and enough shards have been forwarded to guarantee delivery to all honest
// nodes. The application can safely process the contained message bytes.
type EventMessageReceived struct {
	Publisher peer.ID
	Root      MessageRoot
	Message   []byte
}

// EventReconstructionFailed signals that Reed-Solomon reconstruction or
// post-reconstruction verification failed. This typically indicates Byzantine
// behaviour from the publisher (e.g., inconsistent shards).
type EventReconstructionFailed struct {
	Root      MessageRoot
	Publisher peer.ID
	Err       error
}

// EventShardPublishFailed signals that the local node failed to encode or
// distribute shards when acting as publisher.
type EventShardPublishFailed struct {
	Err error
}

// EventShardSendFailed signals that sending a single shard to a specific
// peer failed. The engine continues sending to other peers; this is
// informational for monitoring.
type EventShardSendFailed struct {
	From peer.ID
	To   peer.ID
	Err  error
}

// EventShardValidationFailed signals that an incoming shard was rejected
// during validation. This may indicate Byzantine behaviour from the sender
// or publisher.
type EventShardValidationFailed struct {
	Sender           peer.ID
	ClaimedRoot      MessageRoot
	ClaimedPublisher peer.ID
	Err              error
}

// EventMessageTimeout signals that a message did not reach the receive
// threshold before the stale message timeout elapsed. The engine cleans
// up state for this message.
type EventMessageTimeout struct {
	Channel   CommitteeID
	Publisher peer.ID
	Root      MessageRoot
}

// reasonUnknown is the string representation for unrecognised enum values.
// Extracted as a constant to satisfy goconst.
const reasonUnknown = "unknown"

// ---------------------------------------------------------------------------
// Error types: structured errors for each failure domain.
// Using typed errors rather than sentinel values lets callers inspect the
// specific failure reason programmatically.
// ---------------------------------------------------------------------------

// ShardValidationReason enumerates the specific causes of shard rejection.
type ShardValidationReason int

const (
	// ReasonSelfSending means a peer sent us a unit claiming to be from us.
	ReasonSelfSending ShardValidationReason = iota
	// ReasonReceivedSelfPublishedShard means we received a shard for a
	// message we published ourselves -- we already have all shards.
	ReasonReceivedSelfPublishedShard
	// ReasonDuplicateShard means we already have a shard at this index
	// for this message.
	ReasonDuplicateShard
	// ReasonUnexpectedSender means the sender is not the peer assigned
	// to broadcast this shard index.
	ReasonUnexpectedSender
	// ReasonSignatureVerificationFailed means the publisher's signature
	// over the Merkle root did not verify.
	ReasonSignatureVerificationFailed
	// ReasonMerkleProofVerificationFailed means the Merkle inclusion
	// proof for this shard is invalid.
	ReasonMerkleProofVerificationFailed
	// ReasonScheduleError means the shard-to-peer mapping lookup failed
	// (e.g., publisher not in the channel's peer set).
	ReasonScheduleError
)

func (r ShardValidationReason) String() string {
	switch r {
	case ReasonSelfSending:
		return "self_sending"
	case ReasonReceivedSelfPublishedShard:
		return "received_self_published_shard"
	case ReasonDuplicateShard:
		return "duplicate_shard"
	case ReasonUnexpectedSender:
		return "unexpected_sender"
	case ReasonSignatureVerificationFailed:
		return "signature_verification_failed"
	case ReasonMerkleProofVerificationFailed:
		return "merkle_proof_verification_failed"
	case ReasonScheduleError:
		return "schedule_error"
	default:
		return reasonUnknown
	}
}

// ShardValidationError is returned when an incoming PropellerUnit fails
// validation. The Reason field allows programmatic inspection; the Detail
// field carries human-readable context.
type ShardValidationError struct {
	Reason ShardValidationReason
	Detail string
}

func (e *ShardValidationError) Error() string {
	return fmt.Sprintf("shard validation failed (%s): %s", e.Reason, e.Detail)
}

// ReconstructionReason enumerates the specific causes of reconstruction failure.
type ReconstructionReason int

const (
	// ReasonErasureReconstructionFailed means Reed-Solomon decoding failed,
	// likely because too many shards are missing or corrupted.
	ReasonErasureReconstructionFailed ReconstructionReason = iota
	// ReasonMismatchedMessageRoot means the Merkle root computed from the
	// reconstructed shards does not match the claimed root. This indicates
	// Byzantine behaviour from the publisher.
	ReasonMismatchedMessageRoot
	// ReasonUnequalShardLengths means shards have inconsistent lengths,
	// which violates Reed-Solomon's equal-length requirement.
	ReasonUnequalShardLengths
	// ReasonMessagePaddingError means the varint length prefix in the
	// unpadded message is malformed or points beyond the data.
	ReasonMessagePaddingError
)

func (r ReconstructionReason) String() string {
	switch r {
	case ReasonErasureReconstructionFailed:
		return "erasure_reconstruction_failed"
	case ReasonMismatchedMessageRoot:
		return "mismatched_message_root"
	case ReasonUnequalShardLengths:
		return "unequal_shard_lengths"
	case ReasonMessagePaddingError:
		return "message_padding_error"
	default:
		return reasonUnknown
	}
}

// ReconstructionError is returned when message reconstruction fails after
// collecting enough shards.
type ReconstructionError struct {
	Reason ReconstructionReason
	Detail string
}

func (e *ReconstructionError) Error() string {
	return fmt.Sprintf("reconstruction failed (%s): %s", e.Reason, e.Detail)
}

// ShardPublishReason enumerates the specific causes of publish failure.
type ShardPublishReason int

const (
	// ReasonLocalPeerNotInChannel means the local peer is not a member
	// of the channel it is trying to broadcast on.
	ReasonLocalPeerNotInChannel ShardPublishReason = iota
	// ReasonInvalidDataSize means the message is too large to encode.
	ReasonInvalidDataSize
	// ReasonSigningFailed means the local private key failed to sign.
	ReasonSigningFailed
	// ReasonEncodingFailed means Reed-Solomon encoding failed.
	ReasonEncodingFailed
	// ReasonNotConnectedToPeer means we have no open connection to a
	// target peer.
	ReasonNotConnectedToPeer
	// ReasonChannelNotRegistered means the channel has not been registered
	// with the engine.
	ReasonChannelNotRegistered
	// ReasonBroadcastFailed means the broadcast operation failed for an
	// unspecified reason.
	ReasonBroadcastFailed
)

func (r ShardPublishReason) String() string {
	switch r {
	case ReasonLocalPeerNotInChannel:
		return "local_peer_not_in_channel"
	case ReasonInvalidDataSize:
		return "invalid_data_size"
	case ReasonSigningFailed:
		return "signing_failed"
	case ReasonEncodingFailed:
		return "encoding_failed"
	case ReasonNotConnectedToPeer:
		return "not_connected_to_peer"
	case ReasonChannelNotRegistered:
		return "channel_not_registered"
	case ReasonBroadcastFailed:
		return "broadcast_failed"
	default:
		return reasonUnknown
	}
}

// todo(rdr): check if we want to do this. I think it is better not, unless necessary
// ShardPublishError is returned when the local node fails to publish shards.
type ShardPublishError struct {
	Reason ShardPublishReason
	Detail string
}

func (e *ShardPublishError) Error() string {
	return fmt.Sprintf("shard publish failed (%s): %s", e.Reason, e.Detail)
}
