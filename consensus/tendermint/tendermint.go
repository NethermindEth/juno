package tendermint

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
	db "github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/utils"
)

type (
	step        uint8
	height      uint32
	round       int
	votingPower uint
)

const (
	propose step = iota
	prevote
	precommit
)

func (s step) String() string {
	switch s {
	case propose:
		return "propose"
	case prevote:
		return "prevote"
	case precommit:
		return "precommit"
	default:
		return "unknown"
	}
}

const (
	maxFutureHeight = height(5)
	maxFutureRound  = round(5)
)

type timeoutFn func(r round) time.Duration

type Addr interface {
	// Ethereum Addresses are 20 bytes
	~[20]byte | felt.Felt
}

type Hash interface {
	~[32]byte | felt.Felt
}

// Hashable's Hash() is used as ID()
type Hashable[H Hash] interface {
	Hash() H
}

type Application[V Hashable[H], H Hash] interface {
	// Value returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Blockchain[V Hashable[H], H Hash, A Addr] interface {
	// Height return the current blockchain height
	Height() height

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(height, V, []Precommit[H, A])
}

type Validators[A Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(height) votingPower

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(A) votingPower

	// Proposer returns the proposer of the current round and height.
	Proposer(height, round) A
}

type Slasher[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

type Listener[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Listen would return consensus messages to Tendermint which are set by the validator set.
	Listen() <-chan M
}

type Broadcaster[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(M)

	// SendMsg would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(A, M)
}

type Listeners[V Hashable[H], H Hash, A Addr] struct {
	ProposalListener  Listener[Proposal[V, H, A], V, H, A]
	PrevoteListener   Listener[Prevote[H, A], V, H, A]
	PrecommitListener Listener[Precommit[H, A], V, H, A]
}

type Broadcasters[V Hashable[H], H Hash, A Addr] struct {
	ProposalBroadcaster  Broadcaster[Proposal[V, H, A], V, H, A]
	PrevoteBroadcaster   Broadcaster[Prevote[H, A], V, H, A]
	PrecommitBroadcaster Broadcaster[Precommit[H, A], V, H, A]
}

type Driver[V Hashable[H], H Hash, A Addr] struct {
	db TendermintDB[V, H, A]

	stateMachine *Tendermint[V, H, A]

	timeoutPropose   timeoutFn
	timeoutPrevote   timeoutFn
	timeoutPrecommit timeoutFn

	listeners    Listeners[V, H, A]
	broadcasters Broadcasters[V, H, A]

	scheduledTms map[timeout]*time.Timer
	timeoutsCh   chan timeout

	wg   sync.WaitGroup
	quit chan struct{}
}

type Tendermint[V Hashable[H], H Hash, A Addr] struct {
	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?

	messages messages[V, H, A]

	application Application[V, H]
	blockchain  Blockchain[V, H, A]
	validators  Validators[A]
}

type state[V Hashable[H], H Hash] struct {
	height height
	round  round
	step   step

	lockedValue *V
	lockedRound round
	validValue  *V
	validRound  round

	// The following are round level variable therefore when a round changes they must be reset.
	timeoutPrevoteScheduled       bool // line34 for the first time condition
	timeoutPrecommitScheduled     bool // line47 for the first time condition
	lockedValueAndOrValidValueSet bool // line36 for the first time condition
}

func New[V Hashable[H], H Hash, A Addr](
	nodeAddr A,
	app Application[V, H],
	chain Blockchain[V, H, A],
	vals Validators[A],
) *Tendermint[V, H, A] {
	return &Tendermint[V, H, A]{
		nodeAddr: nodeAddr,
		state: state[V, H]{
			height:      chain.Height(),
			lockedRound: -1,
			validRound:  -1,
		},
		messages:    newMessages[V, H, A](),
		application: app,
		blockchain:  chain,
		validators:  vals,
	}
}

func NewDriver[V Hashable[H], H Hash, A Addr](database db.KeyValueStore, nodeAddr A, app Application[V, H], chain Blockchain[V, H, A],
	vals Validators[A], listeners Listeners[V, H, A], broadcasters Broadcasters[V, H, A], tmPropose, tmPrevote, tmPrecommit timeoutFn,
) *Driver[V, H, A] {
	return &Driver[V, H, A]{
		db:               NewTendermintDB[V, H, A](database, chain.Height()),
		stateMachine:     New(nodeAddr, app, chain, vals),
		timeoutPropose:   tmPropose,
		timeoutPrevote:   tmPrevote,
		timeoutPrecommit: tmPrecommit,
		listeners:        listeners,
		broadcasters:     broadcasters,
		scheduledTms:     make(map[timeout]*time.Timer),
		timeoutsCh:       make(chan timeout),
		quit:             make(chan struct{}),
	}
}

type CachedProposal[V Hashable[H], H Hash, A Addr] struct {
	Proposal[V, H, A]
	Valid bool
	ID    *H
}

func (d *Driver[V, H, A]) Start() {
	// Todo: replay WAL msgs

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		actions := d.stateMachine.processStart(0)
		d.execute(actions)

		// Todo: check message signature everytime a message is received.
		// For the time being it can be assumed the signature is correct.

		for {
			select {
			case <-d.quit:
				return
			case tm := <-d.timeoutsCh:
				// Handling of timeouts is priorities over messages
				actions = d.stateMachine.processTimeout(tm)
				delete(d.scheduledTms, tm)
			case p := <-d.listeners.ProposalListener.Listen():
				actions = d.stateMachine.processProposal(p)
			case p := <-d.listeners.PrevoteListener.Listen():
				actions = d.stateMachine.processPrevote(p)
			case p := <-d.listeners.PrecommitListener.Listen():
				actions = d.stateMachine.processPrecommit(p)
			}
			d.execute(actions)
		}
	}()
}

