package sync

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	sync2 "sync"
	"time"

	blockDB "github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/db/transaction"
	"github.com/NethermindEth/juno/internal/metrics/prometheus"
	"github.com/NethermindEth/juno/internal/utils"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/log"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Synchronizer struct {
	// feeder is the client that will be used to fetch the data that comes from the Feeder Gateway.
	feeder *feeder.Client
	// l1Client represent the ethereum client
	l1Client L1Client

	logger log.Logger

	// startingBlockNumber is the block number of the first block that we will sync.
	startingBlockNumber      int64
	startingBlockNumberSaved int64

	// latestBlockNumberSynced is the last block that was synced.
	latestBlockNumberSynced int64

	// stateDIffCollector
	stateDiffCollector StateDiffCollector
	// state represent the state of the trie
	state state.State

	// chainId represent the Chain ID of the node
	chainId *big.Int

	// manager is the sync manager.
	syncManager *sync.Manager
	// blockManager represent the manager that store and serve the blocks
	blockManager *blockDB.Manager
	// transactionManager represent the manager that store and serve the transactions and receipts
	transactionManager *transaction.Manager
	// stateManager represent the manager for the state
	stateManager state.StateManager

	// Running is true when the service is active and running
	Running bool

	wg   *sync2.WaitGroup
	quit chan struct{}
}

// NewSynchronizer creates a new Synchronizer.
// notest
func NewSynchronizer(n utils.Network, ethNode string, feederClient *feeder.Client,
	syncManager *sync.Manager, stateManager state.StateManager, blockManager *blockDB.Manager,
	transactionManager *transaction.Manager, logger log.Logger,
) (*Synchronizer, error) {
	synchro := &Synchronizer{
		logger: logger,
		feeder: feederClient,
		quit:   make(chan struct{}),
		wg:     new(sync2.WaitGroup),
	}

	trusted := ethNode == ""

	if trusted {
		synchro.logger.Info("Defaulting to syncing from gateway")
	} else {
		synchro.logger.Infow("Connecting to ethnode", "address", ethNode)
		ethereumClient, err := ethclient.Dial(ethNode)
		if err != nil {
			synchro.logger.Errorw("Cannot connect to Ethereum Client", "error", err)
			return nil, err
		}
		synchro.l1Client = ethereumClient
	}

	synchro.syncManager = syncManager
	synchro.stateManager = stateManager
	synchro.blockManager = blockManager
	synchro.transactionManager = transactionManager

	synchro.Running = false

	if err := synchro.setChainId(n.String()); err != nil {
		return nil, err
	}
	synchro.setStateToLatestRoot()
	synchro.setStateDiffCollector(trusted, logger)
	return synchro, nil
}

// setStateDiffCollector sets the stateDiffCollector.
func (s *Synchronizer) setStateDiffCollector(apiSync bool, logger log.Logger) {
	if apiSync {
		s.stateDiffCollector = NewApiCollector(s.syncManager, s.feeder, logger.Named("api"))
	} else {
		s.stateDiffCollector = NewL1Collector(s.syncManager, s.feeder, s.l1Client,
			int(s.chainId.Int64()), logger.Named("l1"))
	}
}

// Run starts the service.
func (s *Synchronizer) Run() {
	s.Running = true
	go s.updateBlocksInfo()
	go s.handleSync()
}

func (s *Synchronizer) handleSync() {
	s.wg.Add(1)
	for {
		if err := s.sync(); err != nil {
			s.logger.Infow("Sync Failed, restarting iterator in 10 seconds", "error", err)
			time.Sleep(10 * time.Second)
			s.stateDiffCollector.Close()
			continue
		}
		s.wg.Done()
		return
	}
}

