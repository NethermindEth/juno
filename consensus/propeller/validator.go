package propeller

import (
	"bytes"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type verificationResult struct {
	done      bool
	signature Signature
	nonce     Nonce
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
	verification verificationResult

	// track of every shard index received
	receivedShards map[ShardIndex]struct{}
}

func NewValidator(key *messageKey, scheduler *Scheduler) Validator {
	pubKey, err := key.Publisher.ExtractPublicKey()
	// for now we are assuming that extracting a publisher key is always successful
	// and done in constant time
	if err != nil {
		panic(err)
	}
	return Validator{
		committeeID:     key.CommitteeID,
		publisher:       key.Publisher,
		messageRoot:     key.Root,
		publisherPubKey: pubKey, // todo(rdr): nil for now, need to think how to pass this one
		scheduler:       scheduler,
		receivedShards:  make(map[ShardIndex]struct{}, scheduler.NumDataShards()),
	}
}

func (v *Validator) verifyKeyFields(unit *Unit) error {
	if unit.CommitteeID != v.committeeID {
		return fmt.Errorf(
			"different committe id. Expected: %s. Received: %s", unit.CommitteeID, v.committeeID,
		)
	}
	if unit.Publisher != v.publisher {
		return fmt.Errorf(
			"different publisher. Expected: %s. Received: %s", unit.Publisher, v.publisher,
		)
	}
	if unit.MessageRoot != v.messageRoot {
		return fmt.Errorf(
			"different message root. Expected: %s. Received: %s", unit.MessageRoot, v.messageRoot,
		)
	}

	return nil
}

func (v *Validator) verify(unit *Unit) error {
	// todo(rdr): Here we are verifying everything but the data, do we verify the data
	// at some point. What happens if a Peer has all the data correct except the shard data
	// for example? Is that an attack vector?
	// Something fails at some point and the publisher gets slashed?
	if v.verification.done {
		verificationMatch := bytes.Equal(v.verification.signature, unit.Signature) &&
			v.verification.nonce == unit.Nonce
		if verificationMatch {
			return nil
		}
		// todo(rdr): add error information. Perhaps build an error type?
		return fmt.Errorf("unit signature or nonce missmatch")
	}

	err := verifyMessageIDSignature(
		unit.CommitteeID,
		unit.MessageRoot,
		unit.Signature,
		unit.Nonce,
		v.publisherPubKey,
	)
	if err != nil {
		// add error information
		return err
	}

	v.verification = verificationResult{
		done: true,
		// todo(rdr): by storing a field of unit.Signature am I forcing the whole `unit` to
		// continue to exist on the heap, or can the remaining fields be cleaned. Probably the
		// latter.
		signature: unit.Signature,
		nonce:     unit.Nonce,
	}

	return nil
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

	if err = v.verify(unit); err != nil {
		return nil
	}

	// Cache the verified shard to avoid re-verification
	v.receivedShards[unit.ShardIndex] = struct{}{}

	return nil
}

func verifyMessageIDSignature(
	committeeID CommitteeID,
	root MessageRoot,
	signature Signature,
	nonce Nonce,
	publisherPubKey crypto.PubKey,
) error {
	panic("not yet implemented")
}
