package tendermint

import (
	"time"
)

type Action[V Hashable[H], H Hash, A Addr] interface {
	Execute(d *Driver[V, H, A])
}

type BroadcastProposal[V Hashable[H], H Hash, A Addr] Proposal[V, H, A]

type BroadcastPrevote[V Hashable[H], H Hash, A Addr] Prevote[H, A]

type BroadcastPrecommit[V Hashable[H], H Hash, A Addr] Precommit[H, A]

type ScheduleTimeout[V Hashable[H], H Hash, A Addr] timeout

func (a *BroadcastProposal[V, H, A]) Execute(d *Driver[V, H, A]) {
	d.broadcasters.ProposalBroadcaster.Broadcast(Proposal[V, H, A](*a))
}

func (a *BroadcastPrevote[V, H, A]) Execute(d *Driver[V, H, A]) {
	d.broadcasters.PrevoteBroadcaster.Broadcast(Prevote[H, A](*a))
}

func (a *BroadcastPrecommit[V, H, A]) Execute(d *Driver[V, H, A]) {
	d.broadcasters.PrecommitBroadcaster.Broadcast(Precommit[H, A](*a))
}

func (a *ScheduleTimeout[V, H, A]) Execute(d *Driver[V, H, A]) {
	var duration time.Duration
	switch a.s {
	case propose:
		duration = d.timeoutPropose(a.r)
	case prevote:
		duration = d.timeoutPrevote(a.r)
	case precommit:
		duration = d.timeoutPrecommit(a.r)
	default:
		return
	}

	d.scheduledTms[timeout(*a)] = time.AfterFunc(duration, func() {
		select {
		case <-d.quit:
		case d.timeoutsCh <- timeout(*a):
		}
	})
}

func (d *Driver[V, H, A]) execute(actions []Action[V, H, A]) {
	for _, action := range actions {
		action.Execute(d)
	}
}
