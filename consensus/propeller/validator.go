package propeller

import (
	"bytes"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// todo(rdr): A validator lifetime is attached to a `subprocessor`. A `subprocessor` is attached
// to a message key field. This logic is handled by a `Processor`. This means that a validator will // always be given units that have the same committeeID, publisher, messageRoot and Nonce (the
// current fields of a `messageKey`). Does it makes sense for the validator to also hold a copy
// of this. Is there a way of testing this invariant – where a validator only sees the same
// fields. I need to add a test for that invariant

// Validates all the incoming units / shards given a committee and the publisher
type Validator struct {
	// Required fields to perform the validation
	// or not. Check if I can delete them
	// committeeID CommitteeID
	// publisher       peer.ID
	// messageRoot MessageRoot
	// nonce       Nonce
	// ----------------------------------------

	publisherPubKey crypto.PubKey
	scheduler       *Scheduler

	// Once the validation is done it's stored here, subsequent runs
	// compare against it
	verifiedSignature Signature

	// track of every shard index received
	receivedShards map[ShardIndex]struct{}
}

// todo(rdr): maybe just pass the publisher?
func NewValidator(key *messageKey, scheduler *Scheduler) Validator {
	pubKey, err := key.Publisher.ExtractPublicKey()
	// for now we are assuming that extracting a publisher key is always successful
	// and done in constant time
	if err != nil {
		panic(err)
	}
	return Validator{
		// committeeID:     key.CommitteeID,
		// publisher: key.Publisher,
		// messageRoot:     key.Root,
		// nonce:           key.Nonce,
		publisherPubKey: pubKey,
		scheduler:       scheduler,
		receivedShards:  make(map[ShardIndex]struct{}, scheduler.NumDataShards()),
	}
}

func (v *Validator) verify(unit *Unit) error {
	if v.verifiedSignature != nil {
		if bytes.Equal(v.verifiedSignature, unit.Signature) {
			return nil
		}
		// todo(rdr): make sure this error is readable
		return fmt.Errorf(
			"signature missmatch. Expected: %v. Received %v",
			v.verifiedSignature,
			unit.Signature,
		)
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

	// todo(rdr): by storing a field of unit.Signature am I forcing the whole `unit` to
	// continue to exist on the heap, or can the remaining fields be cleaned. Probably the
	// latter.
	v.verifiedSignature = unit.Signature
	return nil
}

func (v *Validator) ValidateUnit(unit *Unit, sender peer.ID) error {
	if _, ok := v.receivedShards[unit.ShardIndex]; ok {
		return fmt.Errorf("duplicated shard %d received", unit.ShardIndex)
	}

	// We can use `unit.Publisher` because it is part of messageKey and hence
	// this validator wouldn't be used otherwise
	err := v.scheduler.ValidateShardOrigin(sender, unit.Publisher, unit.ShardIndex)
	if err != nil {
		return err
	}

	if err = v.verify(unit); err != nil {
		return err
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
