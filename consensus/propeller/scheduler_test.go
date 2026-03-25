// todo(rdr): make it propeller_test
package propeller

import (
	"cmp"
	"math/rand"
	"slices"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPeers creates N deterministic peer IDs that sort in alphabetical order.
// Also returns a local peer ID choosen at random fromt the list
func testPeers(t *testing.T, names ...string) (peer.ID, []PeerCommittee) {
	t.Helper()

	peers := make([]PeerCommittee, len(names))
	for i, n := range names {
		peers[i] = PeerCommittee{
			ID:    peer.ID(n),
			Stake: Stake(rand.Uint32()),
		}
	}
	// note(rdr): should we make the random generation deterministic?
	localPeer := peers[rand.Int()%len(peers)].ID

	return localPeer, peers
}

func TestSchedule_Thresholds(t *testing.T) {
	tests := []struct {
		name             string
		n                int
		numDataShards    int
		numCodingShards  int
		numShards        int
		buildThreshold   int
		receiveThreshold int
	}{
		{
			name:            "N=1 (solo node, no shards)",
			n:               1,
			numDataShards:   0,
			numCodingShards: 0,
			numShards:       0,
		},
		{
			name:            "N=2",
			n:               2,
			numDataShards:   1, // max of 1 and (1/3) yields 1
			numCodingShards: 0, // 1 minus 1 yields 0
			numShards:       1,
		},
		{
			name:            "N=3",
			n:               3,
			numDataShards:   1, // max of 1 and (2/3) yields 1
			numCodingShards: 1, // 2 minus 1 yields 1
			numShards:       2,
		},
		{
			name:            "N=4",
			n:               4,
			numDataShards:   1, // 3/3 = 1
			numCodingShards: 2, // 3 - 1 = 2
			numShards:       3,
		},
		{
			name:            "N=5",
			n:               5,
			numDataShards:   1, // 4/3 = 1
			numCodingShards: 3, // 4 - 1 = 3
			numShards:       4,
		},
		{
			name:            "N=7",
			n:               7,
			numDataShards:   2, // 6/3 = 2
			numCodingShards: 4, // 6 - 2 = 4
			numShards:       6,
		},
		{
			name:            "N=10",
			n:               10,
			numDataShards:   3, // 9/3 = 3
			numCodingShards: 6, // 9 - 3 = 6
			numShards:       9,
		},
		{
			name:            "N=31",
			n:               31,
			numDataShards:   10, // 30/3 = 10
			numCodingShards: 20, // 30 - 10 = 20
			numShards:       30,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := make([]string, tc.n)
			for i := range tc.n {
				names[i] = string(rune('A' + i))
			}
			localPeer, peers := testPeers(t, names...)

			s, err := NewScheduler(localPeer, peers)
			require.NoError(t, err)

			assert.Equal(t, tc.numDataShards, s.DataShards())
			assert.Equal(t, tc.numCodingShards, s.CodingShards())
			assert.Equal(t, tc.numShards, s.NumShards())
		})
	}
}

func TestSchedule_Sorting(t *testing.T) {
	// Peers provided out of order should be sorted.
	localPeer, peers := testPeers(t, "D", "B", "A", "C")

	sortedPeers := make([]PeerCommittee, 0, len(peers))
	copy(sortedPeers, peers)
	slices.SortFunc(sortedPeers, func(a, b PeerCommittee) int {
		return cmp.Compare(a.ID, b.ID)
	})

	s, err := NewScheduler(localPeer, peers)
	require.NoError(t, err)

	assert.Equal(t, sortedPeers, s.Peers())
}

func TestSchedule_PeerForShard_SpecExample(t *testing.T) {
	// From the specification: peers [A, B, C, D], publisher = C (index 2).
	// Shard 0 -> A, Shard 1 -> B, Shard 2 -> D
	localPeer, peers := testPeers(t, "A", "B", "C", "D")

	s, err := NewScheduler(localPeer, peers)
	require.NoError(t, err)

	publisher := peer.ID("C")

	tests := []struct {
		shardIndex ShardIndex
		expected   peer.ID
	}{
		{0, peer.ID("A")},
		{1, peer.ID("B")},
		{2, peer.ID("D")},
	}

	for _, tc := range tests {
		got, err := s.PeerForShard(publisher, tc.shardIndex)
		require.NoError(t, err)
		assert.Equal(t, tc.expected, got, "shard %d", tc.shardIndex)
	}
}

