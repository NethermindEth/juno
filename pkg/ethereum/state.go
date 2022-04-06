// Package ethereum contains all the functions related to Ethereum State and Synchronization
// with Layer 1
package ethereum

import (
	"context"
	"fmt"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/feeder_gateway"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

// Synchronizer represents the base struct for Ethereum Synchronization
type Synchronizer struct {
	ethereumClient      *ethclient.Client
	feederGatewayClient *feeder_gateway.Client
	db                  *db.Databaser
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(ethereumNode, feederGateway string, db *db.Databaser) *Synchronizer {
	client, err := ethclient.Dial(ethereumNode)
	if err != nil {
		log.Default.With("Error", err).Fatal("Unable to connect to Ethereum Client")
	}
	feeder := feeder_gateway.NewClient(feederGateway, "/feeder_gateway", nil)
	return &Synchronizer{
		ethereumClient:      client,
		feederGatewayClient: feeder,
		db:                  db,
	}
}

// UpdateStateRoot keeps updating the Ethereum State Root as a process
func (s Synchronizer) UpdateStateRoot() error {
	log.Default.Info("Starting to update state")
	headers := make(chan *types.Header)
	number, err := s.ethereumClient.BlockNumber(context.Background())
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't get the latest block")
		return err
	}
	contractAddress := common.HexToAddress("0x96375087b2F6eFc59e5e0dd5111B4d090EBFDD8B")
	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(13620297),
		ToBlock:   big.NewInt(int64(number)),
		Addresses: []common.Address{
			contractAddress,
		},
	}

	logs, err := s.ethereumClient.FilterLogs(context.Background(), query)
	if err != nil {
		log.Default.With("Error", err).Error("Couldn't filter the logs")
		return err
	}
	for _, vLog := range logs {
		fmt.Println(vLog.BlockHash.Hex()) // 0x3404b8c050aa0aacd0223e91b5c32fee6400f357764771d0684fa7b3f448f1a8
		fmt.Println(vLog.BlockNumber)     // 2394201
		fmt.Println(vLog.TxHash.Hex())    // 0x280201eda63c9ff6f305fcee51d5eb86167fab40ca3108ec784e8652a0e2b1a6
	}
	sub, err := s.ethereumClient.SubscribeNewHead(context.Background(), headers)
	if err != nil {
		log.Default.With("Error", err).Fatalf("Unable to subscribe to block headers")
		return err
	}

	for {
		select {
		case err := <-sub.Err():
			log.Default.Fatal(err)
			return err
		case header := <-headers:
			log.Default.With("stateRoot", header.Root).Debug("State root retrieved from L1")
			// TODO store ethereum state
		}
	}
}

// Close closes the client for the Layer 1 Ethereum node
func (s Synchronizer) Close(ctx context.Context) {
	// notest
	log.Default.Info("Closing Layer 1 Synchronizer")
	select {
	case <-ctx.Done():
		s.ethereumClient.Close()
	default:
	}
}
