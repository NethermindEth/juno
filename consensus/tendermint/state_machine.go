package tendermint

import consensus "github.com/NethermindEth/juno/consensus/common"

type Step interface {
	NextStep() interface{}
}

type Propose struct{}

func (p *Propose) NextStep() interface{} {
	return PreVote{}
}

type PreVote struct{}

func (p *PreVote) NextStep() interface{} {
	return PreCommit{}
}

type PreCommit struct{}

func (p *PreCommit) NextStep() interface{} {
	return nil
}

type State struct {
	step          Step
	currentHeight uint64
	lockedValue   *consensus.Proposable
	lockedRound   int64
	validValue    *consensus.Proposable
	validRound    int64
	decider       *consensus.Decider
}

func InitialState() *State {
	panic("implement me")
	return newState()
}

func newState() *State {
	panic("implement me")
}

func (s State) CopyWith(map[string]interface{}) *State {
	panic("implement me")
}

func (s State) NextHeight() uint64 {
	panic("implement me")
}

type StateMachine struct {
	state State
}

func NewStateMachine() *StateMachine {
	// initial state
	// timeout call back map
	// timeout time returning function
	panic("implement me")
}

func (sm *StateMachine) Init() error {
	panic("implement me")
}

func (sm *StateMachine) Run() error {
	panic("implement me")
}

func (sm *StateMachine) HandleMessage(msg Message) error {
	panic("implement me")
}

func (sm *StateMachine) startRound(round uint64) error {
	panic("implement me")
}

func (sm *StateMachine) onTimeoutPropose(height, round uint64) error {
	panic("implement me")
}

func (sm *StateMachine) onTimeoutPreVote(height, round uint64) error {
	panic("implement me")
}

func (sm *StateMachine) onTimeoutPreCommit(height, round uint64) error {
	panic("implement me")
}

func (sm *StateMachine) timeOutTime(round uint64) {
	panic("implement me")
}
