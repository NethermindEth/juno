package broadcast_test

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NethermindEth/juno/broadcaster/broadcast"
	"github.com/NethermindEth/juno/broadcaster/broadcast/ring"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// soakDurationEnv sets the total soak wall time, split evenly across scenarios; unset skips the
// test.
const soakDurationEnv = "BROADCAST_SOAK_DURATION"

// soakSeedEnv pins the random seed; unset derives one from the clock, which the test logs.
const soakSeedEnv = "BROADCAST_SOAK_SEED"

const (
	// stallWindow is how long any actor may go without progress before the soak fails.
	stallWindow = 10 * time.Second
	// heartbeatEvery is how often the soak logs its cumulative counters.
	heartbeatEvery = 30 * time.Second
	// closeTimeout bounds how long Unsubscribe may take to close the receive channel.
	closeTimeout = 5 * time.Second
	// shutdownTimeout bounds how long actors may take to exit once a scenario's deadline passes.
	shutdownTimeout = 30 * time.Second
	// leakTimeout bounds how long goroutines may take to drain after a scenario ends.
	leakTimeout = 5 * time.Second
	// quietLength is how long every producer pauses so that readers park on a slot.
	quietLength = 300 * time.Millisecond
	// quietCloseMargin is the quiet left at Unsubscribe above which the close must beat the resume.
	quietCloseMargin = 100 * time.Millisecond
	// idleChurnTimeout is how long an idle-churn subscriber waits for a message before
	// unsubscribing while parked.
	idleChurnTimeout = 20 * time.Millisecond
)

// soakMessage is what producers publish: the producer index and that producer's own send counter.
type soakMessage struct {
	producer int
	n        uint64
}

// producerProfile sends burst messages back to back and then pauses for a random duration in
// [minPause, maxPause].
type producerProfile struct {
	name     string
	burst    uint64
	minPause time.Duration
	maxPause time.Duration
}

var (
	hotProducer = producerProfile{
		name:     "hot",
		burst:    10_000,
		minPause: 0,
		maxPause: time.Millisecond,
	}
	pacedProducer = producerProfile{
		name:     "paced",
		burst:    1,
		minPause: 100 * time.Microsecond,
		maxPause: 200 * time.Microsecond,
	}
	burstyProducer = producerProfile{
		name:     "bursty",
		burst:    500,
		minPause: 5 * time.Millisecond,
		maxPause: 50 * time.Millisecond,
	}
)

// subscriberKind selects how a subscriber drains its channel and when it unsubscribes.
type subscriberKind int

const (
	// fastSubscriber drains as fast as it can and should rarely lag.
	fastSubscriber subscriberKind = iota
	// slowSubscriber sleeps per message, so it lags continuously and exercises resync.
	slowSubscriber
	// burstySubscriber drains a batch, sleeps, and repeats.
	burstySubscriber
	// countChurnSubscriber reads a random number of messages, possibly zero, then unsubscribes.
	countChurnSubscriber
	// idleChurnSubscriber reads until the stream goes quiet, then unsubscribes while parked.
	idleChurnSubscriber
)

var subscriberKindNames = map[subscriberKind]string{
	fastSubscriber:       "fast",
	slowSubscriber:       "slow",
	burstySubscriber:     "bursty",
	countChurnSubscriber: "count-churn",
	idleChurnSubscriber:  "idle-churn",
}

// subscriberMix is the 64-subscriber population every scenario runs.
var subscriberMix = []struct {
	kind  subscriberKind
	count int
}{
	{kind: fastSubscriber, count: 20},
	{kind: slowSubscriber, count: 10},
	{kind: burstySubscriber, count: 10},
	{kind: countChurnSubscriber, count: 12},
	{kind: idleChurnSubscriber, count: 12},
}

// soakScenario is one Broadcast under a fixed producer set; exactSequence enables the sequence
// arithmetic checks that only hold with a single producer.
type soakScenario struct {
	name          string
	capacity      uint64
	producers     []producerProfile
	exactSequence bool
}

