package tendermint

import "github.com/NethermindEth/juno/consensus/types"

type BroadcastProposal[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

type BroadcastPrevote[H types.Hash, A types.Addr] types.Prevote[H, A]

type BroadcastPrecommit[H types.Hash, A types.Addr] types.Precommit[H, A]

type ScheduleTimeout types.Timeout

func (a *BroadcastProposal[V, H, A]) IsTendermintAction() {}

func (a *BroadcastPrevote[H, A]) IsTendermintAction() {}

func (a *BroadcastPrecommit[H, A]) IsTendermintAction() {}

func (a *ScheduleTimeout) IsTendermintAction() {}
