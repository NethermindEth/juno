package core2sn

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknet"
)

func AdaptEntryPoint(ep core.EntryPoint) starknet.EntryPoint {
	return starknet.EntryPoint{
		Selector: ep.Selector,
		Offset:   ep.Offset,
	}
}

func AdaptSierraEntryPoint(ep core.SierraEntryPoint) starknet.SierraEntryPoint {
	return starknet.SierraEntryPoint{
		Index:    ep.Index,
		Selector: ep.Selector,
	}
}
