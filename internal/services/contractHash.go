package services

import (
	"context"
	"math/big"

	"github.com/NethermindEth/juno/internal/db"
	"github.com/NethermindEth/juno/internal/log"
	"go.uber.org/zap"
)

var contractHashService ContractHashService

type ContractHashService struct {
	started      bool
	storeChannel chan contractHashInstruction
	db           *db.Databaser
	logger       *zap.SugaredLogger
}

func NewContractHashService(database db.Databaser) *ContractHashService {
	storeChannel := make(chan contractHashInstruction, 100)
	contractHashService = ContractHashService{
		started:      false,
		storeChannel: storeChannel,
		db:           &database,
		logger:       log.Default.Named("Contract Hash service"),
	}
	return &contractHashService
}

func (service *ContractHashService) Run() error {
	service.started = true
	for storeInst := range service.storeChannel {
		err := (*service.db).Put([]byte(storeInst.ContractAddress), storeInst.ContractHash)
		if err != nil {
			// notest
			log.Default.With("Error", err).Panic("Couldn't save contract hash in database")
		}
	}
	return nil
}

func (service *ContractHashService) Close(ctx context.Context) {
	service.logger.Info("Closing service...")
	close(service.storeChannel)
	(*service.db).Close()
	service.logger.Info("Closed")
}

type contractHashInstruction struct {
	ContractAddress string
	ContractHash    []byte
}

func (service *ContractHashService) StoreContractHash(contractAddress string, contractHash *big.Int) {
	service.storeChannel <- contractHashInstruction{
		ContractAddress: contractAddress,
		ContractHash:    contractHash.Bytes(),
	}
}

func (service *ContractHashService) GetContractHash(contractAddress string) *big.Int {
	get, err := (*service.db).Get([]byte(contractAddress))
	if err != nil || get == nil {
		// notest
		return new(big.Int)
	}
	return new(big.Int).SetBytes(get)
}

func GetContractHashService() *ContractHashService {
	if contractHashService.started {
		return &contractHashService
	}
	// notest
	return nil
}
