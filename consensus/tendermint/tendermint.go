package tendermint

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

type step uint

const (
	propose step = iota
	prevote
	precommit
)

type timeoutFn func(round uint) time.Duration

type Addr interface {
	// Ethereum Addresses are 20 bytes
	~[20]byte | felt.Felt
}

type Hash interface {
	~[32]byte | felt.Felt
}

// Hashable's Hash() is used as ID()
type Hashable[H Hash] interface {
	Hash() H
}

type Application[V Hashable[H], H Hash] interface {
	// Value returns the value to the Tendermint consensus algorith which can be proposed to other validators.
	Value() V

	// Valid returns true if the provided value is valid according to the application context.
	Valid(V) bool
}

type Blockchain[V Hashable[H], H Hash, A Addr] interface {
	// Height return the current blockchain height
	Height() uint

	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(uint, V, []Precommit[H, A])
}

type Validators[A Addr] interface {
	// TotalVotingPower represents N which is required to calculate the thresholds.
	TotalVotingPower(height uint) uint

	// ValidatorVotingPower returns the voting power of the a single validator. This is also required to implement
	// various thresholds. The assumption is that a single validator cannot have voting power more than f.
	ValidatorVotingPower(validatorAddr A) uint

	// Proposer returns the proposer of the current round and height.
	Proposer(height, round uint) A
}

type Slasher[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Equivocation informs the slasher that a validator has sent conflicting messages. Thus it can decide whether to
	// slash the validator and by how much.
	Equivocation(msgs ...M)
}

type Listener[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Listen would return consensus messages to Tendermint which are set // by the validator set.
	Listen() <-chan M
}

type Broadcaster[M Message[V, H, A], V Hashable[H], H Hash, A Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(msg M)

	// SendMsg would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(validatorAddr A, msg M)
}

type Listeners[V Hashable[H], H Hash, A Addr] struct {
	ProposalListener  Listener[Proposal[V, H, A], V, H, A]
	PrevoteListener   Listener[Prevote[H, A], V, H, A]
	PrecommitListener Listener[Precommit[H, A], V, H, A]
}

type Broadcasters[V Hashable[H], H Hash, A Addr] struct {
	ProposalBroadcaster  Broadcaster[Proposal[V, H, A], V, H, A]
	PrevoteBroadcaster   Broadcaster[Prevote[H, A], V, H, A]
	PrecommitBroadcaster Broadcaster[Precommit[H, A], V, H, A]
}

type Tendermint[V Hashable[H], H Hash, A Addr] struct {
	nodeAddr A

	state state[V, H] // Todo: Does state need to be protected?
	// Todo: maintain a separate message set for future round messages.
	// Which are replayed when moving to the next round.
	messages messages[V, H, A]

	timeoutPropose   timeoutFn
	timeoutPrevote   timeoutFn
	timeoutPrecommit timeoutFn

	application Application[V, H]
	blockchain  Blockchain[V, H, A]
	validators  Validators[A]

	listeners    Listeners[V, H, A]
	broadcasters Broadcasters[V, H, A]

	scheduledTms []timeout
	timeoutsCh   chan timeout

	wg   sync.WaitGroup
	quit chan struct{}
}

type state[V Hashable[H], H Hash] struct {
	height uint
	round  uint
	step   step

	lockedValue *V
	validValue  *V

	// The default value of lockedRound and validRound is -1. However, using int for one value is not good use of space,
	// therefore, uint is used and nil would represent -1.
	lockedRound *uint
	validRound  *uint

	// The following are round level variable therefore when a round changes they must be reset.
	line34Executed bool
	line36Executed bool
	line47Executed bool
}

