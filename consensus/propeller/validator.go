package propeller

import (
	"bytes"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type verified struct {
	verified  bool
	signature []byte
	nonce     time.Duration
}

// Retuns if
func (v *verified) Verify(unit *Unit, pubKey crypto.PubKey) error {
	if v.verified {
		if bytes.Equal(v.signature, unit.Signature) && v.nonce == unit.Nonce {
			return nil
		}
		// todo(rdr): add error information. Perhaps build an error type?
		return fmt.Errorf("unit signature or nonce missmatch")
	}

	err := verifyMessageIDSignature(
		unit.CommitteeID,
		unit.MerkleRoot,
		unit.Signature,
		unit.Nonce,
		pubKey,
	)
	if err != nil {
		// add error information
		return err
	}

	*v = verified{
		verified: true,
		// todo(rdr): by storing a field of unit.Signature am I forcing the whole `unit` to
		// continue to exist on the heap, or the remaining fields can be cleaned. Probably the
		// latter.
		signature: unit.Signature,
		nonce:     unit.Nonce,
	}

	return nil
}

func verifyMessageIDSignature(
	committeeID CommitteeID,
	root MessageRoot,
	signature []byte,
	nonce time.Duration,
	publisherPubKey crypto.PubKey,
) error {
	panic("not yet implemented")
}

// Validates all the incoming units / shards given a committee and the publisher
type Validator struct {
	// Required fields to perform the validation
	committeeID     CommitteeID
	publisher       peer.ID
	publisherPubKey crypto.PubKey
	messageRoot     MessageRoot
	scheduler       *Scheduler

	// Once the validation is done it's stored here, subsequent runs
	// compare against it
	verified verified

	// track of every shard index received
	receivedShards map[ShardIndex]struct{}
}

func NewValidator(key *messageKey, scheduler *Scheduler) Validator {
	return Validator{
		committeeID:     key.CommitteeID,
		publisher:       key.Publisher,
		messageRoot:     key.Root,
		publisherPubKey: nil, // todo(rdr): nil for now, need to think how to pass this one
		scheduler:       scheduler,
		receivedShards:  make(map[ShardIndex]struct{}, scheduler.NumDataShards()),
	}
}

func (v *Validator) ValidateUnit(unit *Unit, sender peer.ID) error {
	// todo(rdr): Do I need to check that comitteeID, publisher and messageRoot to be the
	// same as the one being hold by the validator, or is that infered from the signature being
	// correct or wrong?

	if _, ok := v.receivedShards[unit.ShardIndex]; ok {
		return fmt.Errorf("duplicated shard %d received", unit.ShardIndex)
	}

	err := v.scheduler.ValidateShardOrigin(sender, v.publisher, unit.ShardIndex)
	if err != nil {
	}

	if err = v.verified.Verify(unit, v.publisherPubKey); err != nil {
		return nil
	}

	// Cache the verified shard to avoid re-verification
	v.receivedShards[unit.ShardIndex] = struct{}{}

	return nil
}
