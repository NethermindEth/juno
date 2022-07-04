package services

import (
	"context"

	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/types"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/internal/log"
)

// TransactionService is a service to manage the transaction database. Before
// using the service, it must be configured with the Setup method;
// otherwise, the value will be the default. To stop the service, call the
// Close method.
var TransactionService transactionService

type transactionService struct {
	service
	manager *transaction.Manager
}

// Setup is used to configure the service before it's started. The database
// param is the database where the transactions will be stored.
func (s *transactionService) Setup(txDb, receiptDb db.Database) {
	if s.service.Running() {
		// notest
		s.logger.Panic("trying to Setup with service running")
	}
	s.manager = transaction.NewManager(txDb, receiptDb)
}

// Run starts the service. If the Setup method is not called before, the default
// values are used.
func (s *transactionService) Run() error {
	if s.logger == nil {
		s.logger = log.Default.Named("Transaction Service")
	}

	if err := s.service.Run(); err != nil {
		// notest
		return err
	}

	return s.setDefaults()
}

func (s *transactionService) setDefaults() error {
	if s.manager == nil {
		// notest
		env, err := db.GetMDBXEnv()
		if err != nil {
			return err
		}
		txDb, err := db.NewMDBXDatabase(env, "TRANSACTION")
		if err != nil {
			return err
		}
		receiptDb, err := db.NewMDBXDatabase(env, "RECEIPT")
		if err != nil {
			return err
		}
		s.manager = transaction.NewManager(txDb, receiptDb)
	}
	return nil
}

// Close stops the service, waiting to end the current operations, and closes
// the database manager.
func (s *transactionService) Close(ctx context.Context) {
	// notest
	if !s.Running() {
		return
	}
	s.service.Close(ctx)
	s.manager.Close()
}

// GetTransaction searches for the transaction associated with the given
// transaction hash. If the transaction does not exist on the database, then
// returns nil.
func (s *transactionService) GetTransaction(txHash *felt.Felt) types.IsTransaction {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("txHash", txHash).
		Debug("GetTransaction")

	return s.manager.GetTransaction(txHash)
}

// StoreTransaction stores the given transaction into the database. The key used
// to map the transaction it's the hash of the transaction. If the database
// already has a transaction with the same key, then the value is overwritten.
func (s *transactionService) StoreTransaction(txHash *felt.Felt, tx types.IsTransaction) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.
		With("txHash", txHash.String()).
		Debug("StoreTransaction")

	s.manager.PutTransaction(txHash, tx)
}

// GetReceipt searches for the transaction receipt associated with the given
// transaction hash. If the transaction does not exist on the database, then
// returns nil.
func (s *transactionService) GetReceipt(txHash *felt.Felt) *types.TransactionReceipt {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.With("txHash", txHash).Debug("GetReceipt")

	return s.manager.GetReceipt(txHash)
}

// StoreReceipt stores the given transaction receipt into the database. If the
// database already has a receipt with the same key, the value is overwritten.
func (s *transactionService) StoreReceipt(txHash *felt.Felt, receipt *types.TransactionReceipt) {
	s.AddProcess()
	defer s.DoneProcess()

	s.logger.With("txHash", txHash).Debug("StoreReceipt")

	s.manager.PutReceipt(txHash, receipt)
}
