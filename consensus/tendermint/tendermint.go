package tendermint

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"go.uber.org/zap"
)

type Application[V types.Hashable[H], H types.Hash] interface {
	// Value returns the value to the Tendermint consensus algorithm which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Slasher[M types.Message[V, H, A], V types.Hashable[H], H types.Hash, A types.Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

//go:generate mockgen -destination=../mocks/mock_state_machine.go -package=mocks github.com/NethermindEth/juno/consensus/tendermint StateMachine
type StateMachine[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	Height() types.Height
	ProcessStart(types.Round) []actions.Action[V, H, A]
	ProcessTimeout(types.Timeout) []actions.Action[V, H, A]
	ProcessProposal(*types.Proposal[V, H, A]) []actions.Action[V, H, A]
	ProcessPrevote(*types.Prevote[H, A]) []actions.Action[V, H, A]
	ProcessPrecommit(*types.Precommit[H, A]) []actions.Action[V, H, A]
	ProcessWAL(wal.Entry[V, H, A]) []actions.Action[V, H, A]
	ProcessSync(*types.Proposal[V, H, A], []types.Precommit[H, A]) []actions.Action[V, H, A]
}

type stateMachine[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log             utils.Logger
	nodeAddr        A
	state           state[V, H] // Todo: Does state need to be protected?
	voteCounter     votecounter.VoteCounter[V, H, A]
	application     Application[V, H]
	isHeightStarted bool
	lastTriggerSync types.Height
	lastQuorum      types.Height
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
	log utils.Logger,
	nodeAddr A,
	app Application[V, H],
	vals votecounter.Validators[A],
	height types.Height,
) StateMachine[V, H, A] {
	return &stateMachine[V, H, A]{
		log:      log,
		nodeAddr: nodeAddr,
		state: state[V, H]{
			height:      height,
			lockedRound: -1,
			validRound:  -1,
		},
		voteCounter: votecounter.New[V](vals, height),
		application: app,
	}
}

func (s *stateMachine[V, H, A]) Height() types.Height {
	return s.state.height
}

type CachedProposal[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	types.Proposal[V, H, A]
	Valid bool
	ID    *H
}

func (s *stateMachine[V, H, A]) resetState(round types.Round) {
	s.state.round = round
	s.state.step = types.StepPropose

	s.state.timeoutPrevoteScheduled = false
	s.state.lockedValueAndOrValidValueSet = false
	s.state.timeoutPrecommitScheduled = false
}

func (s *stateMachine[V, H, A]) startRound(r types.Round) actions.Action[V, H, A] {
	s.resetState(r)

	if r > 0 {
		proposer := felt.Felt(s.voteCounter.Proposer(r - 1))
		s.log.Debug(
			"Failed round",
			zap.Uint("height", uint(s.state.height)),
			zap.Int("round", int(r-1)),
			zap.Stringer("proposer", &proposer),
		)
	}

	if p := s.voteCounter.Proposer(r); p == s.nodeAddr {
		var proposalValue *V
		if s.state.validValue != nil {
			proposalValue = s.state.validValue
		} else {
			proposalValue = utils.HeapPtr(s.application.Value())
		}
		actions := s.sendProposal(proposalValue)
		return actions
	} else {
		return s.scheduleTimeout(types.StepPropose)
	}
}

func (t *stateMachine[V, H, A]) scheduleTimeout(s types.Step) actions.Action[V, H, A] {
	return utils.HeapPtr(
		actions.ScheduleTimeout{
			Step:   s,
			Height: t.state.height,
			Round:  t.state.round,
		},
	)
}

func (s *stateMachine[V, H, A]) findProposal(r types.Round) *CachedProposal[V, H, A] {
	proposal := s.voteCounter.GetProposal(r)
	if proposal == nil {
		return nil
	}

	return &CachedProposal[V, H, A]{
		Proposal: *proposal,
		Valid:    s.application.Valid(*proposal.Value),
		ID:       utils.HeapPtr((*proposal.Value).Hash()),
	}
}
