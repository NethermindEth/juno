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

func TestScheduler_PeerForShardIndex(t *testing.T) {
	// peers [A, B, C, D]: the publisher is skipped in the sorted list,
	// so each remaining peer maps to shard indices 0..2 in order.
	tests := []struct {
		name      string
		publisher string
		expected  []peer.ID
	}{
		{
			name:      "publisher middle (C, index 2)",
			publisher: "C",
			expected:  []peer.ID{"A", "B", "D"},
		},
		{
			name:      "publisher first (A, index 0)",
			publisher: "A",
			expected:  []peer.ID{"B", "C", "D"},
		},
		{
			name:      "publisher last (D, index 3)",
			publisher: "D",
			expected:  []peer.ID{"A", "B", "C"},
		},
	}

	s, err := NewScheduler(peer.ID("A"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for i, want := range tc.expected {
				got, err := s.PeerForShardIndex(peer.ID(tc.publisher), ShardIndex(i))
				require.NoError(t, err)
				assert.Equal(t, want, got, "shard %d", i)
			}
		})
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
	// BroadcastTargets returns every peer except the local peer, in sorted order.
	tests := []struct {
		name     string
		local    string
		expected []peer.ID
	}{
		{
			name:     "local first",
			local:    "A",
			expected: []peer.ID{"B", "C", "D"},
		},
		{
			name:     "local middle",
			local:    "C",
			expected: []peer.ID{"A", "B", "D"},
		},
		{
			name:     "local last",
			local:    "D",
			expected: []peer.ID{"A", "B", "C"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := NewScheduler(peer.ID(tc.local), testPeers(t, "A", "B", "C", "D"))
			require.NoError(t, err)

			targets := s.BroadcastTargets()
			assert.Equal(t, tc.expected, targets)
		})
	}
}

func TestScheduler_ValidateShardOrigin(t *testing.T) {
	// peers [A, B, C, D], local = C.
	// For publisher A (index 0): shard 0 -> B, shard 1 -> C, shard 2 -> D
	// So local=C is the designated broadcaster for shard 1 when publisher=A.
	s, err := NewScheduler(peer.ID("C"), testPeers(t, "A", "B", "C", "D"))
	require.NoError(t, err)

	tests := []struct {
		name       string
		sender     string
		publisher  string
		shardIndex ShardIndex
		wantErr    bool
	}{
		{
			name:       "valid direct shard from publisher",
			sender:     "A",
			publisher:  "A",
			shardIndex: 1, // C is the designated broadcaster, so publisher sends directly
			wantErr:    false,
		},
		{
			name:       "valid broadcast shard from designated peer",
			sender:     "B",
			publisher:  "A",
			shardIndex: 0, // B is the designated broadcaster for shard 0
			wantErr:    false,
		},
		{
			name:       "self-send rejected",
			sender:     "C",
			publisher:  "A",
			shardIndex: 0,
			wantErr:    true,
		},
		{
			name:       "self-published shard sent back rejected",
			sender:     "A",
			publisher:  "C", // local peer is the publisher
			shardIndex: 0,
			wantErr:    true,
		},
		{
			name:       "wrong sender rejected",
			sender:     "D",
			publisher:  "A",
			shardIndex: 0, // designated broadcaster is B, not D
			wantErr:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := s.ValidateShardOrigin(peer.ID(tc.sender), peer.ID(tc.publisher), tc.shardIndex)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
