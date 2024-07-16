package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database related operations",
	Long:  `This command allows you to perform database operations.`,
}

func InitDBCommand() {
	dbCmd.AddCommand(DBInfoCmd())
	dbCmd.AddCommand(DBInspectCmd())
}

type DBInfo struct {
	Network         string     `json:"network"`
	ChainHeight     uint64     `json:"chain_height"`
	LatestBlockHash *felt.Felt `json:"latest_block_hash"`
	LatestStateRoot *felt.Felt `json:"latest_state_root"`
	L1Height        uint64     `json:"l1_height"`
	L1BlockHash     *felt.Felt `json:"l1_block_hash"`
	L1StateRoot     *felt.Felt `json:"l1_state_root"`
}

func DBInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Retrieve database information",
		Long:  `This subcommand retrieves and displays blockchain information stored in the database.`,
		RunE:  dbInfo,
	}
}

func DBInspectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "inspect",
		Short: "Inspect the database to obtain storage information",
		Long:  `This subcommand retrieves and displays the storage of each data type stored in the database.`,
		RunE:  dbInspect,
	}
}

func dbInfo(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	if _, err = os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("Database path does not exist")
		return nil
	}

	log := utils.NewNopZapLogger()
	database, err := pebble.New(dbPath, defaultCacheSizeMb, defaultMaxHandles, log)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}

	chain := blockchain.New(database, nil)
	info := DBInfo{}

	// Get the latest block information
	headBlock, err := chain.Head()
	if err == nil {
		info.Network = getNetwork(headBlock)
		info.ChainHeight = headBlock.Number
		info.LatestBlockHash = headBlock.Hash
		info.LatestStateRoot = headBlock.GlobalStateRoot
	} else {
		fmt.Printf("Failed to get the latest block information: %v\n", err)
	}

	// Get the latest L1 block information
	l1Head, err := chain.L1Head()
	if err == nil {
		info.L1Height = l1Head.BlockNumber
		info.L1BlockHash = l1Head.BlockHash
		info.L1StateRoot = l1Head.StateRoot
	} else {
		fmt.Printf("Failed to get the latest L1 block information: %v\n", err)
	}

	jsonData, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	fmt.Println(string(jsonData))

	return nil
}

