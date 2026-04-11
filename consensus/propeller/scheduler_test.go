package propeller

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPeers creates PeerCommittee entries from the given names.
// Each test chooses its own local peer explicitly.
func testPeers(t *testing.T, names ...string) []PeerCommittee {
	t.Helper()
	peers := make([]PeerCommittee, len(names))
	for i, n := range names {
		peers[i] = PeerCommittee{ID: peer.ID(n), Stake: 1}
	}
	return peers
}

func TestScheduler_NewScheduler_Validation(t *testing.T) {
	t.Run("fewer than 2 peers", func(t *testing.T) {
		peers := testPeers(t, "A")
		_, err := NewScheduler(peer.ID("A"), peers)
		assert.Error(t, err)
	})

	t.Run("local peer not in list", func(t *testing.T) {
		peers := testPeers(t, "A", "B", "C")
		_, err := NewScheduler(peer.ID("Z"), peers)
		assert.Error(t, err)
	})

	t.Run("duplicate peers", func(t *testing.T) {
		peers := testPeers(t, "A", "B", "B")
		_, err := NewScheduler(peer.ID("A"), peers)
		assert.Error(t, err)
	})

	t.Run("valid construction", func(t *testing.T) {
		peers := testPeers(t, "A", "B", "C")
		s, err := NewScheduler(peer.ID("B"), peers)
		require.NoError(t, err)
		assert.Equal(t, peer.ID("B"), s.PeerID())
	})
}

func TestScheduler_ShardCounts(t *testing.T) {
	tests := []struct {
		name             string
		n                int
		numDataShards    int
		numCodingShards  int
		numTotalShards   int
		buildThreshold   int
		receiveThreshold int
	}{
		{
			name:             "N=2",
			n:                2,
			numDataShards:    1,
			numCodingShards:  0,
			numTotalShards:   1,
			buildThreshold:   1,
			receiveThreshold: 1, // len<=3, falls back to buildThreshold
		},
		{
			name:             "N=3",
			n:                3,
			numDataShards:    1,
			numCodingShards:  1,
			numTotalShards:   2,
			buildThreshold:   1,
			receiveThreshold: 1, // len<=3, falls back to buildThreshold
		},
		{
			name:             "N=4",
			n:                4,
			numDataShards:    1,
			numCodingShards:  2,
			numTotalShards:   3,
			buildThreshold:   1,
			receiveThreshold: 2,
		},
		{
			name:             "N=5",
			n:                5,
			numDataShards:    1,
			numCodingShards:  3,
			numTotalShards:   4,
			buildThreshold:   1,
			receiveThreshold: 2,
		},
		{
			name:             "N=7",
			n:                7,
			numDataShards:    2,
			numCodingShards:  4,
			numTotalShards:   6,
			buildThreshold:   2,
			receiveThreshold: 4,
		},
		{
			name:             "N=10",
			n:                10,
			numDataShards:    3,
			numCodingShards:  6,
			numTotalShards:   9,
			buildThreshold:   3,
			receiveThreshold: 6,
		},
		{
			name:             "N=31",
			n:                31,
			numDataShards:    10,
			numCodingShards:  20,
			numTotalShards:   30,
			buildThreshold:   10,
			receiveThreshold: 20,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := make([]string, tc.n)
			for i := range tc.n {
				names[i] = string(rune('A' + i))
			}
			peers := testPeers(t, names...)

			s, err := NewScheduler(peers[0].ID, peers)
			require.NoError(t, err)

			assert.Equal(t, tc.numDataShards, s.NumDataShards())
			assert.Equal(t, tc.numCodingShards, s.NumCodingShards())
			assert.Equal(t, tc.numTotalShards, s.NumTotalShards())
			assert.Equal(t, tc.buildThreshold, s.BuildThreshold())
			assert.Equal(t, tc.receiveThreshold, s.ReceiveThreshold())
		})
	}
}