// soakActor is one goroutine whose progress counter the watchdog samples.
type soakActor struct {
	name     string
	progress atomic.Uint64
}

// soakRun holds the shared state of one scenario run.
type soakRun struct {
	scenario          soakScenario
	bcast             *broadcast.Broadcast[soakMessage]
	actors            []*soakActor
	quietUntil        atomic.Int64
	sent              atomic.Uint64
	received          atomic.Uint64
	lagged            atomic.Uint64
	unsubscribed      atomic.Uint64
	quietUnsubscribes atomic.Uint64
	quietPeriods      atomic.Uint64
}

// TestSoak runs mixed producers and subscribers against one Broadcast for BROADCAST_SOAK_DURATION
// and fails if any actor stalls, an Unsubscribe does not close its channel (before producers
// resume, when it lands in a quiet period), goroutines leak, or a subscriber observes out-of-order
// or inconsistent delivery.
//
//	BROADCAST_SOAK_DURATION=30m \
//	  go test ./broadcaster/broadcast/ -run TestSoak -race -v -timeout 2h
func TestSoak(t *testing.T) {
	durationText := os.Getenv(soakDurationEnv)
	if durationText == "" {
		t.Skipf("set %s (for example 30m) to run the soak", soakDurationEnv)
	}
	total, err := time.ParseDuration(durationText)
	require.NoError(t, err)
	seed := soakSeed(t)

	scenarios := []soakScenario{
		{
			name:          "single-producer-exact",
			capacity:      64,
			producers:     []producerProfile{hotProducer},
			exactSequence: true,
		},
		{
			name:          "tiny-ring-exact",
			capacity:      2,
			producers:     []producerProfile{hotProducer},
			exactSequence: true,
		},
		{
			name:     "multi-producer",
			capacity: 256,
			producers: []producerProfile{
				hotProducer,
				hotProducer,
				pacedProducer,
				burstyProducer,
			},
		},
		{
			name:     "tiny-ring-multi-producer",
			capacity: 2,
			producers: []producerProfile{
				hotProducer,
				hotProducer,
				burstyProducer,
			},
		},
	}
	perScenario := total / time.Duration(len(scenarios))
	for index, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			runSoakScenario(t, scenario, perScenario, seed+uint64(index))
		})
	}
}

func soakSeed(t *testing.T) uint64 {
	t.Helper()
	seed := uint64(time.Now().UnixNano())
	if text := os.Getenv(soakSeedEnv); text != "" {
		parsed, err := strconv.ParseUint(text, 10, 64)
		require.NoError(t, err)
		seed = parsed
	}
	t.Logf("seed=%d (set %s to replay)", seed, soakSeedEnv)
	return seed
}