func TestSchedule_PeerForShard_PublisherFirst(t *testing.T) {
	// Publisher is the first peer in sorted order.
	peers := testPeers("A", "B", "C", "D")
	s := NewScheduler(peers)
	publisher := peer.ID("A")

	// Shard 0 -> B, Shard 1 -> C, Shard 2 -> D
	expected := testPeers("B", "C", "D")
	for i, exp := range expected {
		got, err := s.PeerForShard(publisher, ShardIndex(i))
		require.NoError(t, err)
		assert.Equal(t, exp, got)
	}
}

func TestSchedule_PeerForShard_PublisherLast(t *testing.T) {
	// Publisher is the last peer in sorted order.
	peers := testPeers("A", "B", "C", "D")
	s := NewScheduler(peers)
	publisher := peer.ID("D")

	// Shard 0 -> A, Shard 1 -> B, Shard 2 -> C
	expected := testPeers("A", "B", "C")
	for i, exp := range expected {
		got, err := s.PeerForShard(publisher, ShardIndex(i))
		require.NoError(t, err)
		assert.Equal(t, exp, got)
	}
}

func TestSchedule_PeerForShard_Errors(t *testing.T) {
	peers := testPeers("A", "B", "C")
	s := NewScheduler(peers)

	// Publisher not in list.
	_, err := s.PeerForShard(peer.ID("Z"), 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Shard index out of range.
	_, err = s.PeerForShard(peer.ID("A"), ShardIndex(s.NumShards()))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "out of range")
}

func TestSchedule_ShardForPeer_SpecExample(t *testing.T) {
	// Inverse of PeerForShard: peers [A,B,C,D], publisher=C.
	// A -> shard 0, B -> shard 1, D -> shard 2
	peers := testPeers("A", "B", "C", "D")
	s := NewScheduler(peers)
	publisher := peer.ID("C")

	tests := []struct {
		localPeer peer.ID
		expected  ShardIndex
	}{
		{peer.ID("A"), 0},
		{peer.ID("B"), 1},
		{peer.ID("D"), 2},
	}

	for _, tc := range tests {
		got, err := s.ShardForPeer(publisher, tc.localPeer)
		require.NoError(t, err)
		assert.Equal(t, tc.expected, got, "peer %s", tc.localPeer)
	}
}

func TestSchedule_ShardForPeer_PublisherError(t *testing.T) {
	peers := testPeers("A", "B", "C")
	s := NewScheduler(peers)

	// The publisher itself has no assigned shard.
	_, err := s.ShardForPeer(peer.ID("B"), peer.ID("B"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is the publisher")
}

func TestSchedule_ShardForPeer_NotFound(t *testing.T) {
	peers := testPeers("A", "B", "C")
	s := NewScheduler(peers)

	_, err := s.ShardForPeer(peer.ID("A"), peer.ID("Z"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSchedule_PeerForShardAndShardForPeer_AreInverses(t *testing.T) {
	// For every publisher, verify that PeerForShard and ShardForPeer
	// are consistent inverses.
	peers := testPeers("A", "B", "C", "D", "E")
	s := NewScheduler(peers)

	for _, publisher := range s.Peers() {
		for shardIdx := range s.NumShards() {
			p, err := s.PeerForShard(publisher, ShardIndex(shardIdx))
			require.NoError(t, err)

			// The reverse: given that peer, find its shard index.
			gotShard, err := s.ShardForPeer(publisher, p)
			require.NoError(t, err)
			assert.Equal(t, ShardIndex(shardIdx), gotShard,
				"publisher=%s, shard=%d, peer=%s", publisher, shardIdx, p)
		}
	}
}

func TestSchedule_BroadcastTargets(t *testing.T) {
	peers := testPeers("A", "B", "C", "D")
	s := NewScheduler(peers)

	targets, err := s.BroadcastTargets(peer.ID("C"))
	require.NoError(t, err)

	// Should be all peers except C, in shard order.
	expected := testPeers("A", "B", "D")
	assert.Equal(t, expected, targets)
}

func TestSchedule_BroadcastTargets_PublisherNotFound(t *testing.T) {
	peers := testPeers("A", "B")
	s := NewScheduler(peers)

	_, err := s.BroadcastTargets(peer.ID("Z"))
	assert.Error(t, err)
}
