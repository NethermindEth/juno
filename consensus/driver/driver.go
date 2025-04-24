package driver

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/consensus/tendermint"
)

type timeoutFn func(step tendermint.Step, round tendermint.Round) time.Duration

type Listener[M tendermint.Message[V, H, A], V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr] interface {
	// Listen would return consensus messages to Tendermint which are set by the validator set.
	Listen() <-chan M
}

type Broadcaster[M tendermint.Message[V, H, A], V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr] interface {
	// Broadcast will broadcast the message to the whole validator set. The function should not be blocking.
	Broadcast(M)

	// SendMsg would send a message to a specific validator. This would be required for helping send resquest and
	// response message to help a specifc validator to catch up.
	SendMsg(A, M)
}

type Listeners[V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr] struct {
	ProposalListener  Listener[tendermint.Proposal[V, H, A], V, H, A]
	PrevoteListener   Listener[tendermint.Prevote[H, A], V, H, A]
	PrecommitListener Listener[tendermint.Precommit[H, A], V, H, A]
}

type Broadcasters[V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr] struct {
	ProposalBroadcaster  Broadcaster[tendermint.Proposal[V, H, A], V, H, A]
	PrevoteBroadcaster   Broadcaster[tendermint.Prevote[H, A], V, H, A]
	PrecommitBroadcaster Broadcaster[tendermint.Precommit[H, A], V, H, A]
}

type Driver[V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr] struct {
	stateMachine tendermint.StateMachine[V, H, A]

	getTimeout timeoutFn

	listeners    Listeners[V, H, A]
	broadcasters Broadcasters[V, H, A]

	scheduledTms map[tendermint.Timeout]*time.Timer
	timeoutsCh   chan tendermint.Timeout

	wg   sync.WaitGroup
	quit chan struct{}
}

func New[V tendermint.Hashable[H], H tendermint.Hash, A tendermint.Addr](
	stateMachine tendermint.StateMachine[V, H, A],
	listeners Listeners[V, H, A],
	broadcasters Broadcasters[V, H, A],
	getTimeout timeoutFn,
) *Driver[V, H, A] {
	return &Driver[V, H, A]{
		stateMachine: stateMachine,
		getTimeout:   getTimeout,
		listeners:    listeners,
		broadcasters: broadcasters,
		scheduledTms: make(map[tendermint.Timeout]*time.Timer),
		timeoutsCh:   make(chan tendermint.Timeout),
		quit:         make(chan struct{}),
	}
}

func (d *Driver[V, H, A]) Start() {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		actions := d.stateMachine.ProcessStart(0)
		d.execute(actions)

		// Todo: check message signature everytime a message is received.
		// For the time being it can be assumed the signature is correct.

		for {
			select {
			case <-d.quit:
				return
			case tm := <-d.timeoutsCh:
				// Handling of timeouts is priorities over messages
				delete(d.scheduledTms, tm)
				actions = d.stateMachine.ProcessTimeout(tm)
			case p := <-d.listeners.ProposalListener.Listen():
				actions = d.stateMachine.ProcessProposal(p)
			case p := <-d.listeners.PrevoteListener.Listen():
				actions = d.stateMachine.ProcessPrevote(p)
			case p := <-d.listeners.PrecommitListener.Listen():
				actions = d.stateMachine.ProcessPrecommit(p)
			}
			d.execute(actions)
		}
	}()
}

func (d *Driver[V, H, A]) Stop() {
	close(d.quit)
	d.wg.Wait()
	for _, tm := range d.scheduledTms {
		tm.Stop()
	}
}

func (d *Driver[V, H, A]) execute(actions []tendermint.Action[V, H, A]) {
	for _, action := range actions {
		switch action := action.(type) {
		case *tendermint.BroadcastProposal[V, H, A]:
			d.broadcasters.ProposalBroadcaster.Broadcast(tendermint.Proposal[V, H, A](*action))
		case *tendermint.BroadcastPrevote[H, A]:
			d.broadcasters.PrevoteBroadcaster.Broadcast(tendermint.Prevote[H, A](*action))
		case *tendermint.BroadcastPrecommit[H, A]:
			d.broadcasters.PrecommitBroadcaster.Broadcast(tendermint.Precommit[H, A](*action))
		case *tendermint.ScheduleTimeout:
			d.scheduledTms[tendermint.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-d.quit:
				case d.timeoutsCh <- tendermint.Timeout(*action):
				}
			})
		}
	}
}