func (s *Synchronizer) sync() error {
	go s.stateDiffCollector.Run()

	blockNumber := s.syncManager.GetLatestBlockSync()
	s.startingBlockNumber = blockNumber
	s.latestBlockNumberSynced = s.startingBlockNumber

	// Get state
	for collectedDiff := range s.stateDiffCollector.GetChannel() {
		select {
		case <-s.quit:
			return nil
		default:
			start := time.Now()

			err := s.updateState(collectedDiff)
			if err != nil || s.state.Root().Cmp(collectedDiff.stateDiff.NewRoot) != 0 {
				// In case some errors exist or the new root of the trie didn't match with
				// the root we receive from the StateDiff, we have to revert the trie
				s.logger.Errorw("State update failed, reverting state", "error", err)
				prometheus.IncreaseCountStarknetStateFailed()
				s.setStateToLatestRoot()
				return err
			}
			prometheus.IncreaseCountStarknetStateSuccess()
			prometheus.UpdateStarknetSyncTime(time.Since(start).Seconds())
			s.syncManager.StoreLatestBlockSync(collectedDiff.stateDiff.BlockNumber)
			if err := s.syncManager.StoreStateUpdate(collectedDiff.stateDiff, collectedDiff.stateDiff.BlockHash); err != nil {
				return err
			}
			if collectedDiff.stateDiff.OldRoot.Hex() == "" {
				collectedDiff.stateDiff.OldRoot = new(felt.Felt).SetHex(s.syncManager.GetLatestStateRoot())
			}
			s.syncManager.StoreLatestStateRoot(s.state.Root().Hex0x())
			s.latestBlockNumberSynced = collectedDiff.stateDiff.BlockNumber
			s.logger.Infow("Synchronized block",
				"number", collectedDiff.stateDiff.BlockNumber,
				"pending", int64(s.stateDiffCollector.LatestBlock().BlockNumber)-collectedDiff.stateDiff.BlockNumber,
				"time", time.Since(start).String(),
			)
		}
	}
	return nil
}

func (s *Synchronizer) Status() *types.SyncStatus {
	latestBlockSaved := float64(s.syncManager.GetLatestBlockSaved())

	latestBlockNumber := uint64(math.Min(float64(s.latestBlockNumberSynced), latestBlockSaved))

	block, err := s.blockManager.GetBlockByNumber(latestBlockNumber)
	if err != nil {
		return nil
	}

	startingBlockNumber := uint64(math.Min(float64(s.startingBlockNumber), latestBlockSaved))

	startingBlock, err := s.blockManager.GetBlockByNumber(startingBlockNumber)
	if err != nil {
		return nil
	}

	highestBlockHash := "pending"
	highestBlockNumber := "pending"
	if s.stateDiffCollector.LatestBlock() != nil {
		highestBlockHash = s.stateDiffCollector.LatestBlock().BlockHash
		highestBlockNumber = fmt.Sprintf("%x", s.stateDiffCollector.LatestBlock().BlockNumber)
	}

	return &types.SyncStatus{
		StartingBlockHash:   startingBlock.BlockHash.Hex0x(),
		StartingBlockNumber: fmt.Sprintf("%x", startingBlockNumber),
		CurrentBlockHash:    block.BlockHash.Hex0x(),
		CurrentBlockNumber:  fmt.Sprintf("%x", block.BlockNumber),
		HighestBlockHash:    highestBlockHash,
		HighestBlockNumber:  highestBlockNumber,
	}
}

func (s *Synchronizer) updateState(collectedDiff *CollectorDiff) error {
	for _, deployedContract := range collectedDiff.stateDiff.DeployedContracts {
		if err := s.SetCode(collectedDiff, &deployedContract); err != nil {
			return err
		}
	}

	for contractAddress, memoryCells := range collectedDiff.stateDiff.StorageDiff {
		slots := make([]state.Slot, 0, len(memoryCells))
		for _, cell := range memoryCells {
			slots = append(slots, state.Slot{Key: cell.Address, Value: cell.Value})
		}
		if err := s.state.SetSlots(&contractAddress, slots); err != nil {
			return err
		}
	}
	s.logger.Debugw("State updated", "Block Number", collectedDiff.stateDiff.BlockNumber)
	return nil
}

