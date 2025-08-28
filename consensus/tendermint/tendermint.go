package tendermint

import (
	"fmt"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/votecounter"
	"github.com/NethermindEth/juno/utils"
)

//go:generate mockgen -destination=../mocks/mock_application.go -package=mocks github.com/NethermindEth/juno/consensus/tendermint Application
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
	ReplayWAL()
	ProcessStart(types.Round) []types.Action[V, H, A]
	ProcessTimeout(types.Timeout) []types.Action[V, H, A]
	ProcessProposal(*types.Proposal[V, H, A]) []types.Action[V, H, A]
	ProcessPrevote(*types.Prevote[H, A]) []types.Action[V, H, A]
	ProcessPrecommit(*types.Precommit[H, A]) []types.Action[V, H, A]
}

type stateMachine[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	db         db.TendermintDB[V, H, A]
	log        utils.Logger
	replayMode bool

	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?

	voteCounter votecounter.VoteCounter[V, H, A]

	application Application[V, H]
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
	db db.TendermintDB[V, H, A],
	log utils.Logger,
	nodeAddr A,
	app Application[V, H],
	vals votecounter.Validators[A],
	height types.Height,
) StateMachine[V, H, A] {
	return &stateMachine[V, H, A]{
		db:       db,
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

func (s *stateMachine[V, H, A]) startRound(r types.Round) types.Action[V, H, A] {
	if err := s.db.Flush(); err != nil {
		s.log.Fatalf("failed to flush WAL at start of round", "height", s.state.height, "round", r, "err", err)
	}

	s.resetState(r)

	if p := s.voteCounter.Proposer(r); p == s.nodeAddr {
		var proposalValue *V
		if s.state.validValue != nil {
			proposalValue = s.state.validValue
		} else {
			proposalValue = utils.HeapPtr(s.application.Value())
		}
		actions := s.sendProposal(proposalValue)
		if err := s.db.Flush(); err != nil {
			s.log.Fatalf("failed to flush WAL when proposing a new block", "height", s.state.height, "round", r, "err", err)
		}
		return actions
	} else {
		return s.scheduleTimeout(types.StepPropose)
	}
}

func (t *stateMachine[V, H, A]) scheduleTimeout(s types.Step) types.Action[V, H, A] {
	return utils.HeapPtr(
		types.ScheduleTimeout{
			Step:   s,
			Height: t.state.height,
			Round:  t.state.round,
		},
	)
}

// - Messages from past heights are ignored.
// - All messages from current and future heights are stored, but only processed when the height is the current height.
func (s *stateMachine[V, H, A]) preprocessMessage(header types.MessageHeader[A], addMessage func()) bool {
	if header.Height < s.state.height || header.Round < 0 {
		return false
	}

	addMessage()
	return header.Height == s.state.height
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

// ReplayWAL replays all WAL (Write-Ahead Log) messages for the current block height
// from persistent storage and applies them to the internal state. This is used for
// recovering consensus state after a crash or restart.
//
// ReplayWAL must not trigger any external effects such as broadcasting messages or
// scheduling timeouts. It is strictly a state recovery mechanism.
//
// Panics if the replaying the messages fails for whatever reason.
func (s *stateMachine[V, H, A]) ReplayWAL() {
	s.replayMode = true
	for walEntry, err := range s.db.GetWALEntries(s.state.height) {
		// TODO: panic here is wrong, but this will be rewritten in the next PR.
		if err != nil {
			panic(fmt.Errorf("ReplayWAL: failed to retrieve WAL messages for height %d: %w", s.state.height, err))
		}

		switch walEntry.Type {
		case types.MessageTypeProposal:
			proposal, ok := (walEntry.Entry).(*types.Proposal[V, H, A])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to proposal")
			}
			s.ProcessProposal(proposal)
		case types.MessageTypePrevote:
			prevote, ok := (walEntry.Entry).(*types.Prevote[H, A])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to prevote")
			}
			s.ProcessPrevote(prevote)
		case types.MessageTypePrecommit:
			precommit, ok := (walEntry.Entry).(*types.Precommit[H, A])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to precommit")
			}
			s.ProcessPrecommit(precommit)
		case types.MessageTypeTimeout:
			timeout, ok := (walEntry.Entry).(types.Timeout)
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to precommit")
			}
			s.ProcessTimeout(timeout)
		default:
			panic("Failed to replay WAL messages, unknown WAL Entry type")
		}
	}
	s.replayMode = false
}
