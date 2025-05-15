package types

type Action[V Hashable[H], H Hash, A Addr] interface {
	IsTendermintAction()
}

type BroadcastProposal[V Hashable[H], H Hash, A Addr] Proposal[V, H, A]

type BroadcastPrevote[H Hash, A Addr] Prevote[H, A]

type BroadcastPrecommit[H Hash, A Addr] Precommit[H, A]

type ScheduleTimeout Timeout

func (a *BroadcastProposal[V, H, A]) IsTendermintAction() {}

func (a *BroadcastPrevote[H, A]) IsTendermintAction() {}

func (a *BroadcastPrecommit[H, A]) IsTendermintAction() {}

func (a *ScheduleTimeout) IsTendermintAction() {}
