package client

import (
	"encoding/json"
	"strconv"

	"github.com/NethermindEth/juno/l1/eth"
)

// FilterQuery selects logs by inclusive block range, contract address, and
// topics. A nil FromBlock/ToBlock is omitted from the wire: geth treats an
// explicit toBlock as a bounded historical filter, which would break live
// eth_subscribe subscriptions.
type FilterQuery struct {
	FromBlock *uint64
	ToBlock   *uint64
	Addresses []eth.Address
	// Topics is position-major: Topics[i] is the allowed-set at topic
	// position i (OR'd together); empty means "any value at that position".
	Topics [][]eth.Hash
}

type filterQueryWire struct {
	FromBlock string        `json:"fromBlock,omitempty"`
	ToBlock   string        `json:"toBlock,omitempty"`
	Address   []eth.Address `json:"address,omitempty"`
	Topics    []any         `json:"topics,omitempty"`
}

func quantityHex(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}

func (q FilterQuery) MarshalJSON() ([]byte, error) {
	wire := filterQueryWire{
		Address: q.Addresses,
	}
	if q.FromBlock != nil {
		wire.FromBlock = quantityHex(*q.FromBlock)
	}
	if q.ToBlock != nil {
		wire.ToBlock = quantityHex(*q.ToBlock)
	}
	if len(q.Topics) > 0 {
		wire.Topics = make([]any, len(q.Topics))
		for i, ts := range q.Topics {
			switch len(ts) {
			case 0:
				wire.Topics[i] = nil
			case 1:
				wire.Topics[i] = ts[0]
			default:
				wire.Topics[i] = ts
			}
		}
	}
	return json.Marshal(wire)
}