func (s *Synchronizer) SetCode(collectedDiff *CollectorDiff, deployedContract *types.DeployedContract) error {
	if deployedContract == nil {
		return errors.New("contract not deployed")
	}
	err := s.state.SetContract(deployedContract.Address, deployedContract.Hash,
		collectedDiff.Code[deployedContract.Address.Hex0x()])
	if err != nil {
		s.logger.Errorw("Error setting code", "blockNumber", collectedDiff.stateDiff.BlockNumber, "address", deployedContract.Address)
		return err
	}
	s.logger.Debugw("State updated for Contract", "blockNumber", collectedDiff.stateDiff.BlockNumber, "address", deployedContract.Address)
	return nil
}

func (s *Synchronizer) GetStateDiff(blockHash *felt.Felt) (*types.StateUpdate, error) {
	return s.syncManager.GetStateUpdate(blockHash)
}

func (s *Synchronizer) LatestBlockSynced() (blockNumber int64, blockHash *felt.Felt) {
	latestBlockNumber := uint64(math.Min(float64(s.latestBlockNumberSynced), float64(s.syncManager.GetLatestBlockSaved())))

	block, err := s.blockManager.GetBlockByNumber(latestBlockNumber)
	if err != nil {
		return 0, nil
	}
	return int64(block.BlockNumber), block.BlockHash
}

func (s *Synchronizer) GetLatestBlockOnChain() int64 {
	return int64(s.stateDiffCollector.LatestBlock().BlockNumber)
}

func (s *Synchronizer) ChainID() *big.Int {
	return s.chainId
}

func (s *Synchronizer) GetPendingBlock() *feeder.StarknetBlock {
	return s.stateDiffCollector.PendingBlock()
}

func (s *Synchronizer) setStateToLatestRoot() {
	stateRoot := s.syncManager.GetLatestStateRoot()
	root := new(felt.Felt).SetHex(stateRoot)
	s.state = state.New(s.stateManager, root)
}

// Close closes the service.
func (s *Synchronizer) Close() {
	s.stateDiffCollector.Close()
	close(s.quit)
	s.wg.Wait()
}

func (s *Synchronizer) setChainId(network string) error {
	if s.l1Client == nil {
		// notest
		if network == "mainnet" {
			s.chainId = new(big.Int).SetInt64(1)
		} else {
			s.chainId = new(big.Int).SetInt64(0)
		}
	} else {
		var err error
		s.chainId, err = s.l1Client.ChainID(context.Background())
		if err != nil {
			// notest
			return fmt.Errorf("retrieve chain ID from Ethereum node: %w", err)
		}
	}
	return nil
}

func (s *Synchronizer) updateBlocks(number int64) error {
	err := s.updateBlock(number)
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) updateBlock(blockNumber int64) error {
	block, err := s.feeder.GetBlock("", strconv.FormatInt(blockNumber, 10))
	if err != nil {
		return err
	}

	for _, txn := range block.Transactions {
		err = s.updateTransactions(txn)
		if err != nil {
			iterations := 0
			for {
				time.Sleep(time.Second * 20)
				if err = s.updateTransactions(txn); err == nil {
					break
				}
				if iterations > 20 {
					return err
				}
				iterations++
			}
			if err != nil {
				return err
			}
		}
	}
	for i, txnReceipt := range block.TransactionReceipts {
		err = s.updateTransactionReceipts(txnReceipt, block.Transactions[i].Type)
		if err != nil {
			iterations := 0
			for {
				time.Sleep(time.Second * 20)
				if err = s.updateTransactionReceipts(txnReceipt, block.Transactions[i].Type); err == nil {
					break
				}
				if iterations > 20 {
					return err
				}
				iterations++
			}
			if err != nil {
				return err
			}
		}
	}

	blockHash := new(felt.Felt).SetHex(block.BlockHash)
	err = s.blockManager.PutBlock(blockHash, feederBlockToDBBlock(block))
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) updateTransactions(txn feeder.TxnSpecificInfo) error {
	transactionInfo, err := s.feeder.GetTransaction(txn.TransactionHash, "")
	if err != nil {
		return err
	}

	transactionHash := new(felt.Felt).SetHex(transactionInfo.Transaction.TransactionHash)
	// transactionHash := types.HexToTransactionHash(transactionInfo.Transaction.TransactionHash)
	err = s.transactionManager.PutTransaction(transactionHash, feederTransactionToDBTransaction(transactionInfo))
	if err != nil {
		return err
	}
	return nil
}

