package driver

import (
	"context"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/proposer"
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
	p2p          p2p.P2P[V, H, A]
	proposer     proposer.Proposer[V, H]

	getTimeout timeoutFn

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	db db.TendermintDB[V, H, A],
	stateMachine tendermint.StateMachine[V, H, A],
	blockchain Blockchain[V, H],
	p2p p2p.P2P[V, H, A],
	proposer proposer.Proposer[V, H],
	getTimeout timeoutFn,
) Driver[V, H, A] {
	return Driver[V, H, A]{
		log:          log,
		db:           db,
		stateMachine: stateMachine,
		blockchain:   blockchain,
		p2p:          p2p,
		proposer:     proposer,
		getTimeout:   getTimeout,
		scheduledTms: make(map[types.Timeout]*time.Timer),
		timeoutsCh:   make(chan types.Timeout),
	}
}

// The Driver is responsible for listening to messages from the network
// and passing them into the stateMachine. The stateMachine processes
// these messages and returns a set of actions to be executed by the Driver.
// The Driver executes these actions (namely broadcasting messages
// and triggering scheduled timeouts).
func (d *Driver[V, H, A]) Run(ctx context.Context) error {
	d.stateMachine.ReplayWAL()

	listeners := d.p2p.Listeners()
	broadcasters := d.p2p.Broadcasters()

	actions := d.stateMachine.ProcessStart(0)
	d.execute(ctx, broadcasters, actions)

	// Todo: check message signature everytime a message is received.
	// For the time being it can be assumed the signature is correct.
	for {
		select {
		case <-ctx.Done():
			for _, tm := range d.scheduledTms {
				tm.Stop()
			}
			return nil
		case tm := <-d.timeoutsCh:
			// Handling of timeouts is priorities over messages
			delete(d.scheduledTms, tm)
			actions = d.stateMachine.ProcessTimeout(tm)
		case p := <-listeners.ProposalListener.Listen():
			actions = d.stateMachine.ProcessProposal(p)
		case p := <-listeners.PrevoteListener.Listen():
			actions = d.stateMachine.ProcessPrevote(p)
		case p := <-listeners.PrecommitListener.Listen():
			actions = d.stateMachine.ProcessPrecommit(p)
		}
		d.execute(ctx, broadcasters, actions)
	}
}

func (d *Driver[V, H, A]) execute(ctx context.Context, broadcasters p2p.Broadcasters[V, H, A], actions []types.Action[V, H, A]) {
	for _, action := range actions {
		switch action := action.(type) {
		case *types.BroadcastProposal[V, H, A]:
			broadcasters.ProposalBroadcaster.Broadcast(types.Proposal[V, H, A](*action))
		case *types.BroadcastPrevote[H, A]:
			broadcasters.PrevoteBroadcaster.Broadcast(types.Prevote[H, A](*action))
		case *types.BroadcastPrecommit[H, A]:
			broadcasters.PrecommitBroadcaster.Broadcast(types.Precommit[H, A](*action))
		case *types.ScheduleTimeout:
			d.scheduledTms[types.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-ctx.Done():
				case d.timeoutsCh <- types.Timeout(*action):
				}
			})
		case *types.Commit[V, H, A]:
			if err := d.db.Flush(); err != nil {
				d.log.Fatalf("failed to flush WAL during commit", "height", action.Height, "round", action.Round, "err", err)
			}

			d.log.Debugw("Committing", "height", action.Height, "round", action.Round)
			d.blockchain.Commit(action.Height, *action.Value)
			d.proposer.OnCommit(ctx, action.Height, *action.Value)
			d.p2p.OnCommit(ctx, action.Height, *action.Value)

			if err := d.db.DeleteWALEntries(action.Height); err != nil {
				d.log.Errorw("failed to delete WAL messages during commit", "height", action.Height, "round", action.Round, "err", err)
			}
		}
	}
}