func TestScheduler_DeterministicMapping(t *testing.T) {
	// Two schedulers built from differently-ordered peer lists must
	// produce identical shard-to-peer mappings for every publisher.
	s1, err := NewScheduler(peer.ID("A"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	s2, err := NewScheduler(peer.ID("A"), testPeers(t, "D", "B", "A", "C"))
	require.NoError(t, err)

	for _, pub := range s1.Peers() {
		for idx := range s1.NumTotalShards() {
			p1, err1 := s1.PeerForShardIndex(pub.ID, ShardIndex(idx))
			p2, err2 := s2.PeerForShardIndex(pub.ID, ShardIndex(idx))
			require.NoError(t, err1)
			require.NoError(t, err2)
			assert.Equal(t, p1, p2, "publisher=%s shard=%d", pub.ID, idx)
		}
	}
}

func TestScheduler_PeerForShardIndex_SpecExample(t *testing.T) {
	// From the doc comment: peers [A, B, C, D], publisher = C (index 2).
	// Shard 0 -> A, Shard 1 -> B, Shard 2 -> D
	s, err := NewScheduler(peer.ID("A"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	publisher := peer.ID("C")
	expected := []peer.ID{"A", "B", "D"}
	for i, want := range expected {
		got, err := s.PeerForShardIndex(publisher, ShardIndex(i))
		require.NoError(t, err)
		assert.Equal(t, want, got, "shard %d", i)
	}
}

func TestScheduler_PeerForShardIndex_PublisherFirst(t *testing.T) {
	// Publisher is the first peer in sorted order.
	s, err := NewScheduler(peer.ID("B"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	publisher := peer.ID("A")
	// Shard 0 -> B, Shard 1 -> C, Shard 2 -> D
	expected := []peer.ID{"B", "C", "D"}
	for i, want := range expected {
		got, err := s.PeerForShardIndex(publisher, ShardIndex(i))
		require.NoError(t, err)
		assert.Equal(t, want, got, "shard %d", i)
	}
}

func TestScheduler_PeerForShardIndex_PublisherLast(t *testing.T) {
	// Publisher is the last peer in sorted order.
	s, err := NewScheduler(peer.ID("A"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	publisher := peer.ID("D")
	// Shard 0 -> A, Shard 1 -> B, Shard 2 -> C
	expected := []peer.ID{"A", "B", "C"}
	for i, want := range expected {
		got, err := s.PeerForShardIndex(publisher, ShardIndex(i))
		require.NoError(t, err)
		assert.Equal(t, want, got, "shard %d", i)
	}
}

func TestScheduler_PeerForShardIndex_Errors(t *testing.T) {
	s, err := NewScheduler(peer.ID("A"), testPeers(t, "A", "B", "C"))
	require.NoError(t, err)

	t.Run("publisher not in list", func(t *testing.T) {
		_, err := s.PeerForShardIndex(peer.ID("Z"), 0)
		assert.Error(t, err)
	})

	t.Run("shard index out of range", func(t *testing.T) {
		_, err := s.PeerForShardIndex(peer.ID("A"), ShardIndex(s.NumTotalShards()))
		assert.Error(t, err)
	})
}

func TestScheduler_ShardIndexForPublisher(t *testing.T) {
	// peers [A, B, C, D], publisher=C.
	// A(idx 0) -> shard 0, B(idx 1) -> shard 1, D(idx 3) -> shard 2
	tests := []struct {
		localPeer     string
		expectedShard ShardIndex
	}{
		{"A", 0},
		{"B", 1},
		{"D", 2},
	}
	publisher := peer.ID("C")

	for _, tc := range tests {
		t.Run("local="+tc.localPeer, func(t *testing.T) {
			s, err := NewScheduler(
				peer.ID(tc.localPeer), testPeers(t, "A", "B", "C", "D"),
			)
			require.NoError(t, err)

			got, err := s.ShardIndexForPublisher(publisher)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedShard, got)
		})
	}
}

func TestScheduler_ShardIndexForPublisher_Errors(t *testing.T) {
	s, err := NewScheduler(peer.ID("B"), testPeers(t, "A", "B", "C"))
	require.NoError(t, err)

	t.Run("local peer is the publisher", func(t *testing.T) {
		_, err := s.ShardIndexForPublisher(peer.ID("B"))
		assert.Error(t, err)
	})

	t.Run("publisher not in list", func(t *testing.T) {
		_, err := s.ShardIndexForPublisher(peer.ID("Z"))
		assert.Error(t, err)
	})
}

func TestScheduler_InverseProperty(t *testing.T) {
	// For every local peer and every publisher, verify that
	// PeerForShardIndex and ShardIndexForPublisher are inverses.
	names := []string{"A", "B", "C", "D", "E"}

	for _, local := range names {
		s, err := NewScheduler(peer.ID(local), testPeers(t, names...))
		require.NoError(t, err)

		for _, pub := range names {
			if pub == local {
				continue
			}
			// ShardIndexForPublisher -> PeerForShardIndex should round-trip
			shardIdx, err := s.ShardIndexForPublisher(peer.ID(pub))
			require.NoError(t, err)

			gotPeer, err := s.PeerForShardIndex(peer.ID(pub), shardIdx)
			require.NoError(t, err)
			assert.Equal(t, peer.ID(local), gotPeer,
				"local=%s publisher=%s shard=%d", local, pub, shardIdx)
		}
	}
}

func TestScheduler_BroadcastTargets(t *testing.T) {
	s, err := NewScheduler(peer.ID("C"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	targets := s.BroadcastTargets()

	// Should contain all peers except the local peer.
	assert.Len(t, targets, s.NumTotalShards())
	assert.Equal(t, []peer.ID{"A", "B", "D"}, targets)
}

func TestScheduler_ValidateShardOrigin(t *testing.T) {
	// Setup: peers [A, B, C, D], local = C
	// For publisher A: shard 0 -> A(skip, publisher), shard 1 -> B, shard 2 -> D
	// Wait — publisher A is at index 0, so:
	//   shard 0 -> B (peers[1]), shard 1 -> C (peers[2]), shard 2 -> D (peers[3])
	// So local=C is responsible for shard 1 when publisher=A.
	s, err := NewScheduler(peer.ID("C"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	publisher := peer.ID("A")

	t.Run("valid direct shard from publisher", func(t *testing.T) {
		// Shard 1's designated broadcaster is C (local peer).
		// A direct shard means publisher sends to the designated broadcaster.
		err := s.ValidateShardOrigin(publisher, publisher, 1)
		assert.NoError(t, err)
	})

	t.Run("valid broadcast shard from designated peer", func(t *testing.T) {
		// Shard 0's designated broadcaster is B.
		// B broadcasts shard 0 to other peers including C.
		err := s.ValidateShardOrigin(peer.ID("B"), publisher, 0)
		assert.NoError(t, err)
	})

	t.Run("self-send rejected", func(t *testing.T) {
		err := s.ValidateShardOrigin(peer.ID("C"), publisher, 0)
		assert.Error(t, err)
	})

	t.Run("self-published shard sent back rejected", func(t *testing.T) {
		// Local peer is both the publisher and the receiver — should error
		err := s.ValidateShardOrigin(peer.ID("A"), peer.ID("C"), 0)
		assert.Error(t, err)
	})

	t.Run("wrong sender rejected", func(t *testing.T) {
		// Shard 0's designated broadcaster is B, but D sends it.
		err := s.ValidateShardOrigin(peer.ID("D"), publisher, 0)
		assert.Error(t, err)
	})
}
