package rpcv7

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/NethermindEth/juno/core/felt"
)

const subscribeEventsChunkSize = 1024

// The function signature of SubscribeTransactionStatus cannot be changed since the jsonrpc package maps the number
// of argument in the function to the parameters in the starknet spec, therefore, the following variables are not passed
// as arguments, and they can be modified in the test to make them run faster.
var (
	subscribeTxStatusTimeout        = 5 * time.Minute
	subscribeTxStatusTickerDuration = 5 * time.Second
)

var (
	_ BlockIdentifier = (*SubscriptionBlockID)(nil)
	_ BlockIdentifier = (*BlockID)(nil)
)

type SubscriptionResponse struct {
	Version string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type BlockIdentifier interface {
	IsLatest() bool
	IsPending() bool
	GetHash() *felt.Felt
	GetNumber() uint64
	UnmarshalJSON(data []byte) error
}

// As per the spec, this is the same as BlockID, but without `pending`
type SubscriptionBlockID struct {
	Latest bool
	Hash   *felt.Felt
	Number uint64
}

func (b *SubscriptionBlockID) IsLatest() bool {
	return b.Latest
}

func (b *SubscriptionBlockID) IsPending() bool {
	return false // Subscription blocks can't be pending
}

func (b *SubscriptionBlockID) GetHash() *felt.Felt {
	return b.Hash
}

func (b *SubscriptionBlockID) GetNumber() uint64 {
	return b.Number
}

func (b *SubscriptionBlockID) UnmarshalJSON(data []byte) error {
	if string(data) == `"latest"` {
		b.Latest = true
	} else {
		jsonObject := make(map[string]json.RawMessage)
		if err := json.Unmarshal(data, &jsonObject); err != nil {
			return err
		}
		hash, ok := jsonObject["block_hash"]
		if ok {
			b.Hash = new(felt.Felt)
			return json.Unmarshal(hash, b.Hash)
		}

		number, ok := jsonObject["block_number"]
		if ok {
			return json.Unmarshal(number, &b.Number)
		}

		return errors.New("cannot unmarshal block id")
	}
	return nil
}
