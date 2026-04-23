package propeller

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/libp2p/go-libp2p/core/peer"
)

type Stake uint64

// todo(rdr): this is a Peer that belongs to a committee and has a stake. I would like to
// give it a better name
type PeerCommittee struct {
	ID    peer.ID
	Stake Stake
}

// Scheduler represents the tree manager that computes the tree topology on demand for each
// publisher. It holds a deterministic shard-to-peer mapping for a committee.
// Given a sorted set of peers and a publisher, it computes which peer is
// responsible for broadcasting each shard index. The mapping is deterministic
// so that all nodes agree on the assignment without coordination.
//
// The design relies on the invariant that there are N-1 shards for N peers,
// and each non-publisher peer gets exactly one shard. The publisher is "skipped"
// in the sorted peer list when assigning shard indices.
//
// Propeller uses a distributed broadcast approach where:
// - numDataShards = floor((N-1)/3) where N is total number of nodes
// - numDataShards represents both max faulty nodes AND number of data shards
// - numCodingShards = N-1-numDataShards (meaning, the rest)
// - Message is BUILT when numDataShards are received (can reconstruct)
// - Message is RECEIVED when 2*numDataShards shards are received (guarantees gossip property)
// - Each peer broadcasts received shards to all other peers (full mesh)
type Scheduler struct {
	localPeerID      peer.ID
	localPeerIDIndex int
	peers            []PeerCommittee
	numDataShards    int
	numCodingShards  int
}

// NewScheduler creates a schedule from a list of peers. The peers are sorted
// lexicographically by their string representation to ensure all nodes derive
// the same ordering regardless of discovery order.
// Note that `nodes` will be mutated after this function gets called
// todo(rdr): should we return scheduler by reference or by value?
func NewScheduler(
	id peer.ID,
	nodes []PeerCommittee,
) (*Scheduler, error) {
	if len(nodes) < 2 {
		return nil, fmt.Errorf(
			"at least 2 peers are required to form a new committee: %d given",
			len(nodes),
		)
	}

	// todo(rdr): check with function is faster for sorting in our case:
	// `slices.Sort` or `sort.Slice`. Alternative:
	// sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	slices.SortFunc(nodes, func(i, j PeerCommittee) int { return cmp.Compare(i.ID, j.ID) })

	// check that the local peer ID is part of the peer committee
	idIndex, exists := slices.BinarySearchFunc(
		nodes,
		id,
		func(elem PeerCommittee, target peer.ID) int {
			return cmp.Compare(elem.ID, target)
		},
	)
	if !exists {
		return nil, errors.New("the local peer id is not part of the supplied list of peeers")
	}

	// check that there is no duplicated ID in the node list
	for i := range len(nodes) - 1 {
		if nodes[i].ID == nodes[i+1].ID {
			return nil, fmt.Errorf("duplicated ids in the supplied list of peers: %s", nodes[i].ID)
		}
	}

	totalNodes := len(nodes)
	// We guarantee always one data shard for small networks (N = 2 or N = 3)
	numDataShards := max(1, (totalNodes-1)/3)
	// We avoid the possibility of an underflow
	numCodingShards := max(0, totalNodes-1-numDataShards)

	return &Scheduler{
		localPeerID:      id,
		localPeerIDIndex: idIndex,
		peers:            nodes,
		numDataShards:    numDataShards,
		numCodingShards:  numCodingShards,
	}, nil
}

// PeerID returns the Scheduler Peer ID
func (s *Scheduler) PeerID() peer.ID {
	return s.localPeerID
}

// Peers return the Scheduler list of nodes
func (s *Scheduler) Peers() []PeerCommittee {
	return s.peers
}

// DataShards returns the number of data (systematic) shards.
func (s *Scheduler) NumDataShards() int { return s.numDataShards }

// CodingShards returns the number of parity (coding) shards.
func (s *Scheduler) NumCodingShards() int { return s.numCodingShards }

// NumShards returns the total number of shards (data + coding = N-1).
func (s *Scheduler) NumTotalShards() int { return s.numDataShards + s.numCodingShards }

// Minimum (inclusive) amount of shards required to build a message
func (s *Scheduler) BuildThreshold() int { return s.numDataShards }

// Minimum (inclusive) amount of shards required to guarantee a message is received
func (s *Scheduler) ReceiveThreshold() int {
	if len(s.peers) <= 3 {
		return s.BuildThreshold()
	}
	return s.numDataShards * 2
}

