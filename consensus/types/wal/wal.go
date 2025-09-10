package wal

import (
	"github.com/NethermindEth/juno/consensus/types"
)

type Entry[V types.Hashable[H], H types.Hash, A types.Addr] interface {
	GetHeight() types.Height
}

type Start types.Height

func (h *Start) GetHeight() types.Height {
	return types.Height(*h)
}

type Proposal[V types.Hashable[H], H types.Hash, A types.Addr] types.Proposal[V, H, A]

func (p *Proposal[V, H, A]) GetHeight() types.Height {
	return p.Height
}

type Prevote[H types.Hash, A types.Addr] types.Prevote[H, A]

func (p *Prevote[H, A]) GetHeight() types.Height {
	return p.Height
}

type Precommit[H types.Hash, A types.Addr] types.Precommit[H, A]

func (p *Precommit[H, A]) GetHeight() types.Height {
	return p.Height
}

type Timeout types.Timeout

func (p *Timeout) GetHeight() types.Height {
	return p.Height
}
