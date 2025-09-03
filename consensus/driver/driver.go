package driver

import (
	"context"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/p2p"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/consensus/types/wal"
	"github.com/NethermindEth/juno/utils"
)

type TimeoutFn func(step types.Step, round types.Round) time.Duration

type Driver[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log            utils.Logger
	db             db.TendermintDB[V, H, A]
	stateMachine   tendermint.StateMachine[V, H, A]
	commitListener CommitListener[V, H, A]
	p2p            p2p.P2P[V, H, A]

	getTimeout TimeoutFn

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout
	isReplaying  bool
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	db db.TendermintDB[V, H, A],
	stateMachine tendermint.StateMachine[V, H, A],
	commitListener CommitListener[V, H, A],
	p2p p2p.P2P[V, H, A],
	getTimeout TimeoutFn,
) Driver[V, H, A] {
	return Driver[V, H, A]{
		log:            log,
		db:             db,
		stateMachine:   stateMachine,
		commitListener: commitListener,
		p2p:            p2p,
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

	broadcasters := d.p2p.Broadcasters()

	if err := d.replay(ctx, &broadcasters); err != nil {
		return err
	}

	return d.listen(ctx, &broadcasters)
}

func (d *Driver[V, H, A]) replay(ctx context.Context, broadcasters *p2p.Broadcasters[V, H, A]) error {
	d.isReplaying = true
	defer func() {
		d.isReplaying = false
	}()

	var actions []actions.Action[V, H, A]
	for walEntry, err := range d.db.LoadAllEntries() {
		if err != nil {
			return fmt.Errorf("failed to load WAL entries: %w", err)
		}
		switch walEntry := walEntry.(type) {
		case *wal.WALStart:
			actions = d.stateMachine.ProcessStart(0)
		case *wal.WALProposal[V, H, A]:
			actions = d.stateMachine.ProcessProposal((*types.Proposal[V, H, A])(walEntry))
		case *wal.WALPrevote[H, A]:
			actions = d.stateMachine.ProcessPrevote((*types.Prevote[H, A])(walEntry))
		case *wal.WALPrecommit[H, A]:
			actions = d.stateMachine.ProcessPrecommit((*types.Precommit[H, A])(walEntry))
		case *wal.WALTimeout:
			actions = d.stateMachine.ProcessTimeout(types.Timeout(*walEntry))
		}

		if _, err := d.execute(ctx, broadcasters, actions); err != nil {
			return err
		}
	}

	return nil
}

func (d *Driver[V, H, A]) listen(ctx context.Context, broadcasters *p2p.Broadcasters[V, H, A]) error {
	listeners := d.p2p.Listeners()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		actions := d.stateMachine.ProcessStart(0)
		isCommitted, err := d.execute(ctx, broadcasters, actions)
		if err != nil {
			return err
		}

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
			case p, ok := <-listeners.ProposalListener.Listen():
				if !ok {
					return nil
				}
				actions = d.stateMachine.ProcessProposal(p)
			case p, ok := <-listeners.PrevoteListener.Listen():
				if !ok {
					return nil
				}
				actions = d.stateMachine.ProcessPrevote(p)
			case p, ok := <-listeners.PrecommitListener.Listen():
				if !ok {
					return nil
				}
				actions = d.stateMachine.ProcessPrecommit(p)
			}

			isCommitted, err = d.execute(ctx, broadcasters, actions)
			if err != nil {
				return err
			}
		}
	}
}

// This function executes the actions returned by the stateMachine.
// It returns true if a commit action was executed. This is to notify the caller to start a new height with round 0.
func (d *Driver[V, H, A]) execute(
	ctx context.Context,
	broadcasters *p2p.Broadcasters[V, H, A],
	resultActions []actions.Action[V, H, A],
) (isCommitted bool, err error) {
	for _, action := range resultActions {
		switch action := action.(type) {
		case *actions.WriteWAL[V, H, A]:
			if !d.isReplaying {
				if err := d.db.SetWALEntry(action.Entry); err != nil {
					return false, fmt.Errorf("failed to write WAL: %w", err)
				}
			}

		case *actions.BroadcastProposal[V, H, A]:
			if !d.isReplaying {
				if err := d.db.Flush(); err != nil {
					return false, fmt.Errorf("failed to flush WAL: %w", err)
				}
			}
			broadcasters.ProposalBroadcaster.Broadcast(ctx, (*types.Proposal[V, H, A])(action))

		case *actions.BroadcastPrevote[H, A]:
			if !d.isReplaying {
				if err := d.db.Flush(); err != nil {
					return false, fmt.Errorf("failed to flush WAL: %w", err)
				}
			}
			broadcasters.PrevoteBroadcaster.Broadcast(ctx, (*types.Prevote[H, A])(action))

		case *actions.BroadcastPrecommit[H, A]:
			if !d.isReplaying {
				if err := d.db.Flush(); err != nil {
					return false, fmt.Errorf("failed to flush WAL: %w", err)
				}
			}
			broadcasters.PrecommitBroadcaster.Broadcast(ctx, (*types.Precommit[H, A])(action))

		case *actions.ScheduleTimeout:
			d.scheduledTms[types.Timeout(*action)] = time.AfterFunc(d.getTimeout(action.Step, action.Round), func() {
				select {
				case <-ctx.Done():
				case d.timeoutsCh <- types.Timeout(*action):
				}
			})

		case *actions.Commit[V, H, A]:
			if err := d.db.Flush(); err != nil {
				return true, fmt.Errorf("failed to flush WAL: %w", err)
			}

			d.log.Debugw("Committing", "height", action.Height, "round", action.Round)
			d.commitListener.Commit(ctx, action.Height, *action.Value)

			if err := d.db.DeleteWALEntries(action.Height); err != nil {
				return true, fmt.Errorf("failed to delete WAL messages during commit: %w", err)
			}

			return true, nil
		}
	}
	return false, nil
}

func (d *Driver[V, H, A]) writeAndFlush(ctx context.Context, msg wal.Entry[V, H, A]) error {
	if d.isReplaying {
		return nil
	}

	if err := d.db.SetWALEntry(msg); err != nil {
		return fmt.Errorf("failed to write WAL: %w", err)
	}

	if err := d.db.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL: %w", err)
	}

	return nil
}
