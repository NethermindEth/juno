package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/db/transaction"
	"go.uber.org/zap"
	"math/big"
	"sync"
)

// TransactionService is a service to manage the transaction database. Before
// using the service, it must be configured with the Setup method;
// otherwise, the value will be the default. To stop the service, call the
// Close method.
var TransactionService transactionService

type transactionService struct {
	running bool
	manager *transaction.Manager
	logger  *zap.SugaredLogger
	wg      sync.WaitGroup
}

// Setup is used to configure the service before it's started. The database
// param is the database where the transactions will be stored.
func (s *transactionService) Setup(database db.Databaser) {
	s.manager = transaction.NewManager(database)
}

// Run starts the service. If the Setup method is not called before, the default
// values are used.
func (s *transactionService) Run() error {
	if s.running {
		s.logger.Warn("service is already running")
		return nil
	}
	s.running = true
	s.logger = log.Default.Named("Transaction Service")

	if s.manager == nil {
		database := db.New(config.DataDir+"/transaction", 0)
		s.manager = transaction.NewManager(database)
	}

	return nil
}

// Close stops the service, waiting to end the current operations, and closes
// the database manager.
func (s *transactionService) Close(_ context.Context) {
	if !s.running {
		s.logger.Warn("service is not running")
		return
	}
	s.running = false
	s.wg.Wait()
	s.manager.Close()
}

// GetTransaction searches for the transaction associated with the given
// transaction hash. If the transaction does not exist on the database, then
// returns nil.
func (s *transactionService) GetTransaction(transactionHash string) *transaction.Transaction {
	s.wg.Add(1)
	defer s.wg.Done()

	s.logger.
		With("transactionHash", transactionHash).
		Debug("GetTransaction")

	key, ok := new(big.Int).SetString(transactionHash, 16)
	if !ok {
		log.Default.
			With("transactionHash", transactionHash).
			Panicf("error decoding transaction type into big.Int")
	}

	return s.manager.GetTransaction(*key)
}

// StoreTransaction stores the given transaction into the database. The key used
// to map the transaction it's the hash of the transaction. If the database
// already has a transaction with the same key, then the value is overwritten.
func (s *transactionService) StoreTransaction(tx *transaction.Transaction) {
	s.wg.Add(1)
	defer s.wg.Done()

	s.logger.
		With("transactionHash").
		Debug("StoreTransaction")

	s.manager.PutTransaction(tx)
}
