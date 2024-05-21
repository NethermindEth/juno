package tendermint

type Step struct{}

type Propose struct {
	Step
}
type PreVote struct {
	Step
}
type PreCommit struct {
	Step
}

type State struct {
}

type StateMachine struct {
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