func (s *Synchronizer) updateTransactionReceipts(receipt feeder.TransactionExecution, txnType string) error {
	txnReceipt, err := s.feeder.GetTransactionReceipt(receipt.TransactionHash, "")
	if err != nil {
		return err
	}
	transactionHash := new(felt.Felt).SetHex(receipt.TransactionHash)
	return s.transactionManager.PutReceipt(transactionHash, feederTransactionToDBReceipt(txnReceipt, txnType))
}

func (s *Synchronizer) updateBlocksInfo() {
	s.wg.Add(1)
	latestBlockInfoFetched := s.syncManager.GetLatestBlockSaved()
	currentBlock := latestBlockInfoFetched
	s.startingBlockNumberSaved = currentBlock
	for {
		select {
		case <-s.quit:
			s.wg.Done()
			return
		default:
			latestBlock := s.stateDiffCollector.LatestBlock()
			if latestBlock == nil {
				time.Sleep(time.Second * 1)
				continue
			}
			if currentBlock == int64(latestBlock.BlockNumber) {
				time.Sleep(time.Minute)
			}
			err := s.updateBlocks(currentBlock)
			if err != nil {
				time.Sleep(time.Minute)
				continue
			}
			s.logger.Infow("Updated block info", "Block Number", currentBlock)
			s.syncManager.StoreLatestBlockSaved(currentBlock)
			currentBlock++
		}
	}
}

func feederTransactionToDBReceipt(receipt *feeder.TransactionReceipt, txnType string) types.TxnReceipt {
	common := types.TxnReceiptCommon{
		TxnHash:     new(felt.Felt).SetHex(receipt.TransactionHash),
		ActualFee:   new(felt.Felt).SetHex(receipt.ActualFee),
		Status:      types.TxStatusValue[receipt.Status],
		BlockHash:   new(felt.Felt).SetHex(receipt.BlockHash),
		BlockNumber: uint64(receipt.BlockNumber),
	}
	switch txnType {
	case "INVOKE_FUNCTION":
		l2ToL1 := make([]*types.MsgToL1, 0)
		for _, msg := range receipt.L2ToL1Messages {

			payload := make([]*felt.Felt, 0)
			for _, p := range msg.Payload {
				payload = append(payload, new(felt.Felt).SetHex(p))
			}

			l2ToL1 = append(l2ToL1, &types.MsgToL1{
				ToAddress:   types.HexToEthAddress(msg.ToAddress),
				FromAddress: new(felt.Felt).SetHex(msg.FromAddress),
				Payload:     payload,
			})
		}
		var msgToL2 *types.MsgToL2
		if receipt.L1ToL2Message != nil {
			payload := make([]*felt.Felt, 0, len(receipt.L1ToL2Message.Payload))
			for _, item := range receipt.L1ToL2Message.Payload {
				payload = append(payload, new(felt.Felt).SetHex(item))
			}
			msgToL2 = &types.MsgToL2{
				FromAddress: types.HexToEthAddress(receipt.L1ToL2Message.ToAddress),
				ToAddress:   new(felt.Felt).SetHex(receipt.L1ToL2Message.FromAddress),
				Payload:     payload,
			}
		}

		events := make([]*types.Event, 0)

		for _, event := range receipt.Events {

			keys := make([]*felt.Felt, 0)
			for _, p := range event.Keys {
				keys = append(keys, new(felt.Felt).SetHex(p))
			}
			data := make([]*felt.Felt, 0)
			for _, p := range event.Data {
				data = append(data, new(felt.Felt).SetHex(p))
			}

			events = append(events, &types.Event{
				FromAddress: new(felt.Felt).SetHex(event.FromAddress),
				Keys:        keys,
				Data:        data,
			})
		}

		return &types.TxnInvokeReceipt{
			TxnReceiptCommon: common,
			MessagesSent:     l2ToL1,
			L1OriginMessage:  msgToL2,
			Events:           events,
		}
	case "DECLARE":
		return &types.TxnDeclareReceipt{TxnReceiptCommon: common}
	case "DEPLOY":
		return &types.TxnDeployReceipt{TxnReceiptCommon: common}
	default:
		return &common
	}
}

