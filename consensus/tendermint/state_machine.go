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

func (sm *StateMachine) Init() error {
	panic("implement me")
}

func (sm *StateMachine) Run() error {
	panic("implement me")
}

func (sm *StateMachine) startRound(round int64) error {
	panic("implement me")
}

func (sm *StateMachine) HandleMessage(msg Message) error {
	panic("implement me")
}
