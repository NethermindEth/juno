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

type Driver[V types.Hashable] struct {
	db db.KeyValueStore

	stateMachine tendermint.StateMachine[V]

	getTimeout timeoutFn

	listeners    p2p.Listeners[V]
	broadcasters p2p.Broadcasters[V]

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout

	wg   sync.WaitGroup
	quit chan struct{}
}

func New[V types.Hashable](
	db db.KeyValueStore,
	stateMachine tendermint.StateMachine[V],
	listeners p2p.Listeners[V],
	broadcasters p2p.Broadcasters[V],
	getTimeout timeoutFn,
) *Driver[V] {
	return &Driver[V]{
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
func (d *Driver[V]) Start() {
	d.stateMachine.ReplayWAL()

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

func (d *Driver[V]) Stop() {
	close(d.quit)
	d.wg.Wait()
	for _, tm := range d.scheduledTms {
		tm.Stop()
	}
}

func (d *Driver[V]) execute(actions []types.Action[V]) {
	for _, action := range actions {
		switch action := action.(type) {
		case *types.BroadcastProposal[V]:
			d.broadcasters.ProposalBroadcaster.Broadcast(types.Proposal[V](*action))
		case *types.BroadcastPrevote:
			d.broadcasters.PrevoteBroadcaster.Broadcast(types.Prevote(*action))
		case *types.BroadcastPrecommit:
			d.broadcasters.PrecommitBroadcaster.Broadcast(types.Precommit(*action))
		case *types.ScheduleTimeout:
			d.scheduledTms[types.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-d.quit:
				case d.timeoutsCh <- types.Timeout(*action):
				}
			})
		}
	}
}
