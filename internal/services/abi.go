package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/db/abi"
	"go.uber.org/zap"
)

var (
	abiService ABIService
)

type ABIService struct {
	started      bool
	storeChannel chan storeInstruction
	db           *abi.Manager
	logger       *zap.SugaredLogger
}

func NewABIService() *ABIService {
	database := db.Databaser(db.New(config.Runtime.DbPath+"/abi", 0))
	storeChannel := make(chan storeInstruction, 100)
	abiService = ABIService{
		started:      false,
		storeChannel: storeChannel,
		db:           abi.NewABIManager(database),
		logger:       log.Default.Named("ABI service"),
	}
	return &abiService
}

func (service *ABIService) Run() error {
	service.started = true
	for {
		// TODO: Check if the channel is closed
		select {
		case storeInst := <-service.storeChannel:
			service.logger.
				With("Contract address", storeInst.ContractAddress).
				Info("Fetching ABI from contract address")
			err := service.db.PutABI(storeInst.ContractAddress, storeInst.Abi)
			if err != nil {
				service.logger.
					With("Error", err, "Contract address", storeInst.ContractAddress).
					Error("Error storing the ABI in the database")
				return err
			}
		}
	}
}

func (service *ABIService) Close(ctx context.Context) {
	service.logger.Info("Closing service...")
	close(service.storeChannel)
	service.logger.Info("Closed")
}

type storeInstruction struct {
	ContractAddress string
	Abi             *abi.Abi
}

func (service *ABIService) StoreABI(contractAddress string, abi abi.Abi) {
	service.storeChannel <- storeInstruction{
		ContractAddress: contractAddress,
		Abi:             &abi,
	}
}

func (service *ABIService) GetABI(contractAddress string) (*abi.Abi, error) {
	return service.db.GetABI(contractAddress)
}

func GetABIService() *ABIService {
	if abiService.started {
		return &abiService
	}
	return nil
}
