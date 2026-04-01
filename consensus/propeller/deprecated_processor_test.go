package propeller

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// processorTestEnv bundles the common setup for processor tests. It creates
// a realistic N-peer environment with a real Reed-Solomon encoder.
type processorTestEnv struct {
	peers     []peer.ID
	privKeys  map[peer.ID]crypto.PrivKey
	schedule  *Scheduler
	encoder   Encoder
	config    Config
	eventCh   chan any
	sentUnits []sentUnit
	sentMu    sync.Mutex
}

type sentUnit struct {
	To   peer.ID
	Unit *Unit
}

func newProcessorTestEnv(t *testing.T, n int) *processorTestEnv {
	t.Helper()

	rawPeers := make([]peer.ID, n)
	privKeys := make(map[peer.ID]crypto.PrivKey, n)
	for i := range n {
		priv, id := realPeer(byte(i))
		rawPeers[i] = id
		privKeys[id] = priv
	}

	schedule := NewScheduler(rawPeers)

	enc, err := NewEncoder(schedule.NumDataShards(), schedule.NumCodingShards())
	require.NoError(t, err)

	return &processorTestEnv{
		peers:    schedule.Peers(), // Use sorted order.
		privKeys: privKeys,
		schedule: schedule,
		encoder:  enc,
		config: Config{
			StaleMessageTimeout: 5 * time.Second,
		},
		eventCh: make(chan any, 100),
	}
}

// sendFunc records sent units for later inspection.
func (env *processorTestEnv) sendFunc() SendUnitFunc {
	return func(_ context.Context, to peer.ID, unit *Unit) error {
		env.sentMu.Lock()
		defer env.sentMu.Unlock()
		env.sentUnits = append(env.sentUnits, sentUnit{To: to, Unit: unit})
		return nil
	}
}

// encodeTestMessage encodes a message from the given publisher and returns
// the signed units and root.
func (env *processorTestEnv) encodeTestMessage(
	t *testing.T, publisher peer.ID, msg []byte,
) ([]Unit, MessageRoot) {
	t.Helper()

	units, root, err := EncodeMessage(msg, env.schedule, env.encoder)
	require.NoError(t, err)

	privKey, ok := env.privKeys[publisher]
	require.True(t, ok, "no private key for publisher %s", publisher)

	sig, err := SignRoot(root, privKey)
	require.NoError(t, err)

	for i := range units {
		units[i].Publisher = publisher
		units[i].Signature = sig
		units[i].CommitteeID = 1
	}

	return units, root
}

// drainEvents reads all currently available events from the event channel.
func (env *processorTestEnv) drainEvents() []any {
	var events []any
	for {
		select {
		case ev := <-env.eventCh:
			events = append(events, ev)
		default:
			return events
		}
	}
}

func TestProcessor_FullLifecycle(t *testing.T) {
	// Simulate a 7-node network. localPeer (sorted[0]) receives shards from
	// a message published by sorted[1]. With 7 peers: 2 data shards,
	// 4 coding shards. Build threshold = 2, receive threshold = 4.
	env := newProcessorTestEnv(t, 7)

	localPeer := env.peers[0]
	publisher := env.peers[1]
	msg := []byte("hello propeller protocol")

	units, root := env.encodeTestMessage(t, publisher, msg)

	validator := NewValidator(
		env.schedule, localPeer, &DefaultSignatureVerifier{},
	)

	shardCh := make(chan shardDelivery, 20)
	proc := NewMessageProcessor(
		1, publisher, root, localPeer, env.config,
		env.schedule, validator, env.encoder,
		shardCh, env.eventCh, env.sendFunc(),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	// Run the processor in a goroutine.
	done := make(chan struct{})
	go func() {
		proc.Run(ctx)
		close(done)
	}()

	// Feed shards one at a time, from their correct senders.
	// Skip shards "from" localPeer (the validator rejects self-sends).
	for i, unit := range units {
		sender, err := env.schedule.PeerForShard(publisher, ShardIndex(i))
		require.NoError(t, err)

		if sender == localPeer {
			continue
		}

		unitCopy := unit
		shardCh <- shardDelivery{Unit: &unitCopy, Sender: sender}
	}

	// Wait for the processor to finalise.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("processor did not finalise in time")
	}

	// Check that we got a MessageReceived event.
	events := env.drainEvents()
	var received *EventMessageReceived
	for _, ev := range events {
		if r, ok := ev.(EventMessageReceived); ok {
			received = &r
			break
		}
	}
	require.NotNil(t, received,
		"expected EventMessageReceived, got %d events", len(events))
	assert.Equal(t, msg, received.Message)
	assert.Equal(t, publisher, received.Publisher)
	assert.Equal(t, root, received.Root)

	// Check that our shard was broadcast to other peers.
	env.sentMu.Lock()
	defer env.sentMu.Unlock()
	assert.NotEmpty(t, env.sentUnits, "should have broadcast our shard")
}

