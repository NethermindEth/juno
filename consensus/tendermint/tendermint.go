package tendermint

import (
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type (
	Step        uint8
	Height      uint
	Round       int
	VotingPower uint
)

const (
	StepPropose Step = iota
	StepPrevote
	StepPrecommit
)

func (s Step) String() string {
	switch s {
	case StepPropose:
		return "propose"
	case StepPrevote:
		return "prevote"
	case StepPrecommit:
		return "precommit"
	default:
		return "unknown"
	}
}

const (
	maxFutureHeight = Height(5)
	maxFutureRound  = Round(5)
)

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
	Height() Height

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(Height, V, []Precommit[H, A])
}

type Validators[A Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(Height) VotingPower

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(A) VotingPower

	// Proposer returns the proposer of the current round and height.
	Proposer(Height, Round) A
}

type Slasher[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

type StateMachine[V Hashable[H], H Hash, A Addr] interface {
	ProcessStart(Round) []Action[V, H, A]
	ProcessTimeout(Timeout) []Action[V, H, A]
	ProcessProposal(Proposal[V, H, A]) []Action[V, H, A]
	ProcessPrevote(Prevote[H, A]) []Action[V, H, A]
	ProcessPrecommit(Precommit[H, A]) []Action[V, H, A]
}

type stateMachine[V Hashable[H], H Hash, A Addr] struct {
	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?

	messages messages[V, H, A]

	application Application[V, H]
	blockchain  Blockchain[V, H, A]
	validators  Validators[A]
}

type state[V Hashable[H], H Hash] struct {
	height Height
	round  Round
	step   Step

	lockedValue *V
	lockedRound Round
	validValue  *V
	validRound  Round

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
) StateMachine[V, H, A] {
	return &stateMachine[V, H, A]{
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

type CachedProposal[V Hashable[H], H Hash, A Addr] struct {
	Proposal[V, H, A]
	Valid bool
	ID    *H
}

func (t *stateMachine[V, H, A]) startRound(r Round) Action[V, H, A] {
	t.state.round = r
	t.state.step = StepPropose

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
		return t.scheduleTimeout(StepPropose)
	}
}

type Timeout struct {
	Step   Step
	Height Height
	Round  Round
}

func (t *stateMachine[V, H, A]) scheduleTimeout(s Step) Action[V, H, A] {
	return utils.HeapPtr(
		ScheduleTimeout{
			Step:   s,
			Height: t.state.height,
			Round:  t.state.round,
		},
	)
}

func (t *stateMachine[V, H, A]) validatorSetVotingPower(vals []A) VotingPower {
	var totalVotingPower VotingPower
	for _, v := range vals {
		totalVotingPower += t.validators.ValidatorVotingPower(v)
	}
	return totalVotingPower
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower VotingPower) VotingPower {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower VotingPower) VotingPower {
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
func (t *stateMachine[V, H, A]) preprocessMessage(header MessageHeader[A], addMessage func()) bool {
	isCurrentHeight := header.Height == t.state.height

	var currentRoundOfHeaderHeight Round
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
func (t *stateMachine[V, H, A]) checkForQuorumPrecommit(r Round, vID H) (matchingPrecommits []Precommit[H, A], hasQuorum bool) {
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
func (t *stateMachine[V, H, A]) checkQuorumPrevotesGivenProposalVID(r Round, vID H) (hasQuorum bool) {
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

func (t *stateMachine[V, H, A]) findProposal(r Round) *CachedProposal[V, H, A] {
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