// Todo: Add Slashers later
func New[V Hashable[H], H Hash, A Addr](addr A, app Application[V, H], chain Blockchain[V, H, A], vals Validators[A],
	listeners Listeners[V, H, A], broadcasters Broadcasters[V, H, A], tmPropose, tmPrevote, tmPrecommit timeoutFn,
) *Tendermint[V, H, A] {
	return &Tendermint[V, H, A]{
		nodeAddr:         addr,
		state:            state[V, H]{height: chain.Height()},
		messages:         newMessages[V, H, A](),
		timeoutPropose:   tmPropose,
		timeoutPrevote:   tmPrevote,
		timeoutPrecommit: tmPrecommit,
		application:      app,
		blockchain:       chain,
		validators:       vals,
		listeners:        listeners,
		broadcasters:     broadcasters,
		scheduledTms:     make([]timeout, 0),
		timeoutsCh:       make(chan timeout),
		quit:             make(chan struct{}),
	}
}

//nolint:gocyclo,funlen
func (t *Tendermint[V, H, A]) Start() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		t.startRound(0)

		// Todo: check message signature everytime a message is received.
		// For the time being it can be assumed the signature is correct.
		i := 0
		for {
			fmt.Println("iteration: ", i)
			select {
			case <-t.quit:
				fmt.Println("quit", t.state.step)
				return
			case p := <-t.listeners.ProposalListener.Listen():
				if p.Height < t.state.height {
					continue
				}

				if p.Height > t.state.height { //nolint:staticcheck
					continue
				}

				if p.Round > t.state.round {
					// Todo: add the message to a future round messages and then replay them after arriving in that
					// round.
					t.line55(p.Round)
				}

				// The below shouldn't panic because it is expected Proposal is well-formed. However,
				// there need to be a way to distinguish between nil and zero value.
				// This is expected to be handled by the p2p layer.
				vID := (*p.Value).Hash()
				fmt.Println("got proposal with valueID and validRound", vID, p.ValidRound)
				validProposal := t.application.Valid(*p.Value)
				proposalFromProposer := p.Sender == t.validators.Proposer(p.Height, p.Round)
				vr := p.ValidRound

				if validProposal {
					// Add the proposal to the message set even if the sender is not the proposer,
					// this is because of slahsing purposes
					t.messages.addProposal(p)
				}

				_, prevotesForHR, precommitsForHR := t.messages.allMessages(p.Height, p.Round)

				var precommits []Precommit[H, A]
				var vals []A

				for addr, valPrecommits := range precommitsForHR {
					for _, p := range valPrecommits {
						if *p.ID == vID {
							precommits = append(precommits, p)
							vals = append(vals, addr)
						}
					}
				}

				/*
					Check the upon condition on line 49:

						49: upon {PROPOSAL, h_p, r, v, *} from proposer(h_p, r) AND 2f + 1 {PRECOMMIT, h_p, r, id(v)} while decision_p[h_p] = nil do
						50: 	if valid(v) then
						51: 		decisionp[hp] = v
						52: 		h_p ← h_p + 1
						53: 		reset lockedRound_p, lockedValue_p,	validRound_p and validValue_p to initial values and empty message log
						54: 		StartRound(0)

					 There is no need to check decision_p[h_p] = nil since it is implied that decision are made
					 sequentially, i.e. x, x+1, x+2... . The validity of the proposal value can be checked in the same if
					 statement since there is no else statement.
				*/
				if validProposal && proposalFromProposer &&
					t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
					// After committing the block, how the new height and round is started needs to be coordinated
					// with the synchronisation process.
					t.blockchain.Commit(t.state.height, *p.Value, precommits)

					// Todo: this may need to be changed depending how slashing is handled.
					t.messages.deleteOldMessages(t.state.height)
					t.state.height++
					t.startRound(0)

					continue
				}

				/*
					Check the upon condition on line 22:

						22: upon {PROPOSAL, h_p, round_p, v, nil} from proposer(h_p, round_p) while step_p = propose do
						23: 	if valid(v) ∧ (lockedRound_p = −1 ∨ lockedValue_p = v) then
						24: 		broadcast {PREVOTE, h_p, round_p, id(v)}
						25: 	else
						26: 		broadcast {PREVOTE, h_p, round_p, nil}
						27:		step_p ← prevote

					The implementation uses nil as -1 to avoid using int type.

					Since the value's id is expected to be unique the id can be used to compare the values.

				*/

				if vr == nil && proposalFromProposer && t.state.step == propose {
					vote := Prevote[H, A]{
						Vote: Vote[H, A]{
							Height: t.state.height,
							Round:  t.state.round,
							ID:     nil,
							Sender: t.nodeAddr,
						},
					}

					if validProposal && (t.state.lockedRound == nil || (*t.state.lockedValue).Hash() == vID) {
						vote.ID = &vID
					}

					t.messages.addPrevote(vote)
					t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
					t.state.step = prevote
				}

				/*
					Check the upon condition on  line 28:

						28: upon {PROPOSAL, h_p, round_p, v, vr} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE,h_p, vr, id(v)} while
							step_p = propose ∧ (vr ≥ 0 ∧ vr < round_p) do
						29: if valid(v) ∧ (lockedRound_p ≤ vr ∨ lockedValue_p = v) then
						30: 	broadcast {PREVOTE, hp, round_p, id(v)}
						31: else
						32:  	broadcast {PREVOTE, hp, round_p, nil}
						33: step_p ← prevote

				*/

				if vr != nil && proposalFromProposer && t.state.step == propose {
					_, prevotesForHVr, _ := t.messages.allMessages(p.Height, *vr)

					var vals []A

					for addr, valPrevotes := range prevotesForHVr {
						for _, p := range valPrevotes {
							if *p.ID == vID {
								vals = append(vals, addr)
							}
						}
					}

					if t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) && *vr >= uint(0) && *vr < t.state.round {
						vote := Prevote[H, A]{
							Vote: Vote[H, A]{
								Height: t.state.height,
								Round:  t.state.round,
								ID:     nil,
								Sender: t.nodeAddr,
							},
						}

						if validProposal && (*t.state.lockedRound >= *vr || (*t.state.lockedValue).Hash() == vID) {
							vote.ID = &vID
						}

						t.messages.addPrevote(vote)
						t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
						t.state.step = prevote
					}
				}

				/*
					Check upon condition on line 36:

						36: upon {PROPOSAL, h_p, round_p, v, ∗} from proposer(h_p, round_p) AND 2f + 1 {PREVOTE, h_p, round_p, id(v)} while
							valid(v) ∧ step_p ≥ prevote for the first time do
						37: if step_p = prevote then
						38: 	lockedValue_p ← v
						39: 	lockedRound_p ← round_p
						40: 	broadcast {PRECOMMIT, h_p, round_p, id(v))}
						41: 	step_p ← precommit
						42: validValue_p ← v
						43: validRound_p ← round_p
				*/

				if validProposal && t.state.step >= prevote {
					// line 36: 36: upon hPROPOSAL, hp, roundp, v, ∗i from proposer(hp, roundp) AND 2f + 1 hPREVOTE, hp, roundp, id(v)i while
					// valid(v) ∧ stepp ≥ prevote for the first time do
					var vals []A

					for addr, valPrevotes := range prevotesForHR {
						for _, p := range valPrevotes {
							if *p.ID == vID {
								vals = append(vals, addr)
							}
						}
					}

				}

			case p := <-t.listeners.PrevoteListener.Listen():
				if p.Height < t.state.height {
					continue
				}

				if p.Height > t.state.height { //nolint:staticcheck
					continue
				}

				if p.Round > t.state.round {
					t.line55(p.Round)
				}

				fmt.Println("got prevote")
				t.messages.addPrevote(p)

			case p := <-t.listeners.PrecommitListener.Listen():
				if p.Height < t.state.height {
					continue
				}

				if p.Height > t.state.height { //nolint:staticcheck
					continue
				}

				if p.Round > t.state.round {
					t.line55(p.Round)
				}

				fmt.Println("got precommmit")
				t.messages.addPrecommit(p)

				proposalsForHR, _, precommitsForHR := t.messages.allMessages(p.Height, p.Round)

				/*
					Check the upon condition on line 49:

						49: upon {PROPOSAL, h_p, r, v, *} from proposer(h_p, r) AND 2f + 1 {PRECOMMIT, h_p, r, id(v)} while decision_p[h_p] = nil do
						50: 	if valid(v) then
						51: 		decisionp[hp] = v
						52: 		h_p ← h_p + 1
						53: 		reset lockedRound_p, lockedValue_p,	validRound_p and validValue_p to initial values and empty message log
						54: 		StartRound(0)

					Fetching the relevant proposal implies the sender of the proposal was the proposer for that
					height and round. Also, since only the proposals with valid value are added to the message set, the
					validity of the proposal can be skipped.

					There is no need to check decision_p[h_p] = nil since it is implied that decision are made
					sequentially, i.e. x, x+1, x+2... .

				*/

				if p.ID != nil {
					var (
						proposal   Proposal[V, H, A]
						precommits []Precommit[H, A]
						vals       []A
					)

					for _, prop := range proposalsForHR[t.validators.Proposer(p.Height, p.Round)] {
						if (*prop.Value).Hash() == *p.ID {
							proposal = prop
						}
					}

					for addr, valPrecommits := range precommitsForHR {
						for _, precommit := range valPrecommits {
							if *precommit.ID == *p.ID {
								precommits = append(precommits, precommit)
								vals = append(vals, addr)
							}
						}
					}
					if t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
						t.blockchain.Commit(t.state.height, *proposal.Value, precommits)

						t.messages.deleteOldMessages(t.state.height)
						t.state.height++
						t.startRound(0)

						continue
					}
				}

				/*
					Check the upon condition on line 47:

						47: upon 2f + 1 {PRECOMMIT, h_p, round_p, ∗} for the first time do
						48: schedule OnTimeoutPrecommit(h_p , round_p) to be executed after timeoutPrecommit(round_p)
				*/

				vals := slices.Collect(maps.Keys(precommitsForHR))
				if !t.state.line47Executed && t.validatorSetVotingPower(vals) >= q(t.validators.TotalVotingPower(p.Height)) {
					t.scheduleTimeout(t.timeoutPrecommit(p.Round), precommit, p.Height, p.Round)
					t.state.line47Executed = true
				}

			case tm := <-t.timeoutsCh:
				fmt.Println("Inside timeout case", tm)
				switch tm.s {
				case propose:
					t.OnTimeoutPropose(tm.h, tm.r)
				case prevote:
					t.OnTimeoutPrevote(tm.h, tm.r)
				case precommit:
					t.OnTimeoutPrecommit(tm.h, tm.r)
				}

				i := slices.Index(t.scheduledTms, tm)
				t.scheduledTms = slices.Delete(t.scheduledTms, i, i+1)
			}
			i++
		}
	}()
}

