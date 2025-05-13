package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

const (
	maxFutureHeight = types.Height(5)
	maxFutureRound  = types.Round(5)
)

type Application[V types.Hashable[H], H types.Hash] interface {
	// Value returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Blockchain[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// types.Height return the current blockchain height
	Height() types.Height

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(types.Height, V, []types.Precommit[H, A])
}

type Validators[A types.Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(types.Height) types.VotingPower

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(A) types.VotingPower

	// Proposer returns the proposer of the current round and height.
	Proposer(types.Height, types.Round) A
}

type Slasher[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

//go:generate mockgen -destination=../mocks/mock_state_machine.go -package=mocks github.com/NethermindEth/juno/consensus/tendermint StateMachine
type StateMachine[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	ProcessStart(types.Round) []Action[V, H, A]
	ProcessTimeout(types.Timeout) []Action[V, H, A]
	ProcessProposal(types.Proposal[V, H, A]) []Action[V, H, A]
	ProcessPrevote(types.Prevote[H, A]) []Action[V, H, A]
	ProcessPrecommit(types.Precommit[H, A]) []Action[V, H, A]
}

type stateMachine[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?

	messages types.Messages[V, H, A]

	application Application[V, H]
	blockchain  Blockchain[V, H, A]
	validators  Validators[A]
}

type state[V types.Hashable[H], H types.Hash] struct {
	height types.Height
	round  types.Round
	step   types.Step

	lockedValue *V
	lockedRound types.Round
	validValue  *V
	validRound  types.Round

	// The following are round level variable therefore when a round changes they must be reset.
	timeoutPrevoteScheduled       bool // line34 for the first time condition
	timeoutPrecommitScheduled     bool // line47 for the first time condition
	lockedValueAndOrValidValueSet bool // line36 for the first time condition
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
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
		messages:    types.NewMessages[V, H, A](),
		application: app,
		blockchain:  chain,
		validators:  vals,
	}
}

type CachedProposal[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	types.Proposal[V, H, A]
	Valid bool
	ID    *H
}

func (t *stateMachine[V, H, A]) startRound(r types.Round) Action[V, H, A] {
	t.state.round = r
	t.state.step = types.StepPropose

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
		return t.scheduleTimeout(types.StepPropose)
	}
}

func (t *stateMachine[V, H, A]) scheduleTimeout(s types.Step) Action[V, H, A] {
	return utils.HeapPtr(
		ScheduleTimeout{
			Step:   s,
			Height: t.state.height,
			Round:  t.state.round,
		},
	)
}

func (t *stateMachine[V, H, A]) validatorSetVotingPower(vals []A) types.VotingPower {
	var totalVotingPower types.VotingPower
	for _, v := range vals {
		totalVotingPower += t.validators.ValidatorVotingPower(v)
	}
	return totalVotingPower
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower types.VotingPower) types.VotingPower {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower types.VotingPower) types.VotingPower {
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
func (t *stateMachine[V, H, A]) preprocessMessage(header types.MessageHeader[A], addMessage func()) bool {
	isCurrentHeight := header.Height == t.state.height

	var currentRoundOfHeaderHeight types.Round
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
func (t *stateMachine[V, H, A]) checkForQuorumPrecommit(r types.Round, vID H) (matchingPrecommits []types.Precommit[H, A], hasQuorum bool) {
	precommits, ok := t.messages.Precommits[t.state.height][r]
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
func (t *stateMachine[V, H, A]) checkQuorumPrevotesGivenProposalVID(r types.Round, vID H) (hasQuorum bool) {
	prevotes, ok := t.messages.Prevotes[t.state.height][r]
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

func (t *stateMachine[V, H, A]) findProposal(r types.Round) *CachedProposal[V, H, A] {
	v, ok := t.messages.Proposals[t.state.height][r][t.validators.Proposer(t.state.height, r)]
	if !ok {
		return nil
	}

	return &CachedProposal[V, H, A]{
		Proposal: v,
		Valid:    t.application.Valid(*v.Value),
		ID:       utils.HeapPtr((*v.Value).Hash()),
	}
}
