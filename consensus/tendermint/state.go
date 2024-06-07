package tendermint

import consensus "github.com/NethermindEth/juno/consensus/common"

type Step = int8
type RoundType = consensus.RoundType
type HeightType = consensus.HeightType

const (
	STEP_PROPOSE   Step = 0
	STEP_PREVOTE   Step = 1
	STEP_PRECOMMIT Step = 2

	ROUND_NONE int64 = -1
)

var (
	VOTE_NONE consensus.Proposable = nil
)

type State struct {
	step             Step
	currentHeight    HeightType
	round            RoundType
	lockedValue      consensus.Proposable
	lockedRound      RoundType
	validValue       consensus.Proposable
	validRound       RoundType
	decider          consensus.Decider
	isFirstPreVote   bool
	isFirstPreCommit bool
}

func checkStep(step Step) {
	//if step != STEP_PROPOSE || step != STEP_PREVOTE || step != STEP_PRECOMMIT {
	//	panic("Invalid - invalid step")
	//}
}

func checkDecider(decider consensus.Decider) {
	if decider == nil {
		panic("state builder: decider is missing")
	}
}

func checkRound(round RoundType) {
	if round < 0 {
		panic("state builder: invalid round, round can not be negative")
	}
}

func InitialState(decider consensus.Decider) *State {
	return initialStateWithHeight(0, decider)
}

func initialStateWithHeight(height HeightType, decider consensus.Decider) *State {
	return newState(STEP_PROPOSE, height, 0, nil, nil, -1, -1,
		true, true, decider)
}

func newState(step Step, height HeightType, round RoundType, lockedValue, validValue consensus.Proposable,
	lockedRound, validRound RoundType, firstPreVote, firstPreCommit bool, decider consensus.Decider) *State {

	checkStep(step)
	checkDecider(decider)
	checkRound(round)

	return &State{
		step:             step,
		currentHeight:    height,
		round:            round,
		lockedValue:      lockedValue,
		validValue:       validValue,
		validRound:       validRound,
		lockedRound:      lockedRound,
		decider:          decider,
		isFirstPreVote:   firstPreVote,
		isFirstPreCommit: firstPreCommit,
	}
}

func (s *State) Builder() *StateBuilder {
	return newStateBuilderFromState(s)
}

func (s *State) nextHeight() HeightType {
	return s.currentHeight + 1
}

func (s *State) nextRound() RoundType {
	return s.round + 1
}

type StateBuilder struct {
	state *State
}

func newStateBuilderFromState(state *State) *StateBuilder {
	cpyState := *state
	return &StateBuilder{state: &cpyState} // a copy
}

func NewStateBuilder(decider consensus.Decider) *StateBuilder {
	return &StateBuilder{state: InitialState(decider)} // todo should be a copy
}

func (sb *StateBuilder) Build() *State {
	checkDecider(sb.state.decider)
	checkStep(sb.state.step)
	checkRound(sb.state.round)

	cpyState := *sb.state
	return &cpyState // calling build twice on the same builder should return two different states so actually create a new state here, using state copy constructor>?
}

func (sb *StateBuilder) SetDecider(decider consensus.Decider) *StateBuilder {
	sb.state.decider = decider
	return sb
}

func (sb *StateBuilder) SetHeight(height HeightType) *StateBuilder {
	sb.state.currentHeight = height
	return sb
}

func (sb *StateBuilder) SetStep(step Step) *StateBuilder {
	sb.state.step = step
	return sb
}

func (sb *StateBuilder) SetRound(round RoundType) *StateBuilder {
	sb.state.round = round
	return sb
}

func (sb *StateBuilder) SetLockedValue(lockedValue consensus.Proposable) *StateBuilder {
	sb.state.lockedValue = lockedValue
	return sb
}

func (sb *StateBuilder) SetValidValue(validValue consensus.Proposable) *StateBuilder {
	sb.state.validValue = validValue
	return sb
}

func (sb *StateBuilder) SetLockedRound(lockedRound RoundType) *StateBuilder {
	sb.state.lockedRound = lockedRound
	return sb
}

func (sb *StateBuilder) SetValidRound(validRound RoundType) *StateBuilder {
	sb.state.validRound = validRound
	return sb
}

func (sb *StateBuilder) SetIsFirstPreVote(isFirstPreVote bool) *StateBuilder {
	sb.state.isFirstPreVote = isFirstPreVote
	return sb
}

func (sb *StateBuilder) SetIsFirstPreCommit(isFirstPreCommit bool) *StateBuilder {
	sb.state.isFirstPreCommit = isFirstPreCommit
	return sb
}
