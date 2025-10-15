package driver

import (
	"context"
	"fmt"
	gosync "sync"
	"time"

	"github.com/NethermindEth/juno/consensus/db"
	"github.com/NethermindEth/juno/consensus/p2p"
	consensusSync "github.com/NethermindEth/juno/consensus/sync"
	"github.com/NethermindEth/juno/consensus/tendermint"
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/actions"
	"github.com/NethermindEth/juno/p2p/sync"
	"github.com/NethermindEth/juno/utils"
)

type TimeoutFn func(step types.Step, round types.Round) time.Duration

type Driver[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	log              utils.Logger
	db               db.TendermintDB[V, H, A]
	stateMachine     tendermint.StateMachine[V, H, A]
	commitListener   CommitListener[V, H]
	broadcasters     p2p.Broadcasters[V, H, A]
	listeners        p2p.Listeners[V, H, A]
	blockFetcher     *sync.BlockFetcher
	messageExtractor *consensusSync.MessageExtractor[V, H, A]
	getTimeout       TimeoutFn

	scheduledTms map[types.Timeout]*time.Timer
	timeoutsCh   chan types.Timeout
	syncListener chan sync.BlockBody
	lastQuorum   types.Height
	wg           gosync.WaitGroup
}

func New[V types.Hashable[H], H types.Hash, A types.Addr](
	log utils.Logger,
	db db.TendermintDB[V, H, A],
	stateMachine tendermint.StateMachine[V, H, A],
	commitListener CommitListener[V, H],
	broadcasters p2p.Broadcasters[V, H, A],
	listeners p2p.Listeners[V, H, A],
	blockFetcher *sync.BlockFetcher,
	messageExtractor *consensusSync.MessageExtractor[V, H, A],
	getTimeout TimeoutFn,
) Driver[V, H, A] {
	return Driver[V, H, A]{
		log:              log,
		db:               db,
		stateMachine:     stateMachine,
		commitListener:   commitListener,
		broadcasters:     broadcasters,
		listeners:        listeners,
		blockFetcher:     blockFetcher,
		messageExtractor: messageExtractor,
		getTimeout:       getTimeout,
		scheduledTms:     make(map[types.Timeout]*time.Timer),
		timeoutsCh:       make(chan types.Timeout),
		syncListener:     make(chan sync.BlockBody, 1),
		wg:               gosync.WaitGroup{},
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

	if err := d.replay(ctx); err != nil {
		return err
	}

	return d.listen(ctx)
}

func (d *Driver[V, H, A]) replay(ctx context.Context) error {
	for walEntry, err := range d.db.LoadAllEntries() {
		if err != nil {
			return fmt.Errorf("failed to load WAL entries: %w", err)
		}

		if _, err := d.execute(ctx, true, d.stateMachine.ProcessWAL(walEntry)); err != nil {
			return err
		}
	}

	return nil
}

//nolint:gocyclo // This is having higher complexity due to adding ok check for unmanaged channels.
func (d *Driver[V, H, A]) listen(ctx context.Context) error {
	defer d.wg.Wait()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		actions := d.stateMachine.ProcessStart(0)
		isCommitted, err := d.execute(ctx, false, actions)
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
			case p, ok := <-d.syncListener:
				if !ok {
					return nil
				}

				if p.Err != nil {
					d.syncCurrentHeight(ctx)
				} else {
					proposal, precommits := d.messageExtractor.Extract(&p)
					actions = d.stateMachine.ProcessSync(&proposal, precommits)
				}
			}

			isCommitted, err = d.execute(ctx, false, actions)
			if err != nil {
				return err
			}
		}

		d.syncCurrentHeight(ctx)
	}
}

// This function executes the actions returned by the stateMachine.
// It returns true if a commit action was executed. This is to notify the caller to start a new height with round 0.
// Note: `WriteWAL` actions are generated as part of processing the event itself, so there's no
// need to write them to the WAL again here. `isReplaying` is used to disable the writing of WAL.
func (d *Driver[V, H, A]) execute(
	ctx context.Context,
	isReplaying bool,
	resultActions []actions.Action[V, H, A],
) (isCommitted bool, err error) {
	for _, action := range resultActions {
		if !isReplaying && action.RequiresWALFlush() {
			if err := d.db.Flush(); err != nil {
				return false, fmt.Errorf("failed to flush WAL: %w", err)
			}
		}

		switch action := action.(type) {
		case *actions.WriteWAL[V, H, A]:
			if !isReplaying {
				if err := d.db.SetWALEntry(action.Entry); err != nil {
					return false, fmt.Errorf("failed to write WAL: %w", err)
				}
			}

		case *actions.BroadcastProposal[V, H, A]:
			d.broadcasters.ProposalBroadcaster.Broadcast(ctx, (*types.Proposal[V, H, A])(action))

		case *actions.BroadcastPrevote[H, A]:
			d.broadcasters.PrevoteBroadcaster.Broadcast(ctx, (*types.Prevote[H, A])(action))

		case *actions.BroadcastPrecommit[H, A]:
			d.broadcasters.PrecommitBroadcaster.Broadcast(ctx, (*types.Precommit[H, A])(action))

		case *actions.ScheduleTimeout:
			d.scheduleTimeout(ctx, types.Timeout(*action))

		case *actions.Commit[V, H, A]:
			return true, d.commit(ctx, action)

		case *actions.TriggerSync:
			d.triggerSync(ctx, *action)
		}
	}
	return false, nil
}

func (d *Driver[V, H, A]) scheduleTimeout(ctx context.Context, timeout types.Timeout) {
	d.scheduledTms[timeout] = time.AfterFunc(d.getTimeout(timeout.Step, timeout.Round), func() {
		select {
		case <-ctx.Done():
		case d.timeoutsCh <- timeout:
		}
	})
}

func (d *Driver[V, H, A]) commit(ctx context.Context, commit *actions.Commit[V, H, A]) error {
	d.log.Debugw("Committing", "height", commit.Height, "round", commit.Round)
	d.commitListener.OnCommit(ctx, commit.Height, *commit.Value)

	if err := d.db.DeleteWALEntries(commit.Height); err != nil {
		return fmt.Errorf("failed to delete WAL messages during commit: %w", err)
	}

	return nil
}

func (d *Driver[V, H, A]) triggerSync(ctx context.Context, triggerSync actions.TriggerSync) {
	currentlyHasFutureQuorum := d.hasFutureQuorum()
	// TODO: Temporary workaround to only trigger the next height, because the sync process
	// doesn't support triggering multiple heights at once.
	d.lastQuorum = max(d.lastQuorum, triggerSync.End)

	// Only trigger sync if haven't triggered yet.
	if !currentlyHasFutureQuorum {
		d.syncCurrentHeight(ctx)
	}
}

// TODO: Temporary workaround to only trigger the next height, because the sync process currently
// doesn't support triggering multiple heights at once.
func (d *Driver[V, H, A]) syncCurrentHeight(ctx context.Context) {
	height := d.stateMachine.Height()

	if !d.hasFutureQuorum() {
		return
	}

	d.wg.Go(func() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := d.blockFetcher.ProcessBlock(ctx, uint64(height), d.syncListener); err != nil {
			select {
			case <-ctx.Done():
			case d.syncListener <- sync.BlockBody{Err: err}:
			}
		}
	})
}

func (d *Driver[V, H, A]) hasFutureQuorum() bool {
	return d.lastQuorum > d.stateMachine.Height()
}
