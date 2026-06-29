package client

import (
	"encoding/json"
	"strconv"

	"github.com/NethermindEth/juno/l1/eth"
)

// FilterQuery is the request shape for eth_getLogs (and eth_subscribe
// "logs"). Block numbers are inclusive on both ends, matching the wire
// format. FromBlock/ToBlock are optional: nil means "omit from the wire"
// — for eth_subscribe this yields a live-logs subscription with no
// historical range; for eth_getLogs it lets the server apply its
// defaults. Addresses and Topics are both optional.
type FilterQuery struct {
	FromBlock *uint64
	ToBlock   *uint64
	Addresses []eth.Address
	// Topics is a position-major filter: Topics[i] is the allowed-set at
	// topic position i (OR'd together). An empty Topics[i] means "any
	// value at that position". Trailing unconstrained positions may be
	// omitted.
	Topics [][]eth.Hash
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

// filterQueryWire is the on-the-wire shape; FilterQuery's MarshalJSON
// reshapes the user-facing fields into this. fromBlock/toBlock use
// omitempty so an unset FilterQuery serialises without them, matching
// the eth_subscribe live-logs semantics on geth.
type filterQueryWire struct {
	FromBlock string        `json:"fromBlock,omitempty"`
	ToBlock   string        `json:"toBlock,omitempty"`
	Address   []eth.Address `json:"address,omitempty"`
	Topics    []any         `json:"topics,omitempty"`
}

// quantityHex encodes n as an Ethereum JSON-RPC "quantity": 0x-prefixed
// minimal hex ("0x0" for zero, no leading zeros otherwise).
func quantityHex(n uint64) string {
	return "0x" + strconv.FormatUint(n, 16)
}