func (t *Tendermint[V, H, A]) Stop() {
	fmt.Println("Stopping Tendermint...")
	close(t.quit)
	for _, tm := range t.scheduledTms {
		tm.Stop()
	}
	fmt.Println("Waiting fo the Tendermint loop to exit")
	t.wg.Wait()
}

func (t *Tendermint[V, H, A]) startRound(r uint) {
	fmt.Println("Inside start round", r)
	if r != 0 && r <= t.state.round {
		return
	}

	t.state.round = r
	t.state.step = propose

	t.state.line34Executed = false
	t.state.line36Executed = false
	t.state.line47Executed = false

	if p := t.validators.Proposer(t.state.height, r); p == t.nodeAddr {
		var proposalValue *V
		if t.state.validValue != nil {
			proposalValue = t.state.validValue
		} else {
			v := t.application.Value()
			proposalValue = &v
		}
		proposalMessage := Proposal[V, H, A]{
			Height:     t.state.height,
			Round:      r,
			ValidRound: t.state.validRound,
			Value:      proposalValue,
			Sender:     t.nodeAddr,
		}

		t.messages.addProposal(proposalMessage)
		fmt.Println("Broadcasting the proposal")
		t.broadcasters.ProposalBroadcaster.Broadcast(proposalMessage)
	} else {
		t.scheduleTimeout(t.timeoutPropose(r), propose, t.state.height, t.state.round)
	}

	// Todo: write a new function startFutureRound which checks for line 22 and 28 incase prevote or precommit where
	// the cause of the round change and the proposal for the future round was received prior future round change.
	// Start a goroutine with which sends the messages to the
}

