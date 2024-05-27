package tendermint

import (
	"errors"
	consensus "github.com/NethermindEth/juno/consensus/common"
	"sync"
	"time"
)

type Step = int8

const (
	STEP_PROPOSE   Step = 0
	STEP_PREVOTE   Step = 1
	STEP_PRECOMMIT Step = 2

	ROUND_NONE int64 = -1
)

var (
	VOTE_NONE *consensus.Proposable = nil
)

// State Todo: locked/validRound can be as large as round so uint vs int might be a bad idea maybe use uint and set to nil for negative value?
type State struct {
	step           Step
	currentHeight  uint64
	round          uint64
	lockedValue    *consensus.Proposable
	lockedRound    int64
	validValue     *consensus.Proposable
	validRound     int64
	decider        *consensus.Decider
	firstPreVote   bool // handle this
	firstPreCommit bool
}

func InitialState(decider *consensus.Decider) *State {
	return initialStateWithHeight(0, decider)
}

func initialStateWithHeight(height uint64, decider *consensus.Decider) *State {
	return newState(STEP_PROPOSE, height, height, nil, nil, -1, -1, true, true, decider)
}

func newState(step Step, height, round uint64, lockedValue, validValue *consensus.Proposable, lockedRound, validRound int64, firstPreVote, firstPreCommit bool, decider *consensus.Decider) *State {
	return &State{
		step:           step,
		currentHeight:  height,
		round:          round,
		lockedValue:    lockedValue,
		validValue:     validValue,
		validRound:     validRound,
		lockedRound:    lockedRound,
		decider:        decider,
		firstPreVote:   firstPreVote,
		firstPreCommit: firstPreCommit,
	}
}

func (s State) CopyWith(map[string]interface{}) *State {
	panic("implement me")
}

func (s State) nextHeight() uint64 {
	return s.currentHeight + 1
}

func (s State) nextRound() uint64 {
	return s.round + 1
}

type StateMachine struct {
	state    State
	proposer consensus.Proposer
	gossiper consensus.Gossiper
	decider  consensus.Decider
	lock     sync.Mutex // locked needed because of timeouts!!!
	// set timeout functions here
	// set timing functions here
}

func NewStateMachine(decider consensus.Decider, initialState *State) *StateMachine {
	// initial state
	// timeout call back map
	// timeout time returning function

	if initialState == nil {
		initialState = InitialState(&decider)
	}
	return &StateMachine{
		state:   *initialState,
		decider: decider,
	}
}

func (sm *StateMachine) start() {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	sm.startRound(0)
}

func toMessages(p interface{}) []Message {
	panic("implement me")
}

func (sm *StateMachine) Run() {
	sm.start()
	for {
		msg := sm.gossiper.ReceiveMessage()
		msgs := toMessages(msg)
		err := sm.HandleMessage(msgs)
		if err != nil {
			//todo: log error
		}
	}
}

func (sm *StateMachine) HandleMessage(msgs []Message) error {
	// order message in proposal, prevote, precommit, Empty order

	invalidMsg := true

	if msgs[0].msgType == MSG_PROPOSAL && msgs[1].msgType == MSG_PREVOTE {
		invalidMsg = false

		sm.handleProposalsWithPreVotes(msgs)
		return nil
	}
	if msgs[0].msgType == MSG_PROPOSAL && msgs[1].msgType == MSG_PRECOMMIT {
		invalidMsg = false

		sm.handleProposalsWithPreCommits(msgs)
		return nil
	}

	if msgs[0].msgType == MSG_PROPOSAL && msgs[1].msgType == MSG_EMPTY {
		invalidMsg = false

		sm.handleJustProposals(msgs)
		return nil
	}

	// TODO: remove not a case
	if msgs[0].msgType == MSG_PREVOTE && msgs[1].msgType == MSG_PRECOMMIT {
		invalidMsg = false

		sm.handlePreVotesWithPreCommits(msgs)
		return nil
	}

	if msgs[0].msgType == MSG_PREVOTE && msgs[1].msgType == MSG_EMPTY {
		invalidMsg = false

		sm.handleJustPreVotes(msgs)
		return nil
	}

	if msgs[0].msgType == MSG_PRECOMMIT && msgs[1].msgType == MSG_EMPTY {
		invalidMsg = false

		sm.handleJustPreCommits(msgs)
		return nil
	}

	if msgs[0].msgType == MSG_UNIQUE_VOTES && msgs[1].msgType == MSG_UNIQUE_VOTES {
		invalidMsg = false

		sm.handleUniqueVotes(msgs)
		return nil
	}

	if invalidMsg {
		return errors.New("invalid msgs type and or transition case")
	}

	return nil
}

func (sm *StateMachine) handleProposalsWithPreVotes(msg []Message) {

	sm.handleProposalPreVoting(msg)
	sm.handleInitialVoting(msg)
}

