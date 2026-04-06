package propeller

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// engineTestEnv provides the common setup for engine-level tests.
type engineTestEnv struct {
	peers     []peer.ID
	privKeys  []crypto.PrivKey
	engines   []*Engine
	sentUnits map[peer.ID][]*Unit
	sentMu    sync.Mutex
	log       utils.Logger
}

//nolint:unparam // n is always 4 in current tests but kept for flexibility
func newEngineTestEnv(t *testing.T, n int) *engineTestEnv {
	t.Helper()

	peers := make([]peer.ID, n)
	privKeys := make([]crypto.PrivKey, n)
	for i := range n {
		seed := make([]byte, ed25519.SeedSize)
		seed[0] = byte(i)
		reader := bytes.NewReader(seed)
		priv, pub, err := crypto.GenerateEd25519Key(reader)
		require.NoError(t, err)
		id, err := peer.IDFromPublicKey(pub)
		require.NoError(t, err)
		privKeys[i] = priv
		peers[i] = id
	}

	log := utils.NewNopZapLogger()

	env := &engineTestEnv{
		peers:     peers,
		privKeys:  privKeys,
		sentUnits: make(map[peer.ID][]*Unit),
		log:       log,
	}

	config := Config{
		StaleMessageTimeout: 5 * time.Second,
		StreamProtocol:      "/propeller/test/0.1.0",
		MaxWireMessageSize:  1 << 20,
	}

	engines := make([]*Engine, n)
	for i := range n {
		engines[i] = NewEngine(
			peers[i], privKeys[i], config,
			env.makeSendFn(),
			log,
		)
	}
	env.engines = engines

	return env
}

// makeSendFn creates a SendUnitFunc that records sent units.
func (env *engineTestEnv) makeSendFn() SendUnitFunc {
	return func(_ context.Context, to peer.ID, unit *Unit) error {
		env.sentMu.Lock()
		env.sentUnits[to] = append(env.sentUnits[to], unit)
		env.sentMu.Unlock()
		return nil
	}
}

// getSentUnits returns all units sent to a given peer.
func (env *engineTestEnv) getSentUnits(to peer.ID) []*Unit {
	env.sentMu.Lock()
	defer env.sentMu.Unlock()
	result := make([]*Unit, len(env.sentUnits[to]))
	copy(result, env.sentUnits[to])
	return result
}

func TestEngine_RegisterAndBroadcast(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	engine := env.engines[0]

	// Run the engine in the background.
	done := make(chan error, 1)
	go func() {
		done <- engine.Run(ctx)
	}()

	// Register a channel with all peers.
	err := engine.RegisterChannel(ctx, 1, env.peers)
	require.NoError(t, err)

	// Broadcast a message.
	msg := []byte("hello from engine test")
	err = engine.Broadcast(ctx, 1, msg)
	require.NoError(t, err)

	// Verify that units were sent to the other 3 peers.
	// Give a moment for async processing.
	time.Sleep(100 * time.Millisecond)

	totalSent := 0
	for _, p := range env.peers {
		if p == env.peers[0] {
			continue
		}
		units := env.getSentUnits(p)
		totalSent += len(units)
	}
	assert.Equal(t, 3, totalSent, "should send one unit to each non-publisher peer")

	cancel()
	<-done
}

func TestEngine_BroadcastUnregisteredChannel(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	engine := env.engines[0]

	go func() {
		engine.Run(ctx) //nolint:errcheck // test helper
	}()

	err := engine.Broadcast(ctx, 99, []byte("should fail"))
	require.Error(t, err)

	var pubErr *ShardPublishError
	require.ErrorAs(t, err, &pubErr)
	assert.Equal(t, ReasonChannelNotRegistered, pubErr.Reason)

	cancel()
}