func TestProcessor_ReconstructionFromMinimumShards(t *testing.T) {
	// With 4 peers: 1 data shard, 2 coding shards.
	// Build threshold = 1, receive threshold = 2 (N>3).
	// After reconstruction, the processor counts its own shard (=1),
	// so it needs at least 1 more from the network to reach 2.
	env := newProcessorTestEnv(t, 4)

	localPeer := env.peers[0]
	publisher := env.peers[1]
	msg := []byte("minimum shards test")

	units, root := env.encodeTestMessage(t, publisher, msg)

	validator := NewValidator(
		env.schedule, localPeer, &DefaultSignatureVerifier{},
	)

	shardCh := make(chan shardDelivery, 10)
	proc := NewMessageProcessor(
		1, publisher, root, localPeer, env.config,
		env.schedule, validator, env.encoder,
		shardCh, env.eventCh, env.sendFunc(),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		proc.Run(ctx)
		close(done)
	}()

	// Send all non-local shards. With 3 shards total and localPeer holding
	// one slot, we have 2 shards to send. receive threshold = 2, and
	// after reconstruction the processor holds its own shard (+1), so
	// it needs 1 from the network + 1 own = 2 to finalise.
	for i, unit := range units {
		sender, err := env.schedule.PeerForShard(publisher, ShardIndex(i))
		require.NoError(t, err)

		if sender == localPeer {
			continue
		}

		unitCopy := unit
		shardCh <- shardDelivery{Unit: &unitCopy, Sender: sender}
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("processor did not finalise in time")
	}

	events := env.drainEvents()
	var received *EventMessageReceived
	for _, ev := range events {
		if r, ok := ev.(EventMessageReceived); ok {
			received = &r
			break
		}
	}
	require.NotNil(t, received, "expected EventMessageReceived")
	assert.Equal(t, msg, received.Message)
}

func TestProcessor_Timeout(t *testing.T) {
	env := newProcessorTestEnv(t, 4)
	// Use a very short timeout for the test.
	env.config.StaleMessageTimeout = 50 * time.Millisecond

	localPeer := env.peers[0]
	publisher := env.peers[1]

	_, root := env.encodeTestMessage(t, publisher, []byte("will timeout"))

	validator := NewValidator(
		env.schedule, localPeer, &DefaultSignatureVerifier{},
	)

	shardCh := make(chan shardDelivery, 10)
	proc := NewMessageProcessor(
		42, publisher, root, localPeer, env.config,
		env.schedule, validator, env.encoder,
		shardCh, env.eventCh, env.sendFunc(),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		proc.Run(ctx)
		close(done)
	}()

	// Don't send any shards -- just wait for timeout.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processor did not finalise after timeout")
	}

	events := env.drainEvents()
	var timeout *EventMessageTimeout
	for _, ev := range events {
		if to, ok := ev.(EventMessageTimeout); ok {
			timeout = &to
			break
		}
	}
	require.NotNil(t, timeout,
		"expected EventMessageTimeout, got %d events", len(events))
	assert.Equal(t, CommitteeID(42), timeout.Channel)
	assert.Equal(t, publisher, timeout.Publisher)
	assert.Equal(t, root, timeout.Root)
}

