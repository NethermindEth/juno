package propeller

import (
	"github.com/NethermindEth/juno/consensus/propeller/merkle"
	pb "github.com/NethermindEth/juno/consensus/propeller/proto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/starknet-io/starknet-p2p-specs/p2p/proto/common"
)

// Unit is the atomic wire message: one erasure-coded shard plus
// the metadata needed for independent verification. Each unit is self-contained
// so a receiver can validate it without any other shards.
type Unit struct {
	CommitteeID CommitteeID  // Which committee this belongs to
	Publisher   peer.ID      // Original message author
	MerkleRoot  MessageRoot  // Merkle root binding all shards together
	MerkleProof merkle.Proof // Merkle inclusion proof for this shard
	Signature   []byte       // Publisher's Ed25519 signature over the root
	ShardIndex  ShardIndex   // This shard's position in the erasure-coded output
	ShardData   []byte       // The actual data fragment
}

func UnitFromProto(protoUnit *pb.PropellerUnit) Unit {
	return Unit{
		CommitteeID: CommitteeID(protoUnit.Channel),
		// todo(rdr): this casting operations seem a bit risky, are they?
		Publisher:  peer.ID(protoUnit.Publisher.Id),
		MerkleRoot: MessageRoot(protoUnit.MerkleRoot.Elements),
		Signature:  protoUnit.Signature,
		ShardIndex: ShardIndex(protoUnit.Index),
		ShardData:  protoUnit.Shard,
	}
}

func (u *Unit) ToProto() *pb.PropellerUnit {
	siblings := make([]*common.Hash256, len(u.MerkleProof.Siblings))
	for i, s := range u.MerkleProof.Siblings {
		siblings[i] = &common.Hash256{Elements: s[:]}
	}

	root := merkle.Hash(u.MerkleRoot)
	return &pb.PropellerUnit{
		Shard: u.ShardData,
		Index: uint64(u.ShardIndex),
		MerkleRoot: &common.Hash256{
			Elements: root[:],
		},
		MerkleProof: &pb.MerkleProof{Siblings: siblings},
		Publisher:   &common.PeerID{Id: []byte(u.Publisher)},
		Signature:   u.Signature,
		Channel:     uint32(u.CommitteeID),
	}
}