func (sm *StateMachine) checkValidVoting(msg []Message) bool {
	if msg[0].Sender() != sm.proposer.Proposer(sm.state.currentHeight, sm.state.round) && msg[1].VoteLevel() >= VOTE_LEVEL_MAJORITY {
		return false
	}

	if msg[0].Height() != sm.state.currentHeight && msg[0].Height() != msg[1].Height() {
		return false
	}
	if (*msg[0].Value()).Id() != (*msg[1].Value()).Id() {
		return false
	}
	return true
}

func (sm *StateMachine) handleProposalPreVoting(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if !sm.checkValidVoting(msg) {
		return
	}

	if msg[0].LastValidRound() != msg[1].LastValidRound() {
		return
	}

	lastValidRoundRecv := msg[0].LastValidRound()
	if sm.state.step == STEP_PROPOSE && (lastValidRoundRecv >= 0 && lastValidRoundRecv < sm.state.validRound) {
		value := msg[0].Value()
		if (*value).IsValid() && (sm.state.lockedRound <= lastValidRoundRecv || (*value).EqualsTo(*sm.state.lockedValue)) {
			sm.gossiper.SubmitMessageForBroadcast(NewPreVoteMessage(sm.state.currentHeight, sm.state.round, value)) // will get the id from the value
		} else {
			sm.gossiper.SubmitMessageForBroadcast(NewPreVoteMessage(sm.state.currentHeight, sm.state.round, nil))
		}

		params := make(map[string]interface{})
		params["step"] = STEP_PREVOTE
		sm.state = *sm.state.CopyWith(params)
	}
}

func (sm *StateMachine) handleInitialVoting(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if !sm.checkValidVoting(msg) {
		return
	}

	if msg[0].Round() != sm.state.round && msg[0].Round() != msg[1].Round() {
		return
	}

	value := msg[0].Value()
	// todo:  tricky condition for first time?!
	if (*value).IsValid() && sm.state.step >= STEP_PREVOTE && (sm.state.firstPreVote && sm.state.firstPreCommit) {
		params := make(map[string]interface{})

		if sm.state.step == STEP_PREVOTE {
			params["lockedValue"] = *value
			params["lockedRound"] = sm.state.round
			sm.gossiper.SubmitMessageForBroadcast(NewPreCommitMessage(sm.state.currentHeight, sm.state.round, value))
			params["step"] = STEP_PRECOMMIT
		}

		params["validValue"] = value
		params["validRound"] = sm.state.round

		if sm.state.firstPreVote {
			params["firstPreVote"] = false
		}

		if !sm.state.firstPreCommit {
			params["firstPreCommit"] = false
		}

		sm.state = *sm.state.CopyWith(params)
	}
}

func (sm *StateMachine) handleProposalsWithPreCommits(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if msg[1].VoteLevel() < VOTE_LEVEL_MAJORITY {
		return
	}
	if msg[0].Round() != msg[1].Round() {
		return
	}
	if msg[0].Height() != sm.state.currentHeight && msg[0].Height() != msg[1].Height() {
		return
	}
	if msg[0].Sender() != sm.proposer.Proposer(sm.state.currentHeight, msg[0].Round()) {
		return
	}
	value := msg[0].Value()

	if !(*value).EqualsTo(*msg[1].Value()) {
		return
	}

	if sm.decider.GetDecision(sm.state.currentHeight) != nil {
		if (*value).IsValid() {
			sm.decider.SubmitDecision(value, sm.state.currentHeight)
			sm.resetStateWithNewHeight(sm.state.nextHeight())
			sm.startRound(0)
		}
	}
}

func (sm *StateMachine) resetStateWithNewHeight(newHeight uint64) {
	sm.state = *initialStateWithHeight(newHeight, sm.state.decider)
	sm.gossiper.ClearReceive() // clears msgs processing state not all the received msgs
}

func (sm *StateMachine) handleJustProposals(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if msg[0].Sender() != sm.proposer.Proposer(sm.state.currentHeight, msg[0].Round()) {
		return
	}

	if msg[0].Height() != sm.state.currentHeight {
		return
	}

	if msg[0].Round() != sm.state.round {
		return
	}

	if msg[0].LastValidRound() != ROUND_NONE {
		return
	}

	if sm.state.step == STEP_PROPOSE {
		value := msg[0].Value()

		if (*value).IsValid() && (sm.state.lockedRound == ROUND_NONE || (*value).EqualsTo(*sm.state.lockedValue)) {
			sm.gossiper.SubmitMessageForBroadcast(NewPreVoteMessage(sm.state.currentHeight, sm.state.round, value))
		} else {
			sm.gossiper.SubmitMessageForBroadcast(NewPreVoteMessage(sm.state.currentHeight, sm.state.round, nil))
		}

		params := make(map[string]interface{})
		params["step"] = STEP_PREVOTE
		sm.state = *sm.state.CopyWith(params)
	}
}

func (sm *StateMachine) handlePreVotesWithPreCommits(msg []Message) {
	return
}

func (sm *StateMachine) handleJustPreVotes(msg []Message) {
	sm.handleInitialPreVote(msg)
	sm.handlePreVotes(msg)
}

