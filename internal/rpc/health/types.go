package health

import "github.com/NethermindEth/juno/pkg/types"

type Status struct {
	Status        string            `json:"status"`
	Message       string            `json:"message"`
	SyncingStatus *types.SyncStatus `json:"syncingStatus"`
}
