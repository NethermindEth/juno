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

// LogStateUpdateSigHash is keccak256("LogStateUpdate(uint256,int256,uint256)").
var LogStateUpdateSigHash = eth.HashFromString(
	"0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c",
)

// logStateUpdateDataLen is three 32-byte words: no indexed args, so all three
// land in data, not topics.
const logStateUpdateDataLen = 3 * 32

type LogStateUpdate struct {
	GlobalRoot  felt.Felt
	BlockNumber uint64
	BlockHash   felt.Felt

	// Raw log envelope, preserving the emitting L1 block number and the reorg Removed flag.
	Raw eth.Log
}

var ErrWrongTopic = errors.New("log topic is not LogStateUpdate")

// Decode re-checks the sig hash defensively; callers are expected to
// prefilter by topic and contract address.
func Decode(log *eth.Log) (*LogStateUpdate, error) {
	if len(log.Topics) != 1 || log.Topics[0] != LogStateUpdateSigHash {
		return nil, ErrWrongTopic
	}
	if len(log.Data) != logStateUpdateDataLen {
		return nil, fmt.Errorf("bad LogStateUpdate data length: got %d, want %d",
			len(log.Data), logStateUpdateDataLen)
	}
	// Nonzero upper bytes mean a malformed log.
	for _, b := range log.Data[32:56] {
		if b != 0 {
			return nil, fmt.Errorf("blockNumber exceeds uint64: upper bytes not zero: 0x%x",
				log.Data[32:64])
		}
	}
	ev := &LogStateUpdate{
		BlockNumber: binary.BigEndian.Uint64(log.Data[56:64]),
		Raw:         *log,
	}
	// SetBytesCanonical rejects values >= the STARK prime; plain SetBytes
	// would silently alias them to a different felt.
	if err := ev.GlobalRoot.SetBytesCanonical(log.Data[0:32]); err != nil {
		return nil, fmt.Errorf("globalRoot: %w", err)
	}
	if err := ev.BlockHash.SetBytesCanonical(log.Data[64:96]); err != nil {
		return nil, fmt.Errorf("blockHash: %w", err)
	}
	return ev, nil
}

// LogClient lets tests swap in a fake without dialling a real endpoint.
type LogClient interface {
	FilterLogs(ctx context.Context, q client.FilterQuery) ([]eth.Log, error)
}

// LogStateUpdateFilter omits the block range: callers add one for eth_getLogs,
// or leave it unset for a live eth_subscribe subscription.
func LogStateUpdateFilter(contract eth.Address) client.FilterQuery {
	return client.FilterQuery{
		Addresses: []eth.Address{contract},
		Topics:    [][]eth.Hash{{LogStateUpdateSigHash}},
	}
}

// FilterLogStateUpdate treats [from, to] as inclusive on both ends.
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