// feederBlockToDBBlock convert the feeder block to the block stored in the database
func feederBlockToDBBlock(b *feeder.StarknetBlock) *types.Block {
	txnsHash := make([]*felt.Felt, 0)
	for _, data := range b.Transactions {
		txnsHash = append(txnsHash, new(felt.Felt).SetHex(data.TransactionHash))
	}
	status := types.BlockStatusValue[b.Status]
	return &types.Block{
		BlockHash:   new(felt.Felt).SetHex(b.BlockHash),
		BlockNumber: uint64(b.BlockNumber),
		ParentHash:  new(felt.Felt).SetHex(b.ParentBlockHash),
		Status:      status,
		Sequencer:   new(felt.Felt).SetHex(b.SequencerAddress),
		NewRoot:     new(felt.Felt).SetHex(b.StateRoot),
		OldRoot:     new(felt.Felt).SetHex(b.OldStateRoot),
		TimeStamp:   b.Timestamp,
		TxCount:     uint64(len(b.Transactions)),
		TxHashes:    txnsHash,
	}
}

// feederTransactionToDBTransaction convert the feeder TransactionInfo to the transaction stored in DB
func feederTransactionToDBTransaction(info *feeder.TransactionInfo) types.IsTransaction {
	calldata := make([]*felt.Felt, 0)
	for _, data := range info.Transaction.Calldata {
		calldata = append(calldata, new(felt.Felt).SetHex(data))
	}

	switch info.Transaction.Type {
	case "INVOKE_FUNCTION":
		signature := make([]*felt.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, new(felt.Felt).SetHex(data))
		}
		return &types.TransactionInvoke{
			Hash:               new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ContractAddress:    new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			EntryPointSelector: new(felt.Felt).SetHex(info.Transaction.EntryPointSelector),
			CallData:           calldata,
			Signature:          signature,
			MaxFee:             new(felt.Felt).SetHex(info.Transaction.MaxFee),
		}
	case "DECLARE":
		signature := make([]*felt.Felt, 0)
		for _, data := range info.Transaction.Signature {
			signature = append(signature, new(felt.Felt).SetHex(data))
		}
		return &types.TransactionDeclare{
			Hash:          new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ClassHash:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			SenderAddress: new(felt.Felt).SetHex(info.Transaction.SenderAddress),
			MaxFee:        new(felt.Felt).SetHex(info.Transaction.MaxFee),
			Signature:     signature,
			Nonce:         new(felt.Felt).SetHex(info.Transaction.Nonce),
			Version:       new(felt.Felt).SetHex(info.Transaction.Version),
		}
	default:
		constructorCalldata := make([]*felt.Felt, 0, len(info.Transaction.ConstructorCalldata))
		for _, data := range info.Transaction.ConstructorCalldata {
			constructorCalldata = append(constructorCalldata, new(felt.Felt).SetHex(data))
		}
		return &types.TransactionDeploy{
			Hash:                new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ContractAddress:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			ContractAddressSalt: new(felt.Felt).SetHex(info.Transaction.ContractAddressSalt),
			ClassHash:           new(felt.Felt).SetHex(info.Transaction.ClassHash),
			ConstructorCallData: constructorCalldata,
		}
	}
}