//nolint:funlen,gocyclo
func dbInspect(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	if _, err = os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("Database path does not exist")
		return nil
	}

	log := utils.NewNopZapLogger()
	database, err := pebble.New(dbPath, defaultCacheSizeMb, defaultMaxHandles, log)
	if err != nil {
		return fmt.Errorf("open DB: %w", err)
	}

	txn, err := database.NewTransaction(false)
	if err != nil {
		return fmt.Errorf("new transaction: %w", err)
	}

	it, err := txn.NewIterator()
	if err != nil {
		return fmt.Errorf("new iterator: %w", err)
	}
	defer it.Close()

	var (
		start   = time.Now()
		logged  = time.Now()
		buckets = make(map[db.Bucket]*item)

		totalSize  utils.DataSize
		totalCount int64
	)

	for it.Next() {
		key := it.Key()
		val, err := it.Value()
		if err != nil {
			return fmt.Errorf("get value: %w", err)
		}

		size := utils.DataSize(len(key) + len(val))
		totalSize += size

		bucket := db.Bucket(key[0])
		if _, ok := buckets[bucket]; !ok {
			buckets[bucket] = &item{}
		}

		buckets[bucket].add(size)
		totalCount++

		if totalCount%1000 == 0 && time.Since(logged) > 8*time.Second {
			fmt.Println("Inspecting database", "count", totalCount, "size", totalSize.String(), "elapsed", time.Since(start).String())
			logged = time.Now()
		}
	}

	items := [][]string{}
	for bucket, item := range buckets {
		switch bucket {
		case db.StateTrie:
			items = append(items, []string{"StateTrie", item.getSize(), item.getCount()})
		case db.Unused:
			items = append(items, []string{"Unused", item.getSize(), item.getCount()})
		case db.ContractClassHash:
			items = append(items, []string{"ContractClassHash", item.getSize(), item.getCount()})
		case db.ContractStorage:
			items = append(items, []string{"ContractStorage", item.getSize(), item.getCount()})
		case db.Class:
			items = append(items, []string{"Class", item.getSize(), item.getCount()})
		case db.ContractNonce:
			items = append(items, []string{"ContractNonce", item.getSize(), item.getCount()})
		case db.ChainHeight:
			items = append(items, []string{"ChainHeight", item.getSize(), item.getCount()})
		case db.BlockHeaderNumbersByHash:
			items = append(items, []string{"BlockHeaderNumbersByHash", item.getSize(), item.getCount()})
		case db.BlockHeadersByNumber:
			items = append(items, []string{"BlockHeadersByNumber", item.getSize(), item.getCount()})
		case db.TransactionBlockNumbersAndIndicesByHash:
			items = append(items, []string{"TransactionBlockNumbersAndIndicesByHash", item.getSize(), item.getCount()})
		case db.TransactionsByBlockNumberAndIndex:
			items = append(items, []string{"TransactionsByBlockNumberAndIndex", item.getSize(), item.getCount()})
		case db.ReceiptsByBlockNumberAndIndex:
			items = append(items, []string{"ReceiptsByBlockNumberAndIndex", item.getSize(), item.getCount()})
		case db.StateUpdatesByBlockNumber:
			items = append(items, []string{"StateUpdatesByBlockNumber", item.getSize(), item.getCount()})
		case db.ClassesTrie:
			items = append(items, []string{"ClassesTrie", item.getSize(), item.getCount()})
		case db.ContractStorageHistory:
			items = append(items, []string{"ContractStorageHistory", item.getSize(), item.getCount()})
		case db.ContractNonceHistory:
			items = append(items, []string{"ContractNonceHistory", item.getSize(), item.getCount()})
		case db.ContractClassHashHistory:
			items = append(items, []string{"ContractClassHashHistory", item.getSize(), item.getCount()})
		case db.ContractDeploymentHeight:
			items = append(items, []string{"ContractDeploymentHeight", item.getSize(), item.getCount()})
		case db.L1Height:
			items = append(items, []string{"L1Height", item.getSize(), item.getCount()})
		case db.SchemaVersion:
			items = append(items, []string{"SchemaVersion", item.getSize(), item.getCount()})
		case db.Pending:
			items = append(items, []string{"Pending", item.getSize(), item.getCount()})
		case db.BlockCommitments:
			items = append(items, []string{"BlockCommitments", item.getSize(), item.getCount()})
		case db.Temporary:
			items = append(items, []string{"Temporary", item.getSize(), item.getCount()})
		case db.SchemaIntermediateState:
			items = append(items, []string{"SchemaIntermediateState", item.getSize(), item.getCount()})
		default:
			items = append(items, []string{fmt.Sprintf("UnknownBucket(%d)", bucket), item.getSize(), item.getCount()})
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Bucket", "Size", "Count"})
	table.SetFooter([]string{"Total", totalSize.String(), fmt.Sprintf("%d", totalCount)})
	table.AppendBulk(items)
	table.Render()

	return nil
}

func getNetwork(head *core.Block) string {
	networks := map[string]*utils.Network{
		"mainnet": &utils.Mainnet,
		"sepolia": &utils.Sepolia,
		"goerli":  &utils.Goerli,
		"goerli2": &utils.Goerli2,
	}

	for network, util := range networks {
		if _, err := core.VerifyBlockHash(head, util); err == nil {
			return network
		}
	}

	return "unknown"
}

type item struct {
	count int
	size  utils.DataSize
}

func (i *item) add(size utils.DataSize) {
	i.count++
	i.size += size
}

func (i *item) getSize() string {
	return i.size.String()
}

func (i *item) getCount() string {
	return fmt.Sprintf("%d", i.count)
}