func (sm *StateMachine) checkJustPreVoteMsg(msg []Message) bool {
	if msg[0].VoteLevel() < VOTE_LEVEL_MAJORITY {
		return false
	}

	if msg[0].Height() != sm.state.currentHeight {
		return false
	}

	if msg[0].Round() != sm.state.round {
		return false
	}

	return true
}

func (sm *StateMachine) handleInitialPreVote(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if !sm.checkJustPreVoteMsg(msg) {
		return
	}

	if sm.state.step == STEP_PREVOTE && sm.state.firstPreVote {
		setTimeOut(func() {
			sm.onTimeoutPreVote(sm.state.currentHeight, sm.state.round)
		}, sm.timeOutTime(sm.state.round, sm.state.step))

		params := make(map[string]interface{})
		params["firstPreVote"] = false
		sm.state = *sm.state.CopyWith(params)
	}

}

func (sm *StateMachine) handlePreVotes(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if !sm.checkJustPreVoteMsg(msg) {
		return
	}

	if msg[0].Value() != VOTE_NONE {
		return
	}

	if sm.state.step == STEP_PREVOTE {
		sm.gossiper.SubmitMessageForBroadcast(NewPreCommitMessage(sm.state.currentHeight, sm.state.round, VOTE_NONE))

		params := make(map[string]interface{})
		params["step"] = STEP_PRECOMMIT

		sm.state = *sm.state.CopyWith(params)
	}

}

func (sm *StateMachine) handleJustPreCommits(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if msg[0].VoteLevel() < VOTE_LEVEL_MAJORITY {
		return
	}

	if msg[0].Height() != sm.state.currentHeight {
		return
	}

	if msg[0].Round() != sm.state.round {
		return
	}

	if sm.state.firstPreCommit {
		setTimeOut(func() {
			sm.onTimeoutPreCommit(sm.state.currentHeight, sm.state.round)
		}, sm.timeOutTime(sm.state.round, sm.state.step))
	}
}

func (sm *StateMachine) handleUniqueVotes(msg []Message) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if msg[0].VoteLevel() < VOTE_LEVEL_MINORITY {
		return
	}

	if msg[0].Height() != sm.state.currentHeight {
		return
	}

	roundRecv := msg[0].Round()
	if roundRecv > sm.state.round {
		sm.startRound(roundRecv)
	}
}

func (sm *StateMachine) startRound(round uint64) {
	// no need for lock here,  would always be called within a locked process.
	params := make(map[string]interface{})
	params["round"] = round
	params["step"] = STEP_PROPOSE
	sm.state = *sm.state.CopyWith(params)

	var proposal consensus.Proposable = nil
	if sm.proposer.StrictIsProposer(sm.state.currentHeight, sm.state.round) {
		if sm.state.validValue != nil {
			proposal = *sm.state.validValue
		} else {
			proposal = sm.proposer.Propose(sm.state.currentHeight, sm.state.round)
		}
		sm.gossiper.SubmitMessageForBroadcast(proposal)
	} else {
		//Todo check for late variable binding on closure
		setTimeOut(func() {
			sm.onTimeoutPropose(sm.state.currentHeight, sm.state.round)
		}, sm.timeOutTime(sm.state.round, sm.state.step))
	}
}

func (sm *StateMachine) onTimeoutPropose(height, round uint64) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if height == sm.state.currentHeight && round == sm.state.round && sm.state.step == STEP_PROPOSE {
		sm.gossiper.SubmitMessageForBroadcast(NewPreVoteMessage(sm.state.currentHeight, sm.state.round, nil))
	}
	params := make(map[string]interface{})
	params["step"] = STEP_PREVOTE
	sm.state = *sm.state.CopyWith(params)
}

func (sm *StateMachine) onTimeoutPreVote(height, round uint64) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if height == sm.state.currentHeight && round == sm.state.round && sm.state.step == STEP_PREVOTE {
		sm.gossiper.SubmitMessageForBroadcast(NewPreCommitMessage(sm.state.currentHeight, sm.state.round, nil))
	}
	params := make(map[string]interface{})
	params["step"] = STEP_PRECOMMIT
	sm.state = *sm.state.CopyWith(params)
}

func (sm *StateMachine) onTimeoutPreCommit(height, round uint64) {
	sm.lock.Lock()
	defer sm.lock.Unlock()

	if height == sm.state.currentHeight && round == sm.state.round {
		sm.startRound(sm.state.nextRound()) // this method also locks
	}
}

func (sm *StateMachine) timeOutTime(round uint64, step Step) time.Duration {
	return 5 * time.Second
	// todo fix this
	//var timeOutKey string
	//switch step {
	//case STEP_PREVOTE:
	//	timeOutKey = "prevote_timeout"
	//case STEP_PRECOMMIT:
	//	timeOutKey = "precommit_timeout"
	//case STEP_PROPOSE:
	//	timeOutKey = "propose_timeout"
	//default:
	//	timeOutKey = "timeout"
	//}
	//
	//timeout := sm.configs[timeOutKey]
	//return time.Duration(timeout*time.Second + round*time.Second)
}

func setTimeOut(f func(), t time.Duration) {
	go func() {
		time.Sleep(t)
		f()
	}()
}