func runSoakScenario(
	t *testing.T,
	scenario soakScenario,
	duration time.Duration,
	seed uint64,
) {
	t.Logf("capacity=%d producers=%d duration=%s",
		scenario.capacity,
		len(scenario.producers),
		duration,
	)
	baselineGoroutines := runtime.NumGoroutine()

	run := &soakRun{scenario: scenario, bcast: broadcast.New[soakMessage](scenario.capacity)}
	deadlineCtx, cancel := context.WithTimeout(t.Context(), duration)
	defer cancel()
	group, ctx := errgroup.WithContext(deadlineCtx)

	root := rand.New(rand.NewPCG(seed, seed))
	newRandom := func() *rand.Rand { return rand.New(rand.NewPCG(root.Uint64(), root.Uint64())) }

	quietRandom := newRandom()
	group.Go(func() error { return run.quietController(ctx, quietRandom) })
	for index, profile := range scenario.producers {
		actor := run.newActor(fmt.Sprintf("producer-%d-%s", index, profile.name))
		random := newRandom()
		group.Go(func() error { return run.produce(ctx, actor, index, profile, random) })
	}
	for _, entry := range subscriberMix {
		for index := range entry.count {
			name := fmt.Sprintf("subscriber-%s-%d", subscriberKindNames[entry.kind], index)
			actor := run.newActor(name)
			random := newRandom()
			group.Go(func() error { return run.subscribe(ctx, actor, entry.kind, random) })
		}
	}

	done := make(chan error, 1)
	go func() { done <- group.Wait() }()

	start := time.Now()
	stallTicker := time.NewTicker(stallWindow)
	defer stallTicker.Stop()
	heartbeatTicker := time.NewTicker(heartbeatEvery)
	defer heartbeatTicker.Stop()
	last := run.snapshot()
	running := ctx.Done()
	var shutdownDeadline <-chan time.Time

	for {
		select {
		case err := <-done:
			require.NoError(t, err)
			run.logHeartbeat(t, start)
			run.checkGoroutines(t, baselineGoroutines)
			return
		case <-running:
			// Actors legitimately stop once ctx is done, so the stall check ends here.
			running = nil
			stallTicker.Stop()
			shutdownDeadline = time.After(shutdownTimeout)
		case <-shutdownDeadline:
			dumpGoroutines(t)
			t.Fatalf("actors did not exit within %s after the scenario ended", shutdownTimeout)
		case <-heartbeatTicker.C:
			run.logHeartbeat(t, start)
		case <-stallTicker.C:
			if running == nil {
				continue
			}
			current := run.snapshot()
			for index, actor := range run.actors {
				if current[index] == last[index] {
					run.logHeartbeat(t, start)
					dumpGoroutines(t)
					t.Fatalf("%s made no progress in %s", actor.name, stallWindow)
				}
			}
			last = current
		}
	}
}

func (run *soakRun) newActor(name string) *soakActor {
	actor := &soakActor{name: name}
	run.actors = append(run.actors, actor)
	return actor
}

func (run *soakRun) snapshot() []uint64 {
	values := make([]uint64, len(run.actors))
	for index, actor := range run.actors {
		values[index] = actor.progress.Load()
	}
	return values
}

func (run *soakRun) logHeartbeat(t *testing.T, start time.Time) {
	t.Helper()
	t.Logf("[%s] sent=%d received=%d lagged=%d unsubscribed=%d quietUnsubscribes=%d "+
		"quietPeriods=%d goroutines=%d",
		time.Since(start).Truncate(time.Second),
		run.sent.Load(),
		run.received.Load(),
		run.lagged.Load(),
		run.unsubscribed.Load(),
		run.quietUnsubscribes.Load(),
		run.quietPeriods.Load(),
		runtime.NumGoroutine(),
	)
}

