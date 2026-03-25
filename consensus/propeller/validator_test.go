package propeller

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockVerifier is a test double for SignatureVerifier that returns
// configurable results.
type mockVerifier struct {
	valid bool
	err   error
}

func (m *mockVerifier) Verify(peer.ID, []byte, []byte) (bool, error) {
	return m.valid, m.err
}

// realPeer creates a real libp2p peer.ID from a deterministic Ed25519 seed.
// Real peer IDs are needed because DefaultSignatureVerifier extracts the
// public key from the peer ID, which only works for keys encoded into the ID.
func realPeer(seed byte) (crypto.PrivKey, peer.ID) {
	seedBytes := make([]byte, ed25519.SeedSize)
	seedBytes[0] = seed
	reader := bytes.NewReader(seedBytes)
	priv, pub, err := crypto.GenerateEd25519Key(reader)
	if err != nil {
		panic(err)
	}
	id, err := peer.IDFromPublicKey(pub)
	if err != nil {
		panic(err)
	}
	return priv, id
}

// makeValidUnit creates a PropellerUnit that passes all validation checks
// for the given schedule, publisher, and shard index. The unit has a valid
// Merkle proof and signature.
func makeValidUnit(
	t *testing.T,
	schedule *Scheduler,
	publisherKey crypto.PrivKey,
	publisher peer.ID,
	shardIndex ShardIndex,
) *PropellerUnit {
	t.Helper()

	// Create a simple message and encode it.
	msg := []byte("test message for validation")
	enc, err := NewEncoder(schedule.NumDataShards(), schedule.NumCodingShards())
	require.NoError(t, err)

	units, root, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	sig, err := SignRoot(root, publisherKey)
	require.NoError(t, err)

	unit := &units[shardIndex]
	unit.Publisher = publisher
	unit.Signature = sig
	unit.CommitteeID = 1
	return unit
}

// validatorTestSetup creates a realistic N-peer environment and returns
// the schedule, a local peer (which is NOT the publisher), the publisher's
// key and ID, and a shard index that maps to a sender who is neither
// localPeer nor publisher.
type validatorTestSetup struct {
	schedule     *Scheduler
	localPeer    peer.ID
	publisher    peer.ID
	publisherKey crypto.PrivKey
	// shardIndex and expectedSender: a shard whose assigned sender is a
	// third peer (neither localPeer nor publisher).
	shardIndex     ShardIndex
	expectedSender peer.ID
}

func newValidatorTestSetup(t *testing.T) validatorTestSetup {
	t.Helper()

	// Create 5 peers so we have enough room to find a shard where the
	// sender is a third party.
	n := 5
	keys := make([]crypto.PrivKey, n)
	ids := make([]peer.ID, n)
	for i := range n {
		keys[i], ids[i] = realPeer(byte(i))
	}

	schedule := NewScheduler(ids)
	sorted := schedule.Peers()

	// Pick localPeer = sorted[0], publisher = sorted[1].
	localPeer := sorted[0]
	publisher := sorted[1]

	// Find the publisher's private key.
	var publisherKey crypto.PrivKey
	for i, id := range ids {
		if id == publisher {
			publisherKey = keys[i]
			break
		}
	}
	require.NotNil(t, publisherKey)

	// Find a shard whose expected sender is NOT localPeer.
	var shardIndex ShardIndex
	var expectedSender peer.ID
	found := false
	for si := range schedule.NumShards() {
		s, err := schedule.PeerForShard(publisher, ShardIndex(si))
		require.NoError(t, err)
		if s != localPeer {
			shardIndex = ShardIndex(si)
			expectedSender = s
			found = true
			break
		}
	}
	require.True(t, found, "could not find a shard with a third-party sender")

	return validatorTestSetup{
		schedule:       schedule,
		localPeer:      localPeer,
		publisher:      publisher,
		publisherKey:   publisherKey,
		shardIndex:     shardIndex,
		expectedSender: expectedSender,
	}
}

func TestValidator_HappyPath(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &DefaultSignatureVerifier{})

	unit := makeValidUnit(
		t, setup.schedule, setup.publisherKey,
		setup.publisher, setup.shardIndex,
	)

	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, setup.expectedSender, seenShards, false)
	assert.NoError(t, err)
}

func TestValidator_SelfSending(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: true})

	unit := &PropellerUnit{Publisher: setup.publisher, ShardIndex: setup.shardIndex}
	err := v.ValidateUnit(unit, setup.localPeer, nil, true)

	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonSelfSending, valErr.Reason)
}

func TestValidator_ReceivedSelfPublishedShard(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: true})

	// Unit claims we are the publisher.
	unit := &PropellerUnit{Publisher: setup.localPeer, ShardIndex: 0}
	err := v.ValidateUnit(unit, setup.expectedSender, nil, true)

	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonReceivedSelfPublishedShard, valErr.Reason)
}

func TestValidator_DuplicateShard(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: true})

	unit := &PropellerUnit{Publisher: setup.publisher, ShardIndex: setup.shardIndex}
	seenShards := map[ShardIndex]bool{setup.shardIndex: true}

	err := v.ValidateUnit(unit, setup.expectedSender, seenShards, true)
	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonDuplicateShard, valErr.Reason)
}

