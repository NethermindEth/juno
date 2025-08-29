package actions

import "github.com/NethermindEth/juno/consensus/types"

type Action[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	isTendermintAction()
}

type BroadcastProposal[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

type BroadcastPrevote[H types.Hash, A types.Addr] types.Prevote[H, A]

type BroadcastPrecommit[H types.Hash, A types.Addr] types.Precommit[H, A]

type ScheduleTimeout types.Timeout

type Commit[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

func (a *BroadcastProposal[V, H, A]) isTendermintAction() {}

func (a *BroadcastPrevote[H, A]) isTendermintAction() {}

func (a *BroadcastPrecommit[H, A]) isTendermintAction() {}

func (a *ScheduleTimeout) isTendermintAction() {}

func (a *Commit[V, H, A]) isTendermintAction() {}
