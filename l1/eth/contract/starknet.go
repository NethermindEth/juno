// Package contract hand-decodes the Starknet core L1 contract events
// juno consumes. It exists to remove the abigen + go-ethereum dependency
// from the L1 sync path. Only the event(s) juno actually subscribes to
// are implemented; calls into contract methods are not supported because
// juno doesn't make any.
package contract

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/l1/eth"
	"github.com/NethermindEth/juno/l1/eth/client"
)

// keccak256("LogStateUpdate(uint256,int256,uint256)"), verified against the deployed
// Starknet core contract and the abigen output this package replaces.
var LogStateUpdateSigHash = eth.HashFromString(
	"0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c",
)

// Three 32-byte words: (uint256 globalRoot, int256 blockNumber, uint256 blockHash).
// No indexed args, so all three are in data, not topics.
const logStateUpdateDataLen = 3 * 32

// event LogStateUpdate(uint256 globalRoot, int256 blockNumber, uint256 blockHash).
// blockNumber is int256 but always non-negative and uint64-sized, so we decode it as uint64.
// globalRoot and blockHash are field elements, so they land in felt.Felt directly.
type LogStateUpdate struct {
	GlobalRoot  felt.Felt
	BlockNumber uint64
	BlockHash   felt.Felt

	// Raw log envelope, preserving the emitting L1 block number and the reorg Removed flag.
	Raw eth.Log
}

// ErrWrongTopic is returned when Decode is given a log whose first
// topic is not LogStateUpdateSigHash.
var ErrWrongTopic = errors.New("log topic is not LogStateUpdate")

// Callers are expected to prefilter by topic and contract address;
// sig hash is re-checked defensively.
func Decode(log *eth.Log) (*LogStateUpdate, error) {
	// No indexed args, so a well-formed log carries exactly one topic (the sig hash).
	if len(log.Topics) != 1 || log.Topics[0] != LogStateUpdateSigHash {
		return nil, ErrWrongTopic
	}
	if len(log.Data) != logStateUpdateDataLen {
		return nil, fmt.Errorf("bad LogStateUpdate data length: got %d, want %d",
			len(log.Data), logStateUpdateDataLen)
	}
	ev := &LogStateUpdate{
		// Low 8 bytes of the int256 slot; upper 24 dropped (always fits uint64).
		BlockNumber: binary.BigEndian.Uint64(log.Data[56:64]),
		Raw:         *log,
	}
	ev.GlobalRoot.SetBytes(log.Data[0:32])
	ev.BlockHash.SetBytes(log.Data[64:96])
	return ev, nil
}

// Interface so tests can swap in a fake without dialling a real endpoint.
type LogClient interface {
	FilterLogs(ctx context.Context, q client.FilterQuery) ([]eth.Log, error)
}

// LogStateUpdateFilter is the address+topic filter selecting LogStateUpdate
// events emitted by contract. Callers add a block range for eth_getLogs, or
// leave it unset for a live eth_subscribe subscription.
func LogStateUpdateFilter(contract eth.Address) client.FilterQuery {
	return client.FilterQuery{
		Addresses: []eth.Address{contract},
		Topics:    [][]eth.Hash{{LogStateUpdateSigHash}},
	}
}

// FilterLogStateUpdate returns every LogStateUpdate emitted by contract
// in the inclusive L1 block range [from, to].
func FilterLogStateUpdate(
	ctx context.Context,
	c LogClient,
	contract eth.Address,
	from uint64,
	to uint64,
) ([]*LogStateUpdate, error) {
	q := LogStateUpdateFilter(contract)
	q.FromBlock = &from
	q.ToBlock = &to
	logs, err := c.FilterLogs(ctx, q)
	if err != nil {
		// The client annotates ("filtering logs: …") and the caller adds the
		// block range; re-stating "filtering LogStateUpdate" here just nests.
		return nil, err
	}
	out := make([]*LogStateUpdate, len(logs))
	for i := range logs {
		ev, derr := Decode(&logs[i])
		if derr != nil {
			return nil, fmt.Errorf("decoding LogStateUpdate: %w", derr)
		}
		out[i] = ev
	}
	return out, nil
}
