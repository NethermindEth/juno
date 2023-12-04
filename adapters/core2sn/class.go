package core2sn

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/starknet"
)

func AdaptSierraEntryPoint(ep core.SierraEntryPoint) starknet.SierraEntryPoint {
	return starknet.SierraEntryPoint{
		Index:    ep.Index,
		Selector: ep.Selector,
	}
}
