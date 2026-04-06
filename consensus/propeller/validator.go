package propeller

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/consensus/propeller/merkle"
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
	publisherPubKey crypto.PubKey
	scheduler       *Scheduler

	// track of every shard index received
	receivedShards map[ShardIndex]struct{}
	// Once the validation is done it's stored here, subsequent validation
	// compare against it
	verifiedSignature Signature
}

// todo(rdr): maybe just pass the publisher?
func NewValidator(publisher peer.ID, scheduler *Scheduler) Validator {
	pubKey, err := publisher.ExtractPublicKey()
	// for now we are assuming that extracting a publisher key is always successful
	// and done in constant time
	if err != nil {
		panic(err)
	}
	return Validator{
		publisherPubKey:   pubKey,
		scheduler:         scheduler,
		receivedShards:    make(map[ShardIndex]struct{}, scheduler.NumDataShards()),
		verifiedSignature: nil,
	}
}

func (v *Validator) verifyDataShards(unit *Unit) error {
	if len(unit.ShardData) != 1 {
		return fmt.Errorf(
			"unexpected amount of shards. Expected %d. Received %d",
			1,
			len(unit.ShardData),
		)
	}

	proof := unit.MerkleProof
	root := merkle.Hash(unit.MessageRoot)
	// We marshal to Proto bytes to make the verification language agnostic
	if proof.Verify(&root, unit.ShardData.MarshalProto(), uint32(unit.ShardIndex)) {
		return nil
	}

	return errors.New("data shards verification failed")
}

func (v *Validator) verifySignature(unit *Unit) error {
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

	err := VerifyMessageSignature(
		v.publisherPubKey,
		&unit.MessageRoot,
		&unit.CommitteeID,
		unit.Nonce,
		unit.Signature,
	)
	if err != nil {
		return fmt.Errorf("failed message signature verification: %w", err)
	}

	v.verifiedSignature = unit.Signature
	return nil
}

func (v *Validator) ValidateUnit(unit *Unit, sender peer.ID) error {
	if _, ok := v.receivedShards[unit.ShardIndex]; ok {
		return fmt.Errorf("duplicated shard %d received", unit.ShardIndex)
	}

	err := v.scheduler.ValidateShardOrigin(sender, unit.Publisher, unit.ShardIndex)
	if err != nil {
		return err
	}

	if err = v.verifyDataShards(unit); err != nil {
		return err
	}

	if err = v.verifySignature(unit); err != nil {
		return err
	}

	// Store the verified shard to avoid re-verification
	v.receivedShards[unit.ShardIndex] = struct{}{}

	return nil
}