// checkGoroutines fails if the goroutine count has not returned to baseline within leakTimeout.
func (run *soakRun) checkGoroutines(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(leakTimeout)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= baseline {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	dumpGoroutines(t)
	t.Fatalf("goroutines leaked: %d running, baseline %d", runtime.NumGoroutine(), baseline)
}

func dumpGoroutines(t *testing.T) {
	t.Helper()
	var buffer bytes.Buffer
	if err := pprof.Lookup("goroutine").WriteTo(&buffer, 2); err != nil {
		t.Logf("dumping goroutines: %v", err)
		return
	}
	t.Log(buffer.String())
}

// quietController periodically pauses every producer so readers park on a slot before churn
// unsubscribes them.
func (run *soakRun) quietController(ctx context.Context, random *rand.Rand) error {
	for {
		if !sleepContext(ctx, randomDuration(random, time.Second, 3*time.Second)) {
			return nil
		}
		run.quietUntil.Store(time.Now().Add(quietLength).UnixNano())
		run.quietPeriods.Add(1)
	}
}

// waitQuiet sleeps through an active quiet period; it returns false once ctx is done.
func (run *soakRun) waitQuiet(ctx context.Context) bool {
	until := time.Unix(0, run.quietUntil.Load())
	if remaining := time.Until(until); remaining > 0 {
		return sleepContext(ctx, remaining)
	}
	return ctx.Err() == nil
}

func (run *soakRun) produce(
	ctx context.Context,
	actor *soakActor,
	index int,
	profile producerProfile,
	random *rand.Rand,
) error {
	publisher := run.bcast.NewPublisher()
	var n uint64
	for {
		if !run.waitQuiet(ctx) {
			return nil
		}
		for range profile.burst {
			publisher.Send(soakMessage{producer: index, n: n})
			n++
			actor.progress.Add(1)
		}
		run.sent.Add(profile.burst)
		if !sleepContext(ctx, randomDuration(random, profile.minPause, profile.maxPause)) {
			return nil
		}
	}
}

// subscribe runs subscription sessions back to back until ctx is done; churn kinds end a session
// early.
func (run *soakRun) subscribe(
	ctx context.Context,
	actor *soakActor,
	kind subscriberKind,
	random *rand.Rand,
) error {
	for ctx.Err() == nil {
		if err := run.session(ctx, actor, kind, random); err != nil {
			return fmt.Errorf("%s: %w", actor.name, err)
		}
		actor.progress.Add(1)
	}
	return nil
}

// session is one Subscribe/Unsubscribe lifetime; the channel must close within closeTimeout, and
// before producers resume when Unsubscribe lands in a quiet period, since any later write to the
// parked slot would wake the reader and hide a lost cancellation wakeup.
func (run *soakRun) session(
	ctx context.Context,
	actor *soakActor,
	kind subscriberKind,
	random *rand.Rand,
) error {
	subscription := run.bcast.NewSubscribable().Subscribe()
	recv := subscription.Recv()
	checker := newDeliveryChecker(len(run.scenario.producers), run.scenario.exactSequence)

	consumeErr := run.consume(ctx, actor, kind, random, recv, checker)
	quietUntil := time.Unix(0, run.quietUntil.Load())
	quietRemaining := time.Until(quietUntil)
	subscription.Unsubscribe()
	run.unsubscribed.Add(1)
	if err := waitClosed(recv, closeTimeout); err != nil {
		return err
	}
	if quietRemaining > quietCloseMargin {
		run.quietUnsubscribes.Add(1)
		if late := time.Since(quietUntil); late > 0 {
			return fmt.Errorf("receive channel closed %s after producers resumed; "+
				"Unsubscribe left %s of quiet",
				late,
				quietRemaining,
			)
		}
	}
	return consumeErr
}

// consume drains recv according to kind; it returns nil when the session should end.
func (run *soakRun) consume(
	ctx context.Context,
	actor *soakActor,
	kind subscriberKind,
	random *rand.Rand,
	recv <-chan ring.EventOrLag[soakMessage],
	checker *deliveryChecker,
) error {
	limit := uint64(math.MaxUint64)
	switch kind {
	case countChurnSubscriber:
		limit = random.Uint64N(65)
	case idleChurnSubscriber:
		limit = 10_000
	case fastSubscriber, slowSubscriber, burstySubscriber:
	}

	for count := range limit {
		event, ok, err := receive(ctx, kind, recv)
		if err != nil || !ok {
			return err
		}
		if err := checker.check(event); err != nil {
			return err
		}
		actor.progress.Add(1)
		if _, isLag := event.AsLag(); isLag {
			run.lagged.Add(1)
		} else {
			run.received.Add(1)
		}
		if !pace(ctx, kind, random, count) {
			return nil
		}
	}
	// Linger so some unsubscribes land on a parked reader or a full channel buffer.
	sleepContext(ctx, randomDuration(random, 0, 5*time.Millisecond))
	return nil
}

// receive waits for the next value; ok is false when the session should end, which for an
// idle-churn subscriber is as soon as the stream goes quiet.
func receive(
	ctx context.Context, kind subscriberKind, recv <-chan ring.EventOrLag[soakMessage],
) (event ring.EventOrLag[soakMessage], ok bool, err error) {
	var idle <-chan time.Time
	if kind == idleChurnSubscriber {
		idle = time.After(idleChurnTimeout)
	}
	select {
	case <-ctx.Done():
		return event, false, nil
	case <-idle:
		return event, false, nil
	case received, open := <-recv:
		if !open {
			return event, false, fmt.Errorf("receive channel closed before Unsubscribe")
		}
		return received, true, nil
	}
}

// pace applies the kind's per-message delay; it returns false once ctx is done.
func pace(
	ctx context.Context,
	kind subscriberKind,
	random *rand.Rand,
	count uint64,
) bool {
	switch kind {
	case slowSubscriber:
		return sleepContext(ctx, randomDuration(random, time.Millisecond, 3*time.Millisecond))
	case burstySubscriber:
		if count%200 == 199 {
			return sleepContext(ctx, 20*time.Millisecond)
		}
	case fastSubscriber, countChurnSubscriber, idleChurnSubscriber:
	}
	return true
}

// waitClosed drains recv until it closes, failing if that takes longer than timeout.
func waitClosed[T any](recv <-chan T, timeout time.Duration) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case _, ok := <-recv:
			if !ok {
				return nil
			}
		case <-timer.C:
			return fmt.Errorf("receive channel still open %s after Unsubscribe", timeout)
		}
	}
}

