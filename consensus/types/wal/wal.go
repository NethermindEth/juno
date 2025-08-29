package wal

import (
	"github.com/NethermindEth/juno/consensus/types"
)

type Entry[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	GetHeight() types.Height
}

type WALProposal[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

func (p *WALProposal[V, H, A]) GetHeight() types.Height {
	return p.Height
}

type WALPrevote[H types.Hash, A types.Addr] types.Prevote[H, A]

func (p *WALPrevote[H, A]) GetHeight() types.Height {
	return p.Height
}

type WALPrecommit[H types.Hash, A types.Addr] types.Precommit[H, A]

func (p *WALPrecommit[H, A]) GetHeight() types.Height {
	return p.Height
}

type WALTimeout types.Timeout

func (p *WALTimeout) GetHeight() types.Height {
	return p.Height
}