func (d *Driver[V, H, A]) execute(actions []Action[V, H, A]) {
	for _, action := range actions {
		switch action := action.(type) {
		case *BroadcastProposal[V, H, A]:
			d.broadcasters.ProposalBroadcaster.Broadcast(Proposal[V, H, A](*action))
		case *BroadcastPrevote[H, A]:
			d.broadcasters.PrevoteBroadcaster.Broadcast(Prevote[H, A](*action))
		case *BroadcastPrecommit[H, A]:
			d.broadcasters.PrecommitBroadcaster.Broadcast(Precommit[H, A](*action))
		case *ScheduleTimeout:
			// Schedule the timeout
			var duration time.Duration
			switch action.Step {
			case propose:
				duration = d.timeoutPropose(action.Round)
			case prevote:
				duration = d.timeoutPrevote(action.Round)
			case precommit:
				duration = d.timeoutPrecommit(action.Round)
			default:
				return
			}
			d.scheduledTms[timeout(*action)] = time.AfterFunc(duration, func() {
				select {
				case <-d.quit:
				case d.timeoutsCh <- timeout(*action):
				}
			})
		}
	}
}

func (d *Driver[V, H, A]) Stop() {
	close(d.quit)
	d.wg.Wait()
	for _, tm := range d.scheduledTms {
		tm.Stop()
	}
}

func (t *Tendermint[V, H, A]) startRound(r round) Action[V, H, A] {
	t.state.round = r
	t.state.step = propose

	t.state.timeoutPrevoteScheduled = false
	t.state.lockedValueAndOrValidValueSet = false
	t.state.timeoutPrecommitScheduled = false

	if p := t.validators.Proposer(t.state.height, r); p == t.nodeAddr {
		var proposalValue *V
		if t.state.validValue != nil {
			proposalValue = t.state.validValue
		} else {
			proposalValue = utils.HeapPtr(t.application.Value())
		}
		return t.sendProposal(proposalValue)
	} else {
		return t.scheduleTimeout(propose)
	}
}

func (t *Tendermint[V, H, A]) scheduleTimeout(s step) Action[V, H, A] {
	return utils.HeapPtr(
		ScheduleTimeout{
			Step:   s,
			Height: t.state.height,
			Round:  t.state.round,
		},
	)
}

func (t *Tendermint[V, H, A]) validatorSetVotingPower(vals []A) votingPower {
	var totalVotingPower votingPower
	for _, v := range vals {
		totalVotingPower += t.validators.ValidatorVotingPower(v)
	}
	return totalVotingPower
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower votingPower) votingPower {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower votingPower) votingPower {
	// Unfortunately there is no ceiling function for integers in go.
	d := totalVotingPower * 2
	q := d / 3
	r := d % 3
	if r > 0 {
		q++
	}
	return q
}

// preprocessMessage add message to the message pool if:
// - height is within [current height, current height + maxFutureHeight]
// - if height is the current height, round is within [0, current round + maxFutureRound]
// - if height is a future height, round is within [0, maxFutureRound]
// The message is processed immediately if all the conditions above are met plus height is the current height.
func (t *Tendermint[V, H, A]) preprocessMessage(header MessageHeader[A], addMessage func()) bool {
	isCurrentHeight := header.Height == t.state.height

	var currentRoundOfHeaderHeight round
	// If the height is a future height, the round is considered to be 0, as the height hasn't started yet.
	if isCurrentHeight {
		currentRoundOfHeaderHeight = t.state.round
	}

	switch {
	case header.Height < t.state.height || header.Height > t.state.height+maxFutureHeight:
		return false
	case header.Round < 0 || header.Round > currentRoundOfHeaderHeight+maxFutureRound:
		return false
	default:
		addMessage()
		return isCurrentHeight
	}
}

// TODO: Improve performance. Current complexity is O(n).
func (t *Tendermint[V, H, A]) checkForQuorumPrecommit(r round, vID H) (matchingPrecommits []Precommit[H, A], hasQuorum bool) {
	precommits, ok := t.messages.precommits[t.state.height][r]
	if !ok {
		return nil, false
	}

	var vals []A
	for addr, p := range precommits {
		if p.ID != nil && *p.ID == vID {
			matchingPrecommits = append(matchingPrecommits, p)
			vals = append(vals, addr)
		}
	}
	return matchingPrecommits, t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))
}

// TODO: Improve performance. Current complexity is O(n).
func (t *Tendermint[V, H, A]) checkQuorumPrevotesGivenProposalVID(r round, vID H) (hasQuorum bool) {
	prevotes, ok := t.messages.prevotes[t.state.height][r]
	if !ok {
		return false
	}

	var vals []A
	for addr, p := range prevotes {
		if p.ID != nil && *p.ID == vID {
			vals = append(vals, addr)
		}
	}
	return t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))
}

func (t *Tendermint[V, H, A]) findProposal(r round) *CachedProposal[V, H, A] {
	v, ok := t.messages.proposals[t.state.height][r][t.validators.Proposer(t.state.height, r)]
	if !ok {
		return nil
	}

	return &CachedProposal[V, H, A]{
		Proposal: v,
		Valid:    t.application.Valid(*v.Value),
		ID:       utils.HeapPtr((*v.Value).Hash()),
	}
}