func (s *Scheduler) publisherIndex(publisher peer.ID) (int, error) {
	publisherIndex, found := slices.BinarySearchFunc(
		s.peers,
		publisher,
		func(elem PeerCommittee, target peer.ID) int {
			return cmp.Compare(elem.ID, target)
		},
	)
	if !found {
		return -1, fmt.Errorf("publisher with id \"%s\" not found in the peer list", publisher)
	}
	return publisherIndex, nil
}

// PeerForShardIndex returns the peer responsible for broadcasting a given
// shard index to a given publisher. The mapping skips the publisher in the
// sorted list:
//
//	if shardIndex < publisherIndex: peer = peers[shardIndex]
//	if shardIndex >= publisherIndex: peer = peers[shardIndex + 1]
//
// Example with peers [A, B, C, D] and publisher C (index 2):
//
//	shard 0 -> A, shard 1 -> B, shard 2 -> D
func (s *Scheduler) PeerForShardIndex(
	publisher peer.ID, shardIndex ShardIndex,
) (peer.ID, error) {
	if int(shardIndex) >= s.NumTotalShards() {
		return "", fmt.Errorf(
			"shard index %d out of range [0, %d)", shardIndex, s.NumTotalShards(),
		)
	}

	pubIdx, err := s.publisherIndex(publisher)
	if err != nil {
		return "", err
	}

	// Skip the publisher's position.
	peerIdx := int(shardIndex)
	if peerIdx >= pubIdx {
		peerIdx++
	}

	return s.peers[peerIdx].ID, nil
}

// ShardIndexForPublisher returns the shard index that shceduler is responsible for
// broadcasting for a given publisher. This is the inverse of PeerForShard:
//
//	if localPeerIndex < publisherIndex: shard = localPeerIndex
//	if localPeerIndex > publisherIndex: shard = localPeerIndex - 1
//
// Returns an error if Scheduler's peer is the publisher (publishers don't have an
// assigned shard) or if the publisher is not in the list.
func (s *Scheduler) ShardIndexForPublisher(
	publisher peer.ID,
) (ShardIndex, error) {
	if s.localPeerID == publisher {
		return 0, fmt.Errorf(
			"scheduler peer is the same as the publisher and has no assigned shard: %s",
			publisher,
		)
	}

	pubIdx, err := s.publisherIndex(publisher)
	if err != nil {
		return 0, fmt.Errorf("couldn't locate shard index for publisher: %w", err)
	}

	shardIdx := s.localPeerIDIndex
	if s.localPeerIDIndex >= pubIdx {
		shardIdx = s.localPeerIDIndex - 1
	}

	return ShardIndex(shardIdx), nil
}

// ValidateShardOrigin verifies that a shard unit was received from the expected sender.
// The sender has to be either the publisher for direct shards or a designated
// broadcaster for the given shard index.
// todo(rdr): This implementation should probably be part of `UnitValidator`
func (s *Scheduler) ValidateShardOrigin(
	sender peer.ID,
	publisher peer.ID,
	shardIndex ShardIndex,
) error {
	if sender == s.localPeerID {
		return fmt.Errorf("self sending message from %s", sender)
	}
	if publisher == s.localPeerID {
		return fmt.Errorf("self published shard was sent back by %s", sender)
	}

	expectedBroadcaster, err := s.PeerForShardIndex(publisher, shardIndex)
	if err != nil {
		return fmt.Errorf(
			"couldn't validate publisher %s with shard %d: %w",
			publisher,
			shardIndex,
			err,
		)
	}

	validDirectShard := expectedBroadcaster == s.localPeerID && sender == publisher
	if validDirectShard {
		return nil
	}

	validBroadcastShard := expectedBroadcaster == sender
	if validBroadcastShard {
		return nil
	}

	return fmt.Errorf(
		"received shard index %d from unexpected sender %s",
		shardIndex,
		sender,
	)
}

// BroadcastTargets returns all peers whom to broadcast to, in shard-index order.
// The i-th element of the returned slice is the peer responsible for shard i.
func (s *Scheduler) BroadcastTargets() []peer.ID {
	// todo(rdr): I would like to not use `append` and index directly instead (it's faster)
	targets := make([]peer.ID, 0, s.NumTotalShards())
	for i, p := range s.peers {
		if i == s.localPeerIDIndex {
			continue
		}
		targets = append(targets, p.ID)
	}
	return targets
}
