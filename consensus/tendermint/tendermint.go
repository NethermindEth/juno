package tendermint

import (
	"fmt"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type Application[V types.Hashable] interface {
	// Value returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Blockchain[V types.Hashable] interface {
	// types.Height return the current blockchain height
	Height() types.Height

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(types.Height, V)
}

type Validators interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(types.Height) types.VotingPower

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(types.Addr) types.VotingPower

	// Proposer returns the proposer of the current round and height.
	Proposer(types.Height, types.Round) types.Addr
}

type Slasher[M types.Message[V], V types.Hashable] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

//go:generate mockgen -destination=../mocks/mock_state_machine.go -package=mocks github.com/NethermindEth/juno/consensus/tendermint StateMachine
type StateMachine[V types.Hashable] interface {
	ReplayWAL()
	ProcessStart(types.Round) []types.Action[V]
	ProcessTimeout(types.Timeout) []types.Action[V]
	ProcessProposal(types.Proposal[V]) []types.Action[V]
	ProcessPrevote(types.Prevote) []types.Action[V]
	ProcessPrecommit(types.Precommit) []types.Action[V]
}

type stateMachine[V types.Hashable] struct {
	db         db.TendermintDB[V]
	log        utils.Logger
	replayMode bool

	nodeAddr types.Addr

	state state[V] // Todo: Does state need to be protected?

	messages types.Messages[V]

	application Application[V]
	blockchain  Blockchain[V]
	validators  Validators
}

type state[V types.Hashable] struct {
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

func New[V types.Hashable](
	db db.TendermintDB[V],
	log utils.Logger,
	nodeAddr types.Addr,
	app Application[V],
	chain Blockchain[V],
	vals Validators,
) StateMachine[V] {
	return &stateMachine[V]{
		db:       db,
		log:      log,
		nodeAddr: nodeAddr,
		state: state[V]{
			height:      chain.Height(),
			lockedRound: -1,
			validRound:  -1,
		},
		messages:    types.NewMessages[V](),
		application: app,
		blockchain:  chain,
		validators:  vals,
	}
}

type CachedProposal[V types.Hashable] struct {
	types.Proposal[V]
	Valid bool
	ID    *types.Hash
}

func (t *stateMachine[V]) startRound(r types.Round) types.Action[V] {
	if err := t.db.Flush(); err != nil {
		t.log.Fatalf("failed to flush WAL at start of round", "round", r, "height", t.blockchain.Height(), "err", err)
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
			t.log.Fatalf("failed to flush WAL when propsing a new block", "round", r, "height", t.blockchain.Height(), "err", err)
		}
		return actions
	} else {
		return t.scheduleTimeout(types.StepPropose)
	}
}

func (t *stateMachine[V]) scheduleTimeout(s types.Step) types.Action[V] {
	return utils.HeapPtr(
		types.ScheduleTimeout{
			Step:   s,
			Height: t.state.height,
			Round:  t.state.round,
		},
	)
}

func (t *stateMachine[V]) validatorSetVotingPower(vals []types.Addr) types.VotingPower {
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
func (t *stateMachine[V]) preprocessMessage(header types.MessageHeader, addMessage func()) bool {
	if header.Height < t.state.height || header.Round < 0 {
		return false
	}

	addMessage()
	return header.Height == t.state.height
}

// TODO: Improve performance. Current complexity is O(n).
func (t *stateMachine[V]) checkForQuorumPrecommit(r types.Round, vID types.Hash) (matchingPrecommits []types.Precommit, hasQuorum bool) {
	precommits, ok := t.messages.Precommits[t.state.height][r]
	if !ok {
		return nil, false
	}

	var vals []types.Addr
	for addr, p := range precommits {
		if p.ID != nil && *p.ID == vID {
			matchingPrecommits = append(matchingPrecommits, p)
			vals = append(vals, addr)
		}
	}
	return matchingPrecommits, t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))
}

// TODO: Improve performance. Current complexity is O(n).
func (t *stateMachine[V]) checkQuorumPrevotesGivenProposalVID(r types.Round, vID types.Hash) (hasQuorum bool) {
	prevotes, ok := t.messages.Prevotes[t.state.height][r]
	if !ok {
		return false
	}

	var vals []types.Addr
	for addr, p := range prevotes {
		if p.ID != nil && *p.ID == vID {
			vals = append(vals, addr)
		}
	}
	return t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(t.state.height))
}

func (t *stateMachine[V]) findProposal(r types.Round) *CachedProposal[V] {
	v, ok := t.messages.Proposals[t.state.height][r][t.validators.Proposer(t.state.height, r)]
	if !ok {
		return nil
	}

	return &CachedProposal[V]{
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
func (t *stateMachine[V]) ReplayWAL() {
	height := t.blockchain.Height()
	walEntries, err := t.db.GetWALEntries(height)
	if err != nil {
		panic(fmt.Errorf("ReplayWAL: failed to retrieve WAL messages for height %d: %w", height, err))
	}
	t.replayMode = true
	for _, walEntry := range walEntries {
		switch walEntry.Type {
		case types.MessageTypeProposal:
			proposal, ok := (walEntry.Entry).(types.Proposal[V])
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to proposal")
			}
			t.ProcessProposal(proposal)
		case types.MessageTypePrevote:
			prevote, ok := (walEntry.Entry).(types.Prevote)
			if !ok {
				panic("failed to replay WAL, failed to cast WAL Entry to prevote")
			}
			t.ProcessPrevote(prevote)
		case types.MessageTypePrecommit:
			precommit, ok := (walEntry.Entry).(types.Precommit)
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
