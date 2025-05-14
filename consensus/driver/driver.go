package driver

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/db"
)

type timeoutFn func(step types.Step, round types.Round) time.Duration

type Driver[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	db db.KeyValueStore

	stateMachine tendermint.StateMachine[V, H, A]

	getTimeout timeoutFn

	listeners    p2p.Listeners[V, H, A]
	broadcasters p2p.Broadcasters[V, H, A]

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout

	wg   sync.WaitGroup
	quit chan struct{}
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	db db.KeyValueStore,
	stateMachine tendermint.StateMachine[V, H, A],
	listeners p2p.Listeners[V, H, A],
	broadcasters p2p.Broadcasters[V, H, A],
	getTimeout timeoutFn,
) *Driver[V, H, A] {
	return &Driver[V, H, A]{
		db:           db,
		stateMachine: stateMachine,
		getTimeout:   getTimeout,
		listeners:    listeners,
		broadcasters: broadcasters,
		scheduledTms: make(map[types.Timeout]*time.Timer),
		timeoutsCh:   make(chan types.Timeout),
		quit:         make(chan struct{}),
	}
}

// The Driver is responsible for listening to messages from the network
// and passing them into the stateMachine. The stateMachine processes
// these messages and returns a set of actions to be executed by the Driver.
// The Driver executes these actions (namely broadcasting messages
// and triggering scheduled timeouts).
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
			d.broadcasters.ProposalBroadcaster.Broadcast(types.Proposal[V, H, A](*action))
		case *tendermint.BroadcastPrevote[H, A]:
			d.broadcasters.PrevoteBroadcaster.Broadcast(types.Prevote[H, A](*action))
		case *tendermint.BroadcastPrecommit[H, A]:
			d.broadcasters.PrecommitBroadcaster.Broadcast(types.Precommit[H, A](*action))
		case *tendermint.ScheduleTimeout:
			d.scheduledTms[types.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-d.quit:
				case d.timeoutsCh <- types.Timeout(*action):
				}
			})
		}
	}
}
