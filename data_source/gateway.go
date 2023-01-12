package dataSource

import (
	"errors"

	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type Gateway struct {
	client *clients.GatewayClient
}

func NewGateway(n utils.Network) *Gateway {
	return &Gateway{
		client: clients.NewGatewayClient(n.URL()),
	}
}

// GetBlockByNumber gets the block for a given block number from the feeder gateway,
// then adapts it to the core.Block type.
func (g *Gateway) GetBlockByNumber(blockNumber uint64) (*core.Block, error) {
	return nil, errors.New("not implemented")
}

// GetTransaction gets the transaction for a given transaction hash from the feeder gateway,
// then adapts it to the appropriate core.Transaction types.
func (g *Gateway) GetTransaction(transactionHash *felt.Felt) (*core.Transaction, error) {
	return nil, errors.New("not implemented")
}

// GetClass gets the class for a given class hash from the feeder gateway,
// then adapts it to the core.Class type.
func (g *Gateway) GetClass(classHash *felt.Felt) (*core.Class, error) {
	return nil, errors.New("not implemented")
}

// GetStateUpdate gets the state update for a given block number from the feeder gateway,
// then adapts it to the core.StateUpdate type.
func (g *Gateway) GetStateUpdate(blockNumber uint64) (*core.StateUpdate, error) {
	response, err := g.client.GetStateUpdate(blockNumber)
	if err != nil {
		return nil, err
	}

	return adaptStateUpdate(response)
}

func adaptStateUpdate(response *clients.StateUpdate) (*core.StateUpdate, error) {
	stateDiff := new(core.StateDiff)
	stateDiff.DeclaredContracts = response.StateDiff.DeclaredContracts
	for _, deployedContract := range response.StateDiff.DeployedContracts {
		stateDiff.DeployedContracts = append(stateDiff.DeployedContracts, core.DeployedContract{
			Address:   deployedContract.Address,
			ClassHash: deployedContract.ClassHash,
		})
	}

	stateDiff.Nonces = make(map[felt.Felt]*felt.Felt)
	for addrStr, nonce := range response.StateDiff.Nonces {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return nil, err
		}
		stateDiff.Nonces[*addr] = nonce
	}

	stateDiff.StorageDiffs = make(map[felt.Felt][]core.StorageDiff)
	for addrStr, diffs := range response.StateDiff.StorageDiffs {
		addr, err := new(felt.Felt).SetString(addrStr)
		if err != nil {
			return nil, err
		}

		stateDiff.StorageDiffs[*addr] = []core.StorageDiff{}
		for _, diff := range diffs {
			stateDiff.StorageDiffs[*addr] = append(stateDiff.StorageDiffs[*addr], core.StorageDiff{
				Key:   diff.Key,
				Value: diff.Value,
			})
		}
	}

	return &core.StateUpdate{
		BlockHash: response.BlockHash,
		NewRoot:   response.NewRoot,
		OldRoot:   response.OldRoot,
		StateDiff: stateDiff,
	}, nil
}