func TestEngine_HandleUnit_CreatesProcessor(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	// Set up engine for peer 0.
	engine := env.engines[0]

	go func() {
		engine.Run(ctx) //nolint:errcheck // test helper
	}()

	// Register the channel.
	err := engine.RegisterChannel(ctx, 1, env.peers)
	require.NoError(t, err)

	// Simulate receiving a unit from peer 1 (as publisher).
	schedule := NewScheduler(env.peers)
	enc, err := NewEncoder(schedule.NumDataShards(), schedule.NumCodingShards())
	require.NoError(t, err)

	msg := []byte("incoming message")
	units, root, err := EncodeMessage(msg, schedule, enc)
	require.NoError(t, err)

	publisher := env.peers[1]
	sig, err := SignMessage(root, env.privKeys[1])
	require.NoError(t, err)

	for i := range units {
		units[i].Publisher = publisher
		units[i].Signature = sig
		units[i].CommitteeID = 1
	}

	// Send units from their correct senders.
	for i, unit := range units {
		sender, err := schedule.PeerForShard(publisher, ShardIndex(i))
		require.NoError(t, err)

		// Skip units "from ourselves" -- the validator rejects those.
		if sender == env.peers[0] {
			continue
		}

		unitCopy := unit
		engine.HandleUnit(&unitCopy, sender)
	}

	// Wait for the message to be processed and check events.
	var received *EventMessageReceived
	deadline := time.After(5 * time.Second)
	for received == nil {
		select {
		case ev := <-engine.Events():
			if r, ok := ev.(EventMessageReceived); ok {
				received = &r
			}
		case <-deadline:
			t.Fatal("timed out waiting for EventMessageReceived")
		}
	}

	assert.Equal(t, msg, received.Message)
	assert.Equal(t, publisher, received.Publisher)
	assert.Equal(t, root, received.Root)

	cancel()
}

func TestEngine_UnregisterChannel(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	engine := env.engines[0]

	go func() {
		engine.Run(ctx) //nolint:errcheck // test helper
	}()

	err := engine.RegisterChannel(ctx, 1, env.peers)
	require.NoError(t, err)

	err = engine.UnregisterChannel(ctx, 1)
	require.NoError(t, err)

	// Allow command to be processed.
	time.Sleep(50 * time.Millisecond)

	// Broadcasting should fail now.
	err = engine.Broadcast(ctx, 1, []byte("after unregister"))
	require.Error(t, err)

	cancel()
}

func TestEngine_HandleUnit_UnregisteredChannel(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	engine := env.engines[0]

	go func() {
		engine.Run(ctx) //nolint:errcheck // test helper
	}()

	// Send a unit for an unregistered channel.
	unit := &Unit{
		CommitteeID: 99,
		Publisher:   env.peers[1],
		MessageRoot: MessageRoot{0x01},
		ShardIndex:  0,
		ShardData:   []byte("data"),
	}
	engine.HandleUnit(unit, env.peers[1])

	// Allow time for processing.
	time.Sleep(100 * time.Millisecond)

	// No crash, no panic -- the unit is silently dropped.
	cancel()
}

func TestEngine_GracefulShutdown(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithCancel(t.Context())

	engine := env.engines[0]
	done := make(chan error, 1)
	go func() {
		done <- engine.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("engine did not shut down in time")
	}
}

func TestEngine_SendFailureEmitsEvent(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Create an engine with a failing send function.
	engine := NewEngine(
		env.peers[0], env.privKeys[0],
		Config{
			StaleMessageTimeout: 5 * time.Second,
			StreamProtocol:      "/propeller/test/0.1.0",
			MaxWireMessageSize:  1 << 20,
		},
		func(_ context.Context, _ peer.ID, _ *Unit) error {
			return fmt.Errorf("simulated network failure")
		},
		utils.NewNopZapLogger(),
	)

	go func() {
		engine.Run(ctx) //nolint:errcheck // test helper
	}()

	err := engine.RegisterChannel(ctx, 1, env.peers)
	require.NoError(t, err)

	err = engine.Broadcast(ctx, 1, []byte("will fail sending"))
	require.NoError(t, err) // Broadcast itself succeeds; send failures are events.

	// Collect send failure events.
	deadline := time.After(2 * time.Second)
	failures := 0
loop:
	for failures < 3 {
		select {
		case ev := <-engine.Events():
			if _, ok := ev.(EventShardSendFailed); ok {
				failures++
			}
		case <-deadline:
			break loop
		}
	}
	assert.Equal(t, 3, failures, "should have 3 send failures (one per non-publisher peer)")

	cancel()
}

func TestEngine_RegisterChannelTooFewPeers(t *testing.T) {
	env := newEngineTestEnv(t, 4)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	engine := env.engines[0]

	go func() {
		engine.Run(ctx) //nolint:errcheck // test helper
	}()

	// A single peer cannot form a channel (0 shards).
	err := engine.RegisterChannel(ctx, 1, []peer.ID{env.peers[0]})
	require.Error(t, err)

	cancel()
}
