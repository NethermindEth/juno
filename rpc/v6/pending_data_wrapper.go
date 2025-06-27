package rpcv6

import (
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
)

func (h *Handler) PendingData() (*core.PendingData, error) {
	pending, err := h.syncReader.PendingData()
	if err != nil {
		return nil, err
	}
	// If IsPending == true, network is still polling pending block and running on < 0.14.0
	if pending.Variant() == core.PendingBlockVariant {
		return pending, nil
	} else {
		// If IsPending == false, network is still polling pre_confirmed block and running on >= 0.14.0
		// pendingID == latest
		latestB, err := h.bcReader.Head()
		if err != nil {
			return nil, err
		}

		stateUpdate, err := h.bcReader.StateUpdateByNumber(latestB.Number)
		if err != nil {
			return nil, err
		}
		reader, _, err := h.bcReader.StateAtBlockNumber(latestB.Number)
		if err != nil {
			return nil, err
		}

		newClasses := make(map[felt.Felt]core.Class, 0)
		for classHash := range stateUpdate.StateDiff.DeclaredV1Classes {
			declaredClass, err := reader.Class(&classHash)
			if err != nil {
				return nil, err
			}
			newClasses[classHash] = declaredClass.Class
		}

		for _, classHash := range stateUpdate.StateDiff.DeclaredV0Classes {
			declaredClass, err := reader.Class(classHash)
			if err != nil {
				return nil, err
			}
			newClasses[*classHash] = declaredClass.Class
		}

		return core.NewPending(
			latestB,
			stateUpdate,
			newClasses,
		).AsPendingData(), nil
	}
}

func (h *Handler) PendingBlock() *core.Block {
	pending, err := h.PendingData()
	if err != nil {
		return nil
	}
	return pending.GetBlock()
}
