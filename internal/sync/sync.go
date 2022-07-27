package sync

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/NethermindEth/juno/internal/config"
	blockDB "github.com/NethermindEth/juno/internal/db/block"
	"github.com/NethermindEth/juno/internal/db/sync"
	"github.com/NethermindEth/juno/internal/db/transaction"
	. "github.com/NethermindEth/juno/internal/log"
	"github.com/NethermindEth/juno/pkg/feeder"
	"github.com/NethermindEth/juno/pkg/felt"
	"github.com/NethermindEth/juno/pkg/state"
	"github.com/NethermindEth/juno/pkg/types"
	"go.uber.org/zap"
)

type Synchronizer struct {
	// feeder is the client that will be used to fetch the data that comes from the Feeder Gateway.
	feeder *feeder.Client
	// l1Client represent the ethereum client
	l1Client L1Client

	logger *zap.SugaredLogger

	// startingBlockNumber is the block number of the first block that we will sync.
	startingBlockNumber int64
	// startingBlockHash is the hash of the first block that we will sync.
	startingBlockHash string

	// latestBlockNumberSynced is the last block that was synced.
	latestBlockNumberSynced int64
	// latestBlockHashSynced is the last block that was synced.
	latestBlockHashSynced *felt.Felt

	// highestBlockNumber is the highest block number that we have synced.
	highestBlockNumber int64
	// highestBlockHash is the highest block hash that we have synced.
	highestBlockHash *felt.Felt

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
}

func NewSynchronizer(feederClient *feeder.Client, syncManager *sync.Manager,
	stateManager state.StateManager, blockManager *blockDB.Manager, transactionManager *transaction.Manager,
) *Synchronizer {
	synchro := new(Synchronizer)
	synchro.logger = Logger.Named("Sync Service")
	synchro.feeder = feederClient
	if !config.Runtime.Starknet.ApiSync {
		ethereumClient, err := ethclient.Dial(config.Runtime.Ethereum.Node)
		if err != nil {
			synchro.logger.Fatal("Unable to connect to Ethereum Client", err)
		}
		synchro.l1Client = ethereumClient
	}

	synchro.syncManager = syncManager
	synchro.stateManager = stateManager
	synchro.blockManager = blockManager
	synchro.transactionManager = transactionManager

	synchro.Running = false

	synchro.setChainId()
	if config.Runtime.Starknet.ApiSync {
		synchro.stateDiffCollector = NewApiCollector(synchro.syncManager, synchro.feeder, int(synchro.chainId.Int64()))
	} else {
		synchro.stateDiffCollector = NewL1Collector(synchro.syncManager, synchro.feeder, synchro.l1Client,
			int(synchro.chainId.Int64()))
	}
	synchro.setStateToLatestRoot()
	go func() {
		synchro.stateDiffCollector.Run()
	}()
	return synchro
}

// Run starts the service.
func (s *Synchronizer) Run() {
	s.Running = true
	go s.sync()
}

func (s *Synchronizer) sync() {
	// Get state
	for stateDiff := range s.stateDiffCollector.GetChannel() {
		start := time.Now()

		err := s.updateState(stateDiff)
		if err != nil || s.state.Root().Cmp(stateDiff.NewRoot) != 0 {
			// In case some errors exist or the new root of the trie didn't match with
			// the root we receive from the StateDiff, we have to revert the trie
			s.setStateToLatestRoot()
			continue
		}

		//if err = s.updateBlock(stateDiff.BlockNumber); err != nil {
		//	s.setStateToLatestRoot()
		//	continue
		//}

		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Missing Blocks to fully Sync", int64(s.stateDiffCollector.LatestBlock().BlockNumber)-stateDiff.BlockNumber,
			"Timer", time.Since(start)).
			Info("Synced block")
		s.syncManager.StoreLatestBlockSync(stateDiff.BlockNumber)
		if stateDiff.OldRoot.Hex() == "" {
			stateDiff.OldRoot = new(felt.Felt).SetHex(s.syncManager.GetLatestStateRoot())
		}
		s.syncManager.StoreLatestStateRoot(s.state.Root().Hex0x())
		s.syncManager.StoreStateDiff(stateDiff, s.latestBlockHashSynced.Hex0x())
		s.latestBlockNumberSynced = stateDiff.BlockNumber

		// Used to keep a track of where the sync started
		if s.startingBlockHash == "" {
			s.startingBlockNumber = stateDiff.BlockNumber
			s.startingBlockHash = s.latestBlockHashSynced.Hex0x()
		}

	}
}

func (s *Synchronizer) Status() *types.SyncStatus {
	return &types.SyncStatus{
		StartingBlockHash:   s.startingBlockHash,
		StartingBlockNumber: fmt.Sprintf("%x", s.startingBlockNumber),
		CurrentBlockHash:    s.latestBlockHashSynced.Hex0x(),
		CurrentBlockNumber:  fmt.Sprintf("%x", s.latestBlockNumberSynced),
		HighestBlockHash:    s.stateDiffCollector.LatestBlock().BlockHash,
		HighestBlockNumber:  fmt.Sprintf("%x", s.stateDiffCollector.LatestBlock().BlockNumber),
	}
}

func (s *Synchronizer) updateState(stateDiff *types.StateDiff) error {
	for _, deployedContract := range stateDiff.DeployedContracts {
		err := s.SetCode(stateDiff, deployedContract)
		if err != nil {
			return err
		}
	}

	for contractAddress, memoryCells := range stateDiff.StorageDiff {
		for _, cell := range memoryCells {
			err := s.state.SetSlot(new(felt.Felt).SetString(contractAddress), cell.Address, cell.Value)
			if err != nil {
				return err
			}
		}
	}
	s.logger.With("Block Number", stateDiff.BlockNumber).Debug("State updated")
	return nil
}