type timeout struct {
	*time.Timer

	s step
	h uint
	r uint
}

func (t *Tendermint[V, H, A]) scheduleTimeout(duration time.Duration, s step, h, r uint) {
	tm := timeout{s: s, h: h, r: r}
	tm.Timer = time.AfterFunc(duration, func() {
		select {
		case <-t.quit:
		case t.timeoutsCh <- tm:
		}
	})
	t.scheduledTms = append(t.scheduledTms, tm)
}

func (t *Tendermint[_, H, A]) OnTimeoutPropose(h, r uint) {
	if t.state.height == h && t.state.round == r && t.state.step == propose {
		vote := Prevote[H, A]{
			Vote: Vote[H, A]{
				Height: t.state.height,
				Round:  t.state.round,
				ID:     nil,
				Sender: t.nodeAddr,
			},
		}
		t.messages.addPrevote(vote)
		fmt.Println("About to send prevot nil")
		t.broadcasters.PrevoteBroadcaster.Broadcast(vote)
		t.state.step = prevote
		fmt.Println("Set the step to prevote")
	}
}

func (t *Tendermint[_, H, A]) OnTimeoutPrevote(h, r uint) {
	if t.state.height == h && t.state.round == r && t.state.step == prevote {
		vote := Precommit[H, A]{
			Vote: Vote[H, A]{
				Height: t.state.height,
				Round:  t.state.round,
				ID:     nil,
				Sender: t.nodeAddr,
			},
		}
		t.messages.addPrecommit(vote)
		fmt.Println("About to send precommit nil")
		t.broadcasters.PrecommitBroadcaster.Broadcast(vote)
		t.state.step = precommit
		fmt.Println("Set the step to precommit")
	}
}

