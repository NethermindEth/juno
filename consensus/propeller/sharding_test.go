package propeller

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeSchedule is a test helper that creates a schedule from N single-char peers.
func makeSchedule(n int) *Scheduler {
	names := make([]peer.ID, n)
	for i := range n {
		names[i] = peer.ID(string(rune('A' + i)))
	}
	return NewScheduler(names)
}

func TestEncodeMessage_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		n      int
		msgLen int
	}{
		{"4 peers, short message", 4, 10},
		{"4 peers, medium message", 4, 500},
		{"7 peers, short message", 7, 20},
		{"10 peers, 1KB message", 10, 1024},
		{"2 peers, tiny message", 2, 1},
		{"3 peers, empty message", 3, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schedule := makeSchedule(tc.n)
			if schedule.NumShards() == 0 {
				t.Skip("no shards for single peer")
			}

			enc, err := NewEncoder(
				schedule.NumDataShards(), schedule.NumCodingShards(),
			)
			require.NoError(t, err)

			msg := make([]byte, tc.msgLen)
			for i := range msg {
				msg[i] = byte(i)
			}

			units, root, err := EncodeMessage(msg, schedule, enc)
			require.NoError(t, err)
			assert.Len(t, units, schedule.NumShards())

			// All units should reference the same root.
			for _, u := range units {
				assert.Equal(t, root, u.MerkleRoot)
			}

			// Reconstruct from all shards.
			shards := make([][]byte, schedule.NumShards())
			for _, u := range units {
				shards[u.ShardIndex] = u.ShardData
			}

			recovered, err := ReconstructMessage(
				shards, schedule, enc, root,
			)
			require.NoError(t, err)
			assert.Equal(t, msg, recovered)
		})
	}
}

func TestEncodeMessage_ReconstructFromMinimumShards(t *testing.T) {
	// With N=10 we have 3 data shards and 6 coding shards.
	// We should be able to reconstruct from just the 3 data shards.
	schedule := makeSchedule(10)
	enc, err := NewEncoder(
		schedule.NumDataShards(), schedule.NumCodingShards(),
	)
	require.NoError(t, err)

	msg := []byte("reconstruct me from minimum shards please")
	units, root, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	// Keep only the first numDataShards shards.
	shards := make([][]byte, schedule.NumShards())
	for i := range schedule.NumDataShards() {
		shards[units[i].ShardIndex] = units[i].ShardData
	}

	recovered, err := ReconstructMessage(shards, schedule, enc, root)
	require.NoError(t, err)
	assert.Equal(t, msg, recovered)
}

func TestEncodeMessage_ReconstructWithMissingDataShards(t *testing.T) {
	// With N=7 we have 2 data shards and 4 coding shards.
	// Drop all data shards, keep only coding shards -> should reconstruct.
	schedule := makeSchedule(7)
	enc, err := NewEncoder(
		schedule.NumDataShards(), schedule.NumCodingShards(),
	)
	require.NoError(t, err)

	msg := []byte("even without data shards")
	units, root, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	// Keep only coding shards (indices >= numDataShards).
	shards := make([][]byte, schedule.NumShards())
	for _, u := range units {
		if int(u.ShardIndex) >= schedule.NumDataShards() {
			shards[u.ShardIndex] = u.ShardData
		}
	}

	recovered, err := ReconstructMessage(shards, schedule, enc, root)
	require.NoError(t, err)
	assert.Equal(t, msg, recovered)
}

func TestEncodeMessage_MerkleProofsVerify(t *testing.T) {
	schedule := makeSchedule(5)
	enc, err := NewEncoder(
		schedule.NumDataShards(), schedule.NumCodingShards(),
	)
	require.NoError(t, err)

	msg := []byte("verify all proofs")
	units, root, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	for _, u := range units {
		ok := VerifyMerkleProof(root, u.ShardData, uint32(u.ShardIndex), u.MerkleProof)
		assert.True(t, ok, "proof for shard %d should verify", u.ShardIndex)
	}
}

func TestReconstructMessage_MismatchedRoot(t *testing.T) {
	schedule := makeSchedule(4)
	enc, err := NewEncoder(
		schedule.NumDataShards(), schedule.NumCodingShards(),
	)
	require.NoError(t, err)

	msg := []byte("good message")
	units, _, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	shards := make([][]byte, schedule.NumShards())
	for _, u := range units {
		shards[u.ShardIndex] = u.ShardData
	}

	// Pass a wrong root.
	fakeRoot := MessageRoot{0xff}
	_, err = ReconstructMessage(shards, schedule, enc, fakeRoot)
	require.Error(t, err)

	var reconErr *ReconstructionError
	require.ErrorAs(t, err, &reconErr)
	assert.Equal(t, ReasonMismatchedMessageRoot, reconErr.Reason)
}

func TestReconstructMessage_InsufficientShards(t *testing.T) {
	schedule := makeSchedule(10) // 3 data, 6 coding
	enc, err := NewEncoder(
		schedule.NumDataShards(), schedule.NumCodingShards(),
	)
	require.NoError(t, err)

	msg := []byte("not enough shards")
	units, root, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	// Provide only 2 shards when 3 are needed.
	shards := make([][]byte, schedule.NumShards())
	shards[units[0].ShardIndex] = units[0].ShardData
	shards[units[1].ShardIndex] = units[1].ShardData

	_, err = ReconstructMessage(shards, schedule, enc, root)
	require.Error(t, err)

	var reconErr *ReconstructionError
	require.ErrorAs(t, err, &reconErr)
	assert.Equal(t, ReasonErasureReconstructionFailed, reconErr.Reason)
}

func TestEncodeMessage_NoShards(t *testing.T) {
	// A single-node schedule has no shards.
	schedule := makeSchedule(1)
	enc, err := NewEncoder(1, 0)
	require.NoError(t, err)

	_, _, err = EncodeMessage([]byte("x"), schedule, enc)
	require.Error(t, err)

	var pubErr *ShardPublishError
	require.ErrorAs(t, err, &pubErr)
	assert.Equal(t, ReasonInvalidDataSize, pubErr.Reason)
}
