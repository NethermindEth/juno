package driver

import (
	"context"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/utils"
)

type TimeoutFn func(step types.Step, round types.Round) time.Duration

type Driver[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log            utils.Logger
	db             db.TendermintDB[V, H, A]
	stateMachine   tendermint.StateMachine[V, H, A]
	commitListener CommitListener[V, H]
	broadcasters   p2p.Broadcasters[V, H, A]
	listeners      p2p.Listeners[V, H, A]

	getTimeout TimeoutFn

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	db db.TendermintDB[V, H, A],
	stateMachine tendermint.StateMachine[V, H, A],
	commitListener CommitListener[V, H],
	broadcasters p2p.Broadcasters[V, H, A],
	listeners p2p.Listeners[V, H, A],
	getTimeout TimeoutFn,
) Driver[V, H, A] {
	return Driver[V, H, A]{
		log:            log,
		db:             db,
		stateMachine:   stateMachine,
		commitListener: commitListener,
		broadcasters:   broadcasters,
		listeners:      listeners,
		getTimeout:     getTimeout,
		scheduledTms:   make(map[types.Timeout]*time.Timer),
		timeoutsCh:     make(chan types.Timeout),
	}
}

// The Driver is responsible for listening to messages from the network
// and passing them into the stateMachine. The stateMachine processes
// these messages and returns a set of actions to be executed by the Driver.
// The Driver executes these actions (namely broadcasting messages
// and triggering scheduled timeouts).
func (d *Driver[V, H, A]) Run(ctx context.Context) error {
	defer func() {
		for _, tm := range d.scheduledTms {
			tm.Stop()
		}
	}()

	d.stateMachine.ReplayWAL()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		actions := d.stateMachine.ProcessStart(0)
		isCommitted := d.execute(ctx, actions)

		// Todo: check message signature everytime a message is received.
		// For the time being it can be assumed the signature is correct.
		for !isCommitted {
			select {
			case <-ctx.Done():
				return nil
			case tm := <-d.timeoutsCh:
				// Handling of timeouts is priorities over messages
				delete(d.scheduledTms, tm)
				actions = d.stateMachine.ProcessTimeout(tm)
			case p, ok := <-d.listeners.ProposalListener.Listen():
				if !ok {
					return nil
				}
				actions = d.stateMachine.ProcessProposal(p)
			case p, ok := <-d.listeners.PrevoteListener.Listen():
				if !ok {
					return nil
				}
				actions = d.stateMachine.ProcessPrevote(p)
			case p, ok := <-d.listeners.PrecommitListener.Listen():
				if !ok {
					return nil
				}
				actions = d.stateMachine.ProcessPrecommit(p)
			}

			isCommitted = d.execute(ctx, actions)
		}
	}
}

// This function executes the actions returned by the stateMachine.
// It returns true if a commit action was executed. This is to notify the caller to start a new height with round 0.
func (d *Driver[V, H, A]) execute(
	ctx context.Context,
	executingActions []actions.Action[V, H, A],
) (isCommitted bool) {
	for _, action := range executingActions {
		switch action := action.(type) {
		case *actions.BroadcastProposal[V, H, A]:
			d.broadcasters.ProposalBroadcaster.Broadcast(ctx, (*types.Proposal[V, H, A])(action))
		case *actions.BroadcastPrevote[H, A]:
			d.broadcasters.PrevoteBroadcaster.Broadcast(ctx, (*types.Prevote[H, A])(action))
		case *actions.BroadcastPrecommit[H, A]:
			d.broadcasters.PrecommitBroadcaster.Broadcast(ctx, (*types.Precommit[H, A])(action))
		case *actions.ScheduleTimeout:
			d.scheduledTms[types.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-ctx.Done():
				case d.timeoutsCh <- types.Timeout(*action):
				}
			})
		case *actions.Commit[V, H, A]:
			if err := d.db.Flush(); err != nil {
				d.log.Fatalf("failed to flush WAL during commit", "height", action.Height, "round", action.Round, "err", err)
			}

			d.log.Debug("Committing", utils.SugaredFields("height", action.Height, "round", action.Round)...)
			d.commitListener.OnCommit(ctx, action.Height, *action.Value)

			if err := d.db.DeleteWALEntries(action.Height); err != nil {
				d.log.Error("failed to delete WAL messages during commit", utils.SugaredFields("height", action.Height, "round", action.Round, "err", err)...)
			}

			return true
		}
	}
	return false
}