func TestValidator_UnexpectedSender(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: true})

	// Find a peer that is NOT the expected sender, NOT localPeer, and NOT
	// the publisher. The publisher is now an accepted sender for any shard,
	// so it must be excluded from the "wrong sender" set.
	var wrongSender peer.ID
	for _, p := range setup.schedule.Peers() {
		if p != setup.expectedSender && p != setup.localPeer && p != setup.publisher {
			wrongSender = p
			break
		}
	}
	require.NotEmpty(t, wrongSender)

	unit := &PropellerUnit{Publisher: setup.publisher, ShardIndex: setup.shardIndex}
	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, wrongSender, seenShards, true)

	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonUnexpectedSender, valErr.Reason)
}

func TestValidator_PublisherAsAcceptedSender(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &DefaultSignatureVerifier{})

	// The publisher initially distributes all shards, so it should be
	// accepted as a sender for any shard -- even one assigned to another peer.
	unit := makeValidUnit(
		t, setup.schedule, setup.publisherKey,
		setup.publisher, setup.shardIndex,
	)

	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, setup.publisher, seenShards, false)
	assert.NoError(t, err, "publisher should be accepted as sender for any shard")
}

func TestValidator_MerkleProofFailed(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: true})

	// Create a unit with a bad Merkle proof.
	unit := &PropellerUnit{
		Publisher:   setup.publisher,
		ShardIndex:  setup.shardIndex,
		MerkleRoot:  MessageRoot{0x01},
		ShardData:   []byte("data"),
		MerkleProof: MerkleProof{Siblings: [][32]byte{{0xde, 0xad}}},
	}

	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, setup.expectedSender, seenShards, true)

	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonMerkleProofVerificationFailed, valErr.Reason)
}

func TestValidator_SignatureVerificationFailed(t *testing.T) {
	setup := newValidatorTestSetup(t)

	// Use a verifier that rejects signatures.
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: false})

	unit := makeValidUnit(
		t, setup.schedule, setup.publisherKey,
		setup.publisher, setup.shardIndex,
	)

	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, setup.expectedSender, seenShards, false)

	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonSignatureVerificationFailed, valErr.Reason)
}

func TestValidator_SignatureVerificationError(t *testing.T) {
	setup := newValidatorTestSetup(t)

	// Use a verifier that returns an error.
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{
		valid: false,
		err:   fmt.Errorf("key extraction failed"),
	})

	unit := makeValidUnit(
		t, setup.schedule, setup.publisherKey,
		setup.publisher, setup.shardIndex,
	)

	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, setup.expectedSender, seenShards, false)

	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonSignatureVerificationFailed, valErr.Reason)
	assert.Contains(t, valErr.Detail, "key extraction failed")
}

func TestValidator_SkipSignatureWhenAlreadyVerified(t *testing.T) {
	setup := newValidatorTestSetup(t)

	// Verifier that would reject -- but we pass signatureVerified=true.
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: false})

	unit := makeValidUnit(
		t, setup.schedule, setup.publisherKey,
		setup.publisher, setup.shardIndex,
	)

	seenShards := make(map[ShardIndex]bool)
	err := v.ValidateUnit(unit, setup.expectedSender, seenShards, true)
	assert.NoError(t, err, "should skip signature check when already verified")
}

func TestSignPayload(t *testing.T) {
	root := MessageRoot{0x01, 0x02, 0x03}
	payload := SignPayload(root)

	expected := append([]byte("<propeller>"), root[:]...)
	expected = append(expected, []byte("</propeller>")...)
	assert.Equal(t, expected, payload)
}

func TestSignRoot_RoundTrip(t *testing.T) {
	privKey, peerID := realPeer(42)

	root := MessageRoot{0xaa, 0xbb, 0xcc}
	sig, err := SignRoot(root, privKey)
	require.NoError(t, err)
	require.NotEmpty(t, sig)

	// Verify with the default verifier.
	verifier := DefaultSignatureVerifier{}
	payload := SignPayload(root)
	ok, err := verifier.Verify(peerID, payload, sig)
	require.NoError(t, err)
	assert.True(t, ok)

	// Wrong root should fail.
	wrongRoot := MessageRoot{0xff}
	wrongPayload := SignPayload(wrongRoot)
	ok, err = verifier.Verify(peerID, wrongPayload, sig)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestValidator_ScheduleError(t *testing.T) {
	setup := newValidatorTestSetup(t)
	v := NewValidator(setup.schedule, setup.localPeer, &mockVerifier{valid: true})

	_, unknownPeer := realPeer(99)
	unit := &PropellerUnit{
		Publisher:   unknownPeer,
		ShardIndex:  0,
		ShardData:   []byte("data"),
		MerkleProof: MerkleProof{},
	}

	err := v.ValidateUnit(unit, setup.expectedSender, make(map[ShardIndex]bool), true)
	var valErr *ShardValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, ReasonScheduleError, valErr.Reason)
}
