package sync

import (
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	starknetTypes "github.com/NethermindEth/juno/pkg/starknet/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"math/big"
)

// Synchronizer represents the base struct for Starknet Synchronization
type Synchronizer struct {
	ethereumClient      *ethclient.Client
	feederGatewayClient *feeder.Client
	database            db.Databaser
	transactioner       db.Transactioner2
	memoryPageHash      *starknetTypes.Dictionary
	gpsVerifier         *starknetTypes.Dictionary
	facts               *starknetTypes.Dictionary
	chainID             int64
}

// NewSynchronizer creates a new Synchronizer
func NewSynchronizer(txnDb db.TransactionalDb, client *ethclient.Client, fClient *feeder.Client) *Synchronizer {
	var chainID *big.Int
	if client == nil {
		// notest
		if config.Runtime.Starknet.Network == "mainnet" {
			chainID = new(big.Int).SetInt64(1)
		} else {
			chainID = new(big.Int).SetInt64(0)
		}
	} else {
		var err error
		chainID, err = client.ChainID(context.Background())
		if err != nil {
			// notest
			log.Default.Panic("Unable to retrieve chain ID from Ethereum Node")
		}
	}
	return &Synchronizer{
		ethereumClient:      client,
		feederGatewayClient: fClient,
		database:            txnDb,
		memoryPageHash:      starknetTypes.NewDictionary(txnDb, "memory_pages"),
		gpsVerifier:         starknetTypes.NewDictionary(txnDb, "gps_verifier"),
		facts:               starknetTypes.NewDictionary(txnDb, "facts"),
		chainID:             chainID.Int64(),
		transactioner:       txnDb,
	}
}

// UpdateState initiates network syncing. Syncing will occur against the
// feeder gateway or Layer 1 depending on the configuration.
// notest
func (s *Synchronizer) UpdateState() error {
	log.Default.Info("Starting to update state")

	if config.Runtime.Starknet.ApiSync {
		return s.apiSync()
	}
	return s.l1Sync()
}

func (s *Synchronizer) Close() {
	s.database.Close()
}