// deliveryChecker validates one subscription's stream: per-producer send counters only move
// forward, lag notifications advance and chain onto each other, and with a single producer the
// send counter pins the ring sequence (counter n is published at sequence n+1).
type deliveryChecker struct {
	exactSequence   bool
	seen            []bool
	lastCounter     []uint64
	expectedSeq     uint64
	haveExpectedSeq bool
}

func newDeliveryChecker(producers int, exactSequence bool) *deliveryChecker {
	return &deliveryChecker{
		exactSequence: exactSequence,
		seen:          make([]bool, producers),
		lastCounter:   make([]uint64, producers),
	}
}

func (checker *deliveryChecker) check(event ring.EventOrLag[soakMessage]) error {
	if lag, ok := event.AsLag(); ok {
		if lag.NextSeq <= lag.MissedSeq {
			return fmt.Errorf("lag does not advance: missed=%d next=%d", lag.MissedSeq, lag.NextSeq)
		}
		if checker.haveExpectedSeq && lag.MissedSeq != checker.expectedSeq {
			return fmt.Errorf("lag missed seq %d but reader expected seq %d",
				lag.MissedSeq,
				checker.expectedSeq,
			)
		}
		checker.expectedSeq, checker.haveExpectedSeq = lag.NextSeq, true
		return nil
	}

	message, _ := event.AsEvent()
	if message.producer < 0 || message.producer >= len(checker.seen) {
		return fmt.Errorf("event from unknown producer %d", message.producer)
	}
	if checker.seen[message.producer] && message.n <= checker.lastCounter[message.producer] {
		return fmt.Errorf("producer %d counter went backwards: %d after %d",
			message.producer,
			message.n,
			checker.lastCounter[message.producer],
		)
	}
	checker.seen[message.producer] = true
	checker.lastCounter[message.producer] = message.n

	if !checker.exactSequence {
		checker.haveExpectedSeq = false
		return nil
	}
	if checker.haveExpectedSeq && message.n+1 != checker.expectedSeq {
		return fmt.Errorf("event at seq %d but reader expected seq %d",
			message.n+1,
			checker.expectedSeq,
		)
	}
	checker.expectedSeq, checker.haveExpectedSeq = message.n+2, true
	return nil
}

// sleepContext sleeps for duration or until ctx is done, returning false in the latter case.
func sleepContext(ctx context.Context, duration time.Duration) bool {
	if duration <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func randomDuration(random *rand.Rand, minimum, maximum time.Duration) time.Duration {
	if maximum <= minimum {
		return minimum
	}
	return minimum + time.Duration(random.Int64N(int64(maximum-minimum)+1))
}
