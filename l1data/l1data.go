package l1data

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strings"

	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:generate mockgen -destination=../mocks/mock_stateupdate.go -package=mocks github.com/NethermindEth/juno/l1data StateUpdateLogFetcher
type StateUpdateLogFetcher interface {
	StateUpdateLogs(ctx context.Context, from uint64, minStarknetBlockNumber uint64) ([]*LogStateUpdate, error)
}

//go:generate mockgen -destination=../mocks/mock_l1data.go -package=mocks github.com/NethermindEth/juno/l1data L1Data
type L1Data interface {
	StateUpdateLogFetcher
	StateTransitionFact(ctx context.Context, txIndex uint, blockNumber uint64) (*big.Int, error)
	MemoryPagesHashesLog(ctx context.Context, stateTransitionFact *big.Int, from uint64) (*LogMemoryPagesHashes, error)
	MemoryPageFactContinuousLogs(ctx context.Context, mempageHashes [][32]byte, from uint64) ([]*LogMemoryPageFactContinuous, error)
	EncodedStateDiff(ctx context.Context, txHashes []common.Hash) ([]*big.Int, error)
}

//go:generate mockgen -destination=../mocks/mock_ethclient.go -package=mocks github.com/NethermindEth/juno/l1data EthClient
type EthClient interface {
	TransactionByHash(context.Context, common.Hash) (tx *types.Transaction, isPending bool, err error)
	FilterLogs(context.Context, ethereum.FilterQuery) ([]types.Log, error)
}

const blockRangeStep = 1_000

var (
	//go:embed abi/core.json
	coreABIString string
	//go:embed abi/core_new.json
	coreNewABIString string
	//go:embed abi/verifier.json
	verifierABIString string
	//go:embed abi/memory_page_fact_registry.json
	pageFactRegistryABIString string
)

type Backoff interface {
	BackOff(context.Context, error)
}

type nopBackoff struct{}

func (n nopBackoff) BackOff(_ context.Context, _ error) {}

type Client struct {
	ethclient EthClient
	backoff   Backoff

	coreABI             *abi.ABI
	coreNewABI          *abi.ABI
	verifierABI         *abi.ABI
	pageFactRegistryABI *abi.ABI

	logStateUpdateTopics              [][]common.Hash
	logStateTransitionFactTopics      [][]common.Hash
	logMemoryPagesHashesTopics        [][]common.Hash
	logMemoryPageFactContinuousTopics [][]common.Hash
}

var _ L1Data = (*Client)(nil)

func New(ethclient EthClient) (*Client, error) {
	coreABI, err := abi.JSON(strings.NewReader(coreABIString))
	if err != nil {
		return nil, fmt.Errorf("unmarshal core abi: %v", err)
	}
	coreNewABI, err := abi.JSON(strings.NewReader(coreNewABIString))
	if err != nil {
		return nil, fmt.Errorf("unmarshal core new abi: %v", err)
	}
	verifierABI, err := abi.JSON(strings.NewReader(verifierABIString))
	if err != nil {
		return nil, fmt.Errorf("unmarshal verifier abi: %v", err)
	}
	pageFactRegistryABI, err := abi.JSON(strings.NewReader(pageFactRegistryABIString))
	if err != nil {
		return nil, fmt.Errorf("unmarshal page fact registry abi: %v", err)
	}

	return &Client{
		ethclient:           ethclient,
		backoff:             nopBackoff{},
		coreABI:             &coreABI,
		coreNewABI:          &coreNewABI,
		verifierABI:         &verifierABI,
		pageFactRegistryABI: &pageFactRegistryABI,
		logStateUpdateTopics: [][]common.Hash{{
			common.HexToHash("0xe8012213bb931d3efa0a954cfb0d7b75f2a5e2358ba5f7d3edfb0154f6e7a568"),
			common.HexToHash("0xd342ddf7a308dec111745b00315c14b7efb2bdae570a6856e088ed0c65a3576c"),
		}},
		logStateTransitionFactTopics: [][]common.Hash{{
			common.HexToHash("0x9866f8ddfe70bb512b2f2b28b49d4017c43f7ba775f1a20c61c13eea8cdac111"),
		}},
		logMemoryPagesHashesTopics: [][]common.Hash{{
			common.HexToHash("0x73b132cb33951232d83dc0f1f81c2d10f9a2598f057404ed02756716092097bb"),
		}},
		logMemoryPageFactContinuousTopics: [][]common.Hash{{
			common.HexToHash("0xb8b9c39aeba1cfd98c38dfeebe11c2f7e02b334cbe9f05f22b442a5d9c1ea0c5"),
		}},
	}, nil
}

func (c *Client) WithBackoff(b Backoff) *Client {
	c.backoff = b
	return c
}

// TODO there's lots of edge cases with genesis

func (c *Client) StateUpdateLogs(ctx context.Context, from, minStarknetBlockNumber uint64) ([]*LogStateUpdate, error) {
	to := from + blockRangeStep
	stateUpdateLogs := make([]*LogStateUpdate, 0)
	for len(stateUpdateLogs) == 0 {
		logs, err := c.ethclient.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(from),
			ToBlock:   new(big.Int).SetUint64(to),
			Topics:    c.logStateUpdateTopics,
		})
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			to = (from + to) / 2
			c.backoff.BackOff(ctx, err)
			continue
		}
		c.backoff.BackOff(ctx, err)

		for _, log := range logs { //nolint:gocritic
			logStateUpdate := new(LogStateUpdate)
			if err = c.coreNewABI.UnpackIntoInterface(logStateUpdate, logStateUpdate.ABIName(), log.Data); err != nil {
				if err := c.coreABI.UnpackIntoInterface(logStateUpdate, logStateUpdate.ABIName(), log.Data); err != nil {
					return nil, fmt.Errorf("unpack LogStateUpdate into interface: %v", err)
				}
			}
			if logStateUpdate.BlockNumber.Uint64() >= minStarknetBlockNumber {
				logStateUpdate.Raw = log
				stateUpdateLogs = append(stateUpdateLogs, logStateUpdate)
			}
		}

		from = to
		to += blockRangeStep
	}
	return stateUpdateLogs, nil
}