func (t *Tendermint[_, _, _]) OnTimeoutPrecommit(h, r uint) {
	if t.state.height == h && t.state.round == r {
		t.startRound(r + 1)
	}
}

func (t *Tendermint[V, H, A]) line55(futureR uint) {
	vals := make(map[A]struct{})
	proposals, prevotes, precommits := t.messages.allMessages(t.state.height, futureR)

	// If a validator has sent proposl, prevote and precommit from a future round then it will only be counted once.
	for addr := range proposals {
		vals[addr] = struct{}{}
	}

	for addr := range prevotes {
		vals[addr] = struct{}{}
	}

	for addr := range precommits {
		vals[addr] = struct{}{}
	}

	if t.validatorSetVotingPower(slices.Collect(maps.Keys(vals))) > f(t.validators.TotalVotingPower(t.state.height)) {
		t.startRound(futureR)
	}
}

func (t *Tendermint[V, H, A]) validatorSetVotingPower(vals []A) uint {
	var totalVotingPower uint
	for _, v := range vals {
		totalVotingPower += t.validators.ValidatorVotingPower(v)
	}
	return totalVotingPower
}

// Todo: add separate unit tests to check f and q thresholds.
func f(totalVotingPower uint) uint {
	// note: integer division automatically floors the result as it return the quotient.
	return (totalVotingPower - 1) / 3
}

func q(totalVotingPower uint) uint {
	// Unfortunately there is no ceiling function for integers in go.
	d := totalVotingPower * 2
	q := d / 3
	r := d % 3
	if r > 0 {
		q++
	}
	return q
}