func (s *Synchronizer) SetCode(stateDiff *types.StateDiff, deployedContract types.DeployedContract) error {
	// Get Full Contract
	contractFromApi, err := s.feeder.GetFullContractRaw(deployedContract.Address.Hex0x(), "",
		strconv.FormatInt(stateDiff.BlockNumber, 10))
	if err != nil {
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Contract Address", deployedContract.Address.Hex0x()).
			Error("Error getting full contract")
		return err
	}

	contract := new(types.Contract)
	err = contract.UnmarshalRaw(contractFromApi)
	if err != nil {
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Contract Address", deployedContract.Address.Hex0x()).
			Error("Error unmarshalling contract")
		return err
	}
	err = s.state.SetContract(deployedContract.Address, deployedContract.Hash, contract)
	if err != nil {
		s.logger.With("Block Number", stateDiff.BlockNumber,
			"Contract Address", deployedContract.Address).
			Error("Error setting code")
		return err
	}
	s.logger.With("Block Number", stateDiff.BlockNumber).Debug("State updated for Contract")
	return nil
}

func (s *Synchronizer) GetStateDiff(blockNumber int64) *types.StateDiff {
	return s.syncManager.GetStateDiff(blockNumber)
}

func (s *Synchronizer) GetStateDiffFromHash(blockHash string) *types.StateDiff {
	return s.syncManager.GetStateDiffFromHash(blockHash)
}

func (s *Synchronizer) LatestBlockSynced() (blockNumber int64, blockHash *felt.Felt) {
	return s.latestBlockNumberSynced, s.latestBlockHashSynced
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
}

func (s *Synchronizer) setChainId() {
	if s.l1Client == nil {
		// notest
		if config.Runtime.Starknet.Network == "mainnet" {
			s.chainId = new(big.Int).SetInt64(1)
		} else {
			s.chainId = new(big.Int).SetInt64(0)
		}
	} else {
		var err error
		s.chainId, err = s.l1Client.ChainID(context.Background())
		if err != nil {
			// notest
			Logger.Panic("Unable to retrieve chain ID from Ethereum Node")
		}
	}
}

func (s *Synchronizer) updateBlock(number int64) error {
	block, err := s.UpdateBlock(number)
	if err != nil {
		return err
	}
	s.latestBlockHashSynced = new(felt.Felt).SetHex(block.BlockHash)
	return nil
}

func (s *Synchronizer) UpdateBlock(blockNumber int64) (*feeder.StarknetBlock, error) {
	block, err := s.feeder.GetBlock("", strconv.FormatInt(blockNumber, 10))
	if err != nil {
		return nil, err
	}

	for _, txn := range block.Transactions {
		err = s.updateTransactions(txn)
		if err != nil {
			iterations := 0
			for {
				time.Sleep(time.Second * 1)
				if err = s.updateTransactions(txn); err == nil {
					break
				}
				if iterations > 20 {
					return nil, err
				}
				iterations++
			}
			if err != nil {
				return nil, err
			}
		}
	}
	for i, txnReceipt := range block.TransactionReceipts {
		err = s.updateTransactionReceipts(txnReceipt, block.Transactions[i].Type)
		if err != nil {
			iterations := 0
			for {
				time.Sleep(time.Second * 1)
				if err = s.updateTransactionReceipts(txnReceipt, block.Transactions[i].Type); err == nil {
					break
				}
				if iterations > 20 {
					return nil, err
				}
				iterations++
			}
			if err != nil {
				return nil, err
			}
		}
	}

	blockHash := new(felt.Felt).SetHex(block.BlockHash)
	err = s.blockManager.PutBlock(blockHash, feederBlockToDBBlock(block))
	if err != nil {
		return nil, err
	}
	return block, nil
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

func feederTransactionToDBReceipt(receipt *feeder.TransactionReceipt, txnType string) types.TxnReceipt {
	common := types.TxnReceiptCommon{
		TxnHash:     new(felt.Felt).SetHex(receipt.TransactionHash),
		ActualFee:   new(felt.Felt).SetHex(receipt.ActualFee),
		Status:      types.TxStatusValue[receipt.Status],
		StatusData:  receipt.TxStatus,
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
		payloadFromL1ToL2 := make([]*felt.Felt, 0)
		for _, p := range receipt.L1ToL2Message.Payload {
			payloadFromL1ToL2 = append(payloadFromL1ToL2, new(felt.Felt).SetHex(p))
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
			L1OriginMessage: &types.MsgToL2{
				FromAddress: types.HexToEthAddress(receipt.L1ToL2Message.FromAddress),
				Payload:     payloadFromL1ToL2,
				ToAddress:   new(felt.Felt).SetHex(receipt.L1ToL2Message.ToAddress),
			},
			Events: events,
		}
	case "DECLARE":
		return &types.TxnDeclareReceipt{TxnReceiptCommon: common}
	case "DEPLOY":
		return &types.TxnInvokeReceipt{TxnReceiptCommon: common}

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
	status, _ := types.BlockStatusValue[b.Status]
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
		return &types.TransactionDeploy{
			Hash:                new(felt.Felt).SetHex(info.Transaction.TransactionHash),
			ContractAddress:     new(felt.Felt).SetHex(info.Transaction.ContractAddress),
			ConstructorCallData: calldata,
		}
	}
}