func (c *Client) StateTransitionFact(ctx context.Context, txIndex uint, blockNumber uint64) (*big.Int, error) {
	var logs []types.Log
	for {
		var err error
		logs, err = c.ethclient.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(blockNumber),
			ToBlock:   new(big.Int).SetUint64(blockNumber),
			Topics:    c.logStateTransitionFactTopics,
		})
		// We check the context here so the backoff isn't affected by context errors.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		c.backoff.BackOff(ctx, err)
		if err == nil {
			break
		}
	}
	for _, log := range logs { //nolint:gocritic
		if log.TxIndex == txIndex {
			return new(big.Int).SetBytes(log.Data), nil
		}
	}
	return nil, errors.New("not found")
}

func (c *Client) MemoryPagesHashesLog(ctx context.Context, stateTransitionFact *big.Int, from uint64) (*LogMemoryPagesHashes, error) {
	end := from
	start := end - blockRangeStep
	for {
		logs, err := c.ethclient.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(start),
			ToBlock:   new(big.Int).SetUint64(end),
			Topics:    c.logMemoryPagesHashesTopics,
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// TODO check for genesis
			start += (end - start) / 2
			// TODO log errors before backing off
			c.backoff.BackOff(ctx, err)
			continue
		}
		c.backoff.BackOff(ctx, err)

		for _, log := range logs { //nolint:gocritic
			logMemoryPagesHashes := new(LogMemoryPagesHashes)
			if err := c.verifierABI.UnpackIntoInterface(logMemoryPagesHashes, "LogMemoryPagesHashes", log.Data); err != nil {
				return nil, fmt.Errorf("unpack LogMemoryPagesHashes: %v", err)
			}
			logMemoryPagesHashes.Raw = log
			if new(big.Int).SetBytes(logMemoryPagesHashes.ProgramOutputFact[:]).Cmp(stateTransitionFact) == 0 {
				return logMemoryPagesHashes, nil
			}
		}

		end = start
		start -= blockRangeStep
	}
}

func (c *Client) MemoryPageFactContinuousLogs(ctx context.Context,
	mempageHashes [][32]byte, from uint64,
) ([]*LogMemoryPageFactContinuous, error) {
	requiredPagesHashes := make(map[uint64]*big.Int, 0)
	i := uint64(0)
	for _, pageHash := range mempageHashes {
		requiredPagesHashes[i] = new(big.Int).SetBytes(pageHash[:])
		i++
	}
	matchingLogs := make([][]*LogMemoryPageFactContinuous, 0)

	end := from
	start := end - blockRangeStep
	for len(requiredPagesHashes) > 0 {
		logs, err := c.ethclient.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(start),
			ToBlock:   new(big.Int).SetUint64(end),
			Topics:    c.logMemoryPageFactContinuousTopics,
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			start += (end - start) / 2
			c.backoff.BackOff(ctx, err)
			continue
		}
		c.backoff.BackOff(ctx, err)
		logsInRange := make([]*LogMemoryPageFactContinuous, 0)
		for _, log := range logs { //nolint:gocritic
			logMemoryPageFactContinuous := new(LogMemoryPageFactContinuous)
			if err := c.pageFactRegistryABI.UnpackIntoInterface(logMemoryPageFactContinuous, "LogMemoryPageFactContinuous", log.Data); err != nil {
				return nil, fmt.Errorf("unpack LogMemoryPageFactContinuous: %v", err)
			}
			logMemoryPageFactContinuous.Raw = log
			for i, pageHash := range requiredPagesHashes {
				if logMemoryPageFactContinuous.MemoryHash.Cmp(pageHash) == 0 {
					logsInRange = append(logsInRange, logMemoryPageFactContinuous)
					delete(requiredPagesHashes, i)
				}
			}
		}
		matchingLogs = append(matchingLogs, logsInRange)

		end = start
		start -= blockRangeStep
	}
	slices.Reverse(matchingLogs)
	return utils.Flatten(matchingLogs...), nil
}

func (c *Client) EncodedStateDiff(ctx context.Context, txHashes []common.Hash) ([]*big.Int, error) {
	if len(txHashes) == 0 {
		return nil, errors.New("no txHashes provided")
	}
	encodedDiff := make([]*big.Int, 0)
	txHashes = txHashes[1:] // Skip the first memory page.
	for _, txHash := range txHashes {
		var tx *types.Transaction
		for {
			var err error
			tx, _, err = c.ethclient.TransactionByHash(ctx, txHash)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			c.backoff.BackOff(ctx, err)
			if err == nil {
				break
			}
		}
		data := tx.Data()
		method, err := c.pageFactRegistryABI.MethodById(data[:4])
		if err != nil {
			return nil, fmt.Errorf("get registerContinuousMemoryPage method by id %s: %v", data[:4], err)
		}
		abiData, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			return nil, fmt.Errorf("unpack registerContinuousMemoryPage call data: %v", err)
		}

		mempage, ok := abiData[1].([]*big.Int) // We are interested in the second argument.)
		if !ok {
			return nil, fmt.Errorf("values array cannot be cast to []*big.Int, type is %T", mempage)
		}

		encodedDiff = append(encodedDiff, mempage...)
	}
	return encodedDiff, nil
}