func TestProcessor_DuplicateShardRejected(t *testing.T) {
	// Use 10 peers so the processor doesn't finalise after the first shard.
	// N=10: numDataShards=3, receiveThreshold=6.
	// Sending just one shard (receivedCount=1) is far below the threshold,
	// so the processor stays in PreConstruction and will reject the duplicate.
	env := newProcessorTestEnv(t, 10)

	localPeer := env.peers[0]
	publisher := env.peers[1]
	msg := []byte("test duplicates")

	units, root := env.encodeTestMessage(t, publisher, msg)

	validator := NewValidator(
		env.schedule, localPeer, &DefaultSignatureVerifier{},
	)

	shardCh := make(chan shardDelivery, 10)
	proc := NewMessageProcessor(
		1, publisher, root, localPeer, env.config,
		env.schedule, validator, env.encoder,
		shardCh, env.eventCh, env.sendFunc(),
	)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		proc.Run(ctx)
		close(done)
	}()

	// Find a shard not from localPeer.
	var targetUnit Unit
	var targetSender peer.ID
	for i, unit := range units {
		sender, err := env.schedule.PeerForShard(publisher, ShardIndex(i))
		require.NoError(t, err)
		if sender != localPeer {
			targetUnit = unit
			targetSender = sender
			break
		}
	}

	// Send the same shard twice, back-to-back. The processor handles them
	// sequentially (single goroutine), so the second one will see the first
	// already in seenShards.
	shardCh <- shardDelivery{Unit: &targetUnit, Sender: targetSender}

	dup := targetUnit
	shardCh <- shardDelivery{Unit: &dup, Sender: targetSender}

	// Give the processor time to handle both deliveries.
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	// Check that we got a validation failure event for the duplicate.
	events := env.drainEvents()
	var validationFailed bool
	for _, ev := range events {
		if vf, ok := ev.(EventShardValidationFailed); ok {
			var valErr *ShardValidationError
			if asErr, ok := vf.Err.(*ShardValidationError); ok {
				valErr = asErr
			}
			if valErr != nil && valErr.Reason == ReasonDuplicateShard {
				validationFailed = true
				break
			}
		}
	}
	assert.True(t, validationFailed, "expected duplicate shard to be rejected")
}

func TestProcessor_ContextCancellation(t *testing.T) {
	env := newProcessorTestEnv(t, 4)

	localPeer := env.peers[0]
	publisher := env.peers[1]

	validator := NewValidator(
		env.schedule, localPeer, &DefaultSignatureVerifier{},
	)

	root := MessageRoot{0x01}
	shardCh := make(chan shardDelivery, 10)
	proc := NewMessageProcessor(
		1, publisher, root, localPeer, env.config,
		env.schedule, validator, env.encoder,
		shardCh, env.eventCh, env.sendFunc(),
	)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		proc.Run(ctx)
		close(done)
	}()

	// Cancel immediately.
	cancel()

	select {
	case <-done:
		// Good, processor exited.
	case <-time.After(1 * time.Second):
		t.Fatal("processor did not exit on context cancellation")
	}
}

func TestProcessor_ChannelClose(t *testing.T) {
	env := newProcessorTestEnv(t, 4)

	localPeer := env.peers[0]
	publisher := env.peers[1]

	validator := NewValidator(
		env.schedule, localPeer, &DefaultSignatureVerifier{},
	)

	root := MessageRoot{0x02}
	shardCh := make(chan shardDelivery, 10)
	proc := NewMessageProcessor(
		1, publisher, root, localPeer, env.config,
		env.schedule, validator, env.encoder,
		shardCh, env.eventCh, env.sendFunc(),
	)

	ctx := t.Context()

	done := make(chan struct{})
	go func() {
		proc.Run(ctx)
		close(done)
	}()

	// Close the shard channel to signal teardown.
	close(shardCh)

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("processor did not exit on channel close")
	}
}
