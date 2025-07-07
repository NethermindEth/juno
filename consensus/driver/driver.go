package driver

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/utils"
)

type timeoutFn func(step types.Step, round types.Round) time.Duration

type Blockchain[V types.Hashable[H], H types.Hash] interface {
	// Commit is called by Tendermint when a block has been decided on and can be committed to the DB.
	Commit(types.Height, V)
}

type Driver[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log          utils.Logger
	db           db.TendermintDB[V, H, A]
	stateMachine tendermint.StateMachine[V, H, A]
	blockchain   Blockchain[V, H]

	getTimeout timeoutFn

	listeners    p2p.Listeners[V, H, A]
	broadcasters p2p.Broadcasters[V, H, A]

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout

	wg   sync.WaitGroup
	quit chan struct{}
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	db db.TendermintDB[V, H, A],
	stateMachine tendermint.StateMachine[V, H, A],
	blockchain Blockchain[V, H],
	listeners p2p.Listeners[V, H, A],
	broadcasters p2p.Broadcasters[V, H, A],
	getTimeout timeoutFn,
) *Driver[V, H, A] {
	return &Driver[V, H, A]{
		log:          log,
		db:           db,
		stateMachine: stateMachine,
		blockchain:   blockchain,
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
	d.stateMachine.ReplayWAL()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for {
			actions := d.stateMachine.ProcessStart(0)
			isCommitted := d.execute(actions)

			// Todo: check message signature everytime a message is received.
			// For the time being it can be assumed the signature is correct.

			for !isCommitted {
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

				isCommitted = d.execute(actions)
			}
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

// This function executes the actions returned by the stateMachine.
// It returns true if a commit action was executed. This is to notify the caller to start a new height with round 0.
func (d *Driver[V, H, A]) execute(actions []types.Action[V, H, A]) (isCommitted bool) {
	for _, action := range actions {
		switch action := action.(type) {
		case *types.BroadcastProposal[V, H, A]:
			d.broadcasters.ProposalBroadcaster.Broadcast(types.Proposal[V, H, A](*action))
		case *types.BroadcastPrevote[H, A]:
			d.broadcasters.PrevoteBroadcaster.Broadcast(types.Prevote[H, A](*action))
		case *types.BroadcastPrecommit[H, A]:
			d.broadcasters.PrecommitBroadcaster.Broadcast(types.Precommit[H, A](*action))
		case *types.ScheduleTimeout:
			d.scheduledTms[types.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-d.quit:
				case d.timeoutsCh <- types.Timeout(*action):
				}
			})
		case *types.Commit[V, H, A]:
			if err := d.db.Flush(); err != nil {
				d.log.Fatalf("failed to flush WAL during commit", "height", action.Height, "round", action.Round, "err", err)
			}

			d.blockchain.Commit(action.Height, *action.Value)

			if err := d.db.DeleteWALEntries(action.Height); err != nil {
				d.log.Errorw("failed to delete WAL messages during commit", "height", action.Height, "round", action.Round, "err", err)
			}

			return true
		}
	}
	return false
}
