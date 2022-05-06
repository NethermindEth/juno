package services

import (
	"context"
	"github.com/NethermindEth/juno/internal/config"
	"github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/db"
	"github.com/NethermindEth/juno/pkg/db/state"
	"go.uber.org/zap"
)

var (
	stateService StateService
)

type StateService struct {
	started          bool
	storeCodeChannel chan storeCodeInstruction
	manager          *state.Manager
	logger           *zap.SugaredLogger
}

func NewStateService() *StateService {
	codeDatabase := db.Databaser(db.New(config.Runtime.DbPath+"/code", 0))
	storageDatabase := db.NewBlockSpecificDatabase(db.New(config.Runtime.DbPath+"/storage", 0))
	storeCodeChannel := make(chan storeCodeInstruction, 100)
	stateService = StateService{
		started:          false,
		storeCodeChannel: storeCodeChannel,
		manager:          state.NewStateManager(codeDatabase, *storageDatabase),
		logger:           log.Default.Named("Contract Code Service"),
	}
	return &stateService
}

func (service *StateService) Run() error {
	service.started = true
	service.logger.Info("Service started")
	for {
		// TODO: Check if the channel is closed
		select {
		case storeInst := <-service.storeCodeChannel:
			service.logger.
				With("Contract address", storeInst.ContractAddress).
				Info("Fetching contract code from contract address")
			service.manager.PutCode(storeInst.ContractAddress, &storeInst.Code)
			service.logger.
				With("Contract address", storeInst.ContractAddress).
				Info("Contract code saved")
		}
	}
}

func (service *StateService) Close(ctx context.Context) {
	service.logger.Info("Closing service...")
	close(service.storeCodeChannel)
	service.logger.Info("Closed")
}

type storeCodeInstruction struct {
	ContractAddress string
	Code            state.ContractCode
}

func (service *StateService) StoreCode(contractAddress string, code state.ContractCode) {
	service.storeCodeChannel <- storeCodeInstruction{
		ContractAddress: contractAddress,
		Code:            code,
	}
}

func (service *StateService) GetCode(contractAddress string) *state.ContractCode {
	contractCode := service.manager.GetCode(contractAddress)
	return contractCode
}

func GetStateService() *StateService {
	if stateService.started {
		return &stateService
	}
	return nil
}
