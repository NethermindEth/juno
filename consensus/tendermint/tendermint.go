package tendermint

import (
	"fmt"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type Application[V types.Hashable[H], H types.Hash] interface {
	// Value returns the value to the Tendermint consensus algorithm which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
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
	ReplayWAL()
	ProcessStart(types.Round) []types.Action[V, H, A]
	ProcessTimeout(types.Timeout) []types.Action[V, H, A]
	ProcessProposal(types.Proposal[V, H, A]) []types.Action[V, H, A]
	ProcessPrevote(types.Prevote[H, A]) []types.Action[V, H, A]
	ProcessPrecommit(types.Precommit[H, A]) []types.Action[V, H, A]
}

type stateMachine[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	db         db.TendermintDB[V, H, A]
	log        utils.Logger
	replayMode bool

	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?

	messages types.Messages[V, H, A]

	application Application[V, H]
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
	db db.TendermintDB[V, H, A],
	log utils.Logger,
	nodeAddr A,
	app Application[V, H],
	vals Validators[A],
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
		messages:    types.NewMessages[V, H, A](),
		application: app,
		validators:  vals,
	}
}

type CachedProposal[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	types.Proposal[V, H, A]
	Valid bool
	ID    *H
}

func (t *stateMachine[V, H, A]) startRound(r types.Round) types.Action[V, H, A] {
	if err := t.db.Flush(); err != nil {
		t.log.Fatalf("failed to flush WAL at start of round", "height", t.state.height, "round", r, "err", err)
	}

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
		actions := t.sendProposal(proposalValue)
		if err := t.db.Flush(); err != nil {
			t.log.Fatalf("failed to flush WAL when proposing a new block", "height", t.state.height, "round", r, "err", err)
		}
		return actions
	} else {
		return t.scheduleTimeout(types.StepPropose)
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

// - Messages from past heights are ignored.
// - All messages from current and future heights are stored, but only processed when the height is the current height.
func (t *stateMachine[V, H, A]) preprocessMessage(header types.MessageHeader[A], addMessage func()) bool {
	if header.Height < t.state.height || header.Round < 0 {
		return false
	}

	addMessage()
	return header.Height == t.state.height
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

// ReplayWAL replays all WAL (Write-Ahead Log) messages for the current block height
// from persistent storage and applies them to the internal state. This is used for
// recovering consensus state after a crash or restart.
//
// ReplayWAL must not trigger any external effects such as broadcasting messages or
// scheduling timeouts. It is strictly a state recovery mechanism.
//
// Panics if the replaying the messages fails for whatever reason.
func (t *stateMachine[V, H, A]) ReplayWAL() {
	walEntries, err := t.db.GetWALEntries(t.state.height)
	if err != nil {
		panic(fmt.Errorf("ReplayWAL: failed to retrieve WAL messages for height %d: %w", t.state.height, err))
	}
	t.replayMode = true
	for _, walEntry := range walEntries {
		switch walEntry.Type {
		case types.MessageTypeProposal:
			proposal, ok := (walEntry.Entry).(types.Proposal[V, H, A])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to proposal")
			}
			t.ProcessProposal(proposal)
		case types.MessageTypePrevote:
			prevote, ok := (walEntry.Entry).(types.Prevote[H, A])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to prevote")
			}
			t.ProcessPrevote(prevote)
		case types.MessageTypePrecommit:
			precommit, ok := (walEntry.Entry).(types.Precommit[H, A])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to precommit")
			}
			t.ProcessPrecommit(precommit)
		case types.MessageTypeTimeout:
			timeout, ok := (walEntry.Entry).(types.Timeout)
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to precommit")
			}
			t.ProcessTimeout(timeout)
		default:
			panic("Failed to replay WAL messages, unknown WAL Entry type")
		}
	}
	t.replayMode = false
}
