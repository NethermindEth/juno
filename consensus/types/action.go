package types

type Action[V Hashable[H], H Hash, A Addr] interface {
	isTendermintAction()
}

type BroadcastProposal[V Hashable[H], H Hash, A Addr] Proposal[V, H, A]

type BroadcastPrevote[H Hash, A Addr] Prevote[H, A]

type BroadcastPrecommit[H Hash, A Addr] Precommit[H, A]

type ScheduleTimeout Timeout

type Commit[V Hashable[H], H Hash, A Addr] Proposal[V, H, A]

func (a *BroadcastProposal[V, H, A]) isTendermintAction() {}

func (a *BroadcastPrevote[H, A]) isTendermintAction() {}

func (a *BroadcastPrecommit[H, A]) isTendermintAction() {}

func (a *ScheduleTimeout) isTendermintAction() {}

func (a *Commit[V, H, A]) isTendermintAction() {}
