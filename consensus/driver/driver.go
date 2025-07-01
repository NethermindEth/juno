package driver

import (
	"sync"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	p2pService "github.com/NethermindEth/juno/p2p"
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

	p2pSync p2pService.Service

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
				actions = d.stateMachine.ProcessProposal(p) // We get the p2p block, we feed it in here, eventually call line 49
			case p := <-d.listeners.PrevoteListener.Listen():
				actions = d.stateMachine.ProcessPrevote(p)
			case p := <-d.listeners.PrecommitListener.Listen(): // We get the p2p block, we feed it in here, for signatures
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

func (d *Driver[V, H, A]) execute(actions []types.Action[V, H, A]) {
	for _, action := range actions {
		switch action := action.(type) {
		// call catchup
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
		}
	}
}

// 1. listen to votes and proposals
// 2. store those from future height
// 3. Check if we hit f+1 at a future height in processLoop
// 4. If so, then return a new action (to be defined)
// 5. Update execute() to call catchup
// 6. catchup syncs from curheight to futureHeight, and resets the state etc

// Requires Driver to inherit a p2p service.
// Requires the p2p sync serive to export ProcessBlock

// Need to write set of precommits for the committed block. Will be a dependency. We currently don't do this.
//

// This is blocking. We may want to unblock the Driver to listening to other msgs.
func (d *Driver[V, H, A]) catchup(curHeight, futureHeight types.Height) {
	for i := curHeight; i <= futureHeight; i++ {
		d.p2pSync.CatchUp(curHeight, futureHeight)
	}
	// Dlt WAL
	// Update/reset the state machine

	// Eg we sync form block 5 to 10. In the meantime the network is at block 15. We are stuck in a loop.
	// sync time of a block vs consensus time for a height commit

	// consider sync time > consensus time
	// Distadvantages
	// 1. We block processing future messages (height vs optimsations vs falling behind)
	// 2. If we block future msgs then we may fall behind more
	// 3. If we don't block future messages then we may get race conditions.
	// 4. We need to reset the state machine (doCommitValue)

	// eg we keep syncing in 2 block sizes, but we are 100 blocks behind
	// SN block times goes to 2s. I

	// Consider:
}

// Alternative approach
// listen to msgs
// perofrm threashold check
// request blocks from p2p catch up
// push the bock to the proposal listener in Driver
// Q: how we'll commit?
// Q: are we duplicating messages?

// This is blocking. We may want to unblock the Driver to listening to other msgs.
func (d *Driver[V, H, A]) catchup2(curHeight, futureHeight types.Height) {
	for i := curHeight; i <= futureHeight; i++ {
		d.p2pSync.CatchUp(curHeight, futureHeight)
	}
	// listen to msgs on listeners
	// trigger on messages threshold
	// feed into the proposal channel for current height
	//

	// Consider the race condition of the state machine running.
	// Actually, how would this work, because if we already called proposalBlock....
	// I think we need to make sure the cachedProposal is actually updated with the new signatures etc.
}
