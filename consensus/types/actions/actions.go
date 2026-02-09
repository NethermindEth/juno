package actions

import (
	"github.com/NethermindEth/juno/consensus/types"
	"github.com/NethermindEth/juno/consensus/types/wal"
)

type Action[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	RequiresWALFlush() bool
	isTendermintAction()
}

type WriteWAL[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	Entry wal.Entry[V, H, A]
}

type BroadcastProposal[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

type BroadcastPrevote[H types.Hash, A types.Addr] types.Prevote[H, A]

type BroadcastPrecommit[H types.Hash, A types.Addr] types.Precommit[H, A]

type ScheduleTimeout types.Timeout

type Commit[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

type TriggerSync struct {
	Start, End types.Height
}

func (a *WriteWAL[V, H, A]) RequiresWALFlush() bool          { return false }
func (a *BroadcastProposal[V, H, A]) RequiresWALFlush() bool { return true }
func (a *BroadcastPrevote[H, A]) RequiresWALFlush() bool     { return true }
func (a *BroadcastPrecommit[H, A]) RequiresWALFlush() bool   { return true }
func (a *ScheduleTimeout) RequiresWALFlush() bool            { return false }
func (a *Commit[V, H, A]) RequiresWALFlush() bool            { return true }
func (a *TriggerSync) RequiresWALFlush() bool                { return false }

func (a *WriteWAL[V, H, A]) isTendermintAction()          {}
func (a *BroadcastProposal[V, H, A]) isTendermintAction() {}
func (a *BroadcastPrevote[H, A]) isTendermintAction()     {}
func (a *BroadcastPrecommit[H, A]) isTendermintAction()   {}
func (a *ScheduleTimeout) isTendermintAction()            {}
func (a *Commit[V, H, A]) isTendermintAction()            {}
func (a *TriggerSync) isTendermintAction()                {}
