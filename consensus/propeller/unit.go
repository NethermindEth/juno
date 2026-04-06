package propeller

import (
	"errors"
	"time"

	"github.com/NethermindEth/juno/consensus/propeller/merkle"
	pb "github.com/NethermindEth/juno/consensus/propeller/proto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
	"google.golang.org/protobuf/proto"
)

// The actual shard fragmen
type Shard []byte

// Holds the shard fragments carried by the Propeller Unit
type ShardData []Shard

func (sd ShardData) MarshalProto() []byte {
	shards := make([]*pb.Shard, len(sd))
	for i, s := range sd {
		shards[i] = &pb.Shard{Data: s}
	}
	// We ignore the error because this data has already been converted and it is expected
	// to be correct.
	res, _ := proto.Marshal(&pb.ShardsOfPeer{Shards: shards})
	return res
}

// Propeller Unit Signature
type Signature []byte

// Propeller Unit Nonce
type Nonce time.Duration

// Unit is the atomic wire message: one erasure-coded shard plus
// the metadata needed for independent verification. Each unit is self-contained
// so a receiver can validate it without any other shards.
type Unit struct {
	CommitteeID CommitteeID  // Which committee this belongs to
	Publisher   peer.ID      // Original message author
	MessageRoot MessageRoot  // Merkle root binding all shards together
	MerkleProof merkle.Proof // Merkle inclusion proof for this shard
	Signature   Signature    // Publisher's Ed25519 signature over the root
	ShardIndex  ShardIndex   // This shard's position in the erasure-coded output
	ShardData   ShardData    //
	// todo(rdr): calling it nonce because that's what is called on the rust side but
	// time stamp or some other name would be better
	Nonce Nonce // Strictly increasing number, starting from the Unix epoch
}

func UnitFromProto(protoUnit *pb.PropellerUnit) (Unit, error) {
	shards := make(ShardData, len(protoUnit.Shards.GetShards()))
	for i, s := range protoUnit.Shards.GetShards() {
		shards[i] = Shard(s.Data)
	}

	// validate that all shard length is the same
	// todo(rdr): What other validations should I do?
	// todo(rdr): Should I do these validations here?
	shardLen := len(shards[0])
	for i := range shards[1:] {
		if len(shards[i]) != shardLen {
			return Unit{}, errors.New("unit has shards of different length")
		}
	}

	siblings := make([]merkle.Hash, len(protoUnit.MerkleProof.GetSiblings()))
	for i, s := range protoUnit.MerkleProof.GetSiblings() {
		copy(siblings[i][:], s.Elements)
	}

	return Unit{
		CommitteeID: committeeIDFromBytes(protoUnit.CommitteeId.GetElements()),
		Publisher:   peer.ID(protoUnit.Publisher.GetId()),
		MessageRoot: MessageRoot(protoUnit.MerkleRoot.GetElements()),
		MerkleProof: merkle.Proof{Siblings: siblings},
		Signature:   protoUnit.Signature,
		ShardIndex:  ShardIndex(protoUnit.Index),
		ShardData:   shards,
		Nonce:       Nonce(time.Duration(protoUnit.Nonce)),
	}, nil
}

func (u *Unit) ToProto() *pb.PropellerUnit {
	protoShards := make([]*pb.Shard, len(u.ShardData))
	for i, s := range u.ShardData {
		protoShards[i] = &pb.Shard{Data: s}
	}

	siblings := make([]*common.Hash256, len(u.MerkleProof.Siblings))
	for i, s := range u.MerkleProof.Siblings {
		siblings[i] = &common.Hash256{Elements: s[:]}
	}

	root := merkle.Hash(u.MessageRoot)
	return &pb.PropellerUnit{
		Shards:      &pb.ShardsOfPeer{Shards: protoShards},
		Index:       uint64(u.ShardIndex),
		MerkleRoot:  &common.Hash256{Elements: root[:]},
		MerkleProof: &pb.MerkleProof{Siblings: siblings},
		Publisher:   &common.PeerID{Id: []byte(u.Publisher)},
		Signature:   u.Signature,
		CommitteeId: &common.Hash256{Elements: committeeIDToBytes(u.CommitteeID)},
		Nonce:       uint64(u.Nonce),
	}
}

func committeeIDFromBytes(b []byte) CommitteeID {
	var id CommitteeID
	copy(id[:], b)
	return id
}

func committeeIDToBytes(id CommitteeID) []byte {
	return id[:]
}
