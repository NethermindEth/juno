package types

type Action[V Hashable] interface {
	isTendermintAction()
}

type BroadcastProposal[V Hashable] Proposal[V]

type BroadcastPrevote Prevote

type BroadcastPrecommit Precommit

type ScheduleTimeout Timeout

func (a *BroadcastProposal[V]) isTendermintAction() {}

func (a *BroadcastPrevote) isTendermintAction() {}

func (a *BroadcastPrecommit) isTendermintAction() {}

func (a *ScheduleTimeout) isTendermintAction() {}
