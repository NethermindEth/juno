package propeller

import (
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/consensus/propeller/merkle"
	"github.com/NethermindEth/juno/consensus/propeller/reedsolomon"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

// CreatePropellerUnits creates the PropellerUnits for publishing
// todo(rdr): maybe call it create message for sharing or somth like that
func CreatePropellerUnits(
	privKey crypto.PrivKey,
	committeeID *CommitteeID,
	nonce Nonce,
	message []byte,
	numDataShards,
	parity int,
) ([]Unit, error) {
	publisherID, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("getting publisher id from private key: %w", publisherID)
	}

	paddedMessage := PadMessage(message, numDataShards)
	encodedMessage, err := reedsolomon.EncodeData(paddedMessage, numDataShards, parity)
	if err != nil {
		return nil, fmt.Errorf("encoding the message: %w", err)
	}

	merkleRoot, merkleTree := merkle.New(encodedMessage)
	messageRoot := MessageRoot(merkleRoot)

	signature, err := SignMessage(privKey, &messageRoot, committeeID, nonce)
	if err != nil {
		return nil, err
	}

	units := make([]Unit, len(encodedMessage))
	for i, shard := range encodedMessage {
		merkleProof := merkleTree[i]

		units[i] = Unit{
			CommitteeID: *committeeID,
			Publisher:   publisherID,
			MessageRoot: messageRoot,
			MerkleProof: merkleProof,
			Signature:   signature,
			ShardIndex:  ShardIndex(i),
			// todo(rdr): assigning one shard per unit until multi shard algo per unit
			// is clear to me
			ShardData: []Shard{shard},
		}
	}
	return units, nil
}

// ConstructMessageFromUnits receives Propeller units, recovers any missing data and returns
// the fully verified message, together with the corresponding  shard data and merkle proof.
func ConstructMessageFromUnits(
	units []*Unit,
	localShardIndex ShardIndex,
	numDataShards int,
	parity int,
) ([]byte, ShardData, merkle.Proof, error) {
	if len(units) == 0 {
		return nil, nil, merkle.Proof{}, errors.New("no propeller units to decode")
	}

	shards := make([][]byte, len(units))
	for i := range shards {
		if units[i] != nil {
			// todo(rdr): we are assuming that every unit only carries one shard data for now
			// Not sure how the matrix is built when unit carries more than one
			shards[i] = units[i].ShardData[0]
		}
	}

	shards, err := reedsolomon.RecoverData(shards, numDataShards, parity)
	if err != nil {
		return nil, nil, merkle.Proof{}, fmt.Errorf("recovering shards data: %w", err)
	}
	shardSize := len(shards[0])
	for i := range numDataShards {
		if shards[i] != nil && len(shards[i]) != shardSize {
			return nil, nil, merkle.Proof{}, fmt.Errorf(
				"missmatch on shard size: %d (at index 0) vs %d (at index %d)",
				len(shards[0]),
				len(shards[i]),
				i,
			)
		}
	}

	merkleRoot, merkleTree := merkle.New(shards)

	messageRoot := units[0].MessageRoot
	expectedRoot := MessageRoot(merkleRoot)
	if messageRoot != expectedRoot {
		// todo(rdr): probably need to write string methods for the MessageRoot type
		return nil, nil, merkle.Proof{}, fmt.Errorf(
			"wrong message root hash. Expected %s but got %s",
			&expectedRoot,
			&messageRoot,
		)
	}

	paddedMessage := make([]byte, len(shards[0])*len(shards))
	for i := range shards {
		copy(paddedMessage[i*shardSize:], shards[i])
	}
	message, err := UnpadMessage(paddedMessage)
	if err != nil {
		return nil, nil, merkle.Proof{}, fmt.Errorf("unpadding reconstructed message: %w", err)
	}

	// todo(rdr): only one for now, but there can be more.TBD how that works
	localShard := []Shard{
		shards[localShardIndex],
	}
	localProof := merkleTree[localShardIndex]

	return message, localShard, localProof, nil
}
