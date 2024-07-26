package main

import (
	"encoding/json"
	"fmt"
	"os"

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
	defaultDBPath, dbPathShort := "", "p"
	dbCmd.PersistentFlags().StringP(dbPathF, dbPathShort, defaultDBPath, dbPathUsage)

	dbCmd.AddCommand(DBInfoCmd())
	dbCmd.AddCommand(DBSizeCmd())
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

func DBSizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "size",
		Short: "Calculate database size information for each data type",
		Long:  `This subcommand retrieves and displays the storage of each data type stored in the database.`,
		RunE:  dbSize,
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

	database, err := pebble.New(dbPath)
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

func dbSize(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	if dbPath == "" {
		return fmt.Errorf("--%v cannot be empty", dbPathF)
	}

	if _, err = os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("Database path does not exist")
		return nil
	}

	pebbleDB, err := pebble.New(dbPath)
	if err != nil {
		return err
	}

	var (
		totalSize  utils.DataSize
		totalCount int64

		withHistorySize    utils.DataSize
		withoutHistorySize utils.DataSize

		withHistoryCount    int64
		withoutHistoryCount int64

		items [][]string
	)

	for _, b := range db.BucketValues() {
		fmt.Fprintf(cmd.OutOrStdout(), "Calculating size of %s, remaining buckets: %d\n", b, len(db.BucketValues())-int(b)-1)
		bucketItem, err := pebble.CalculatePrefixSize(cmd.Context(), pebbleDB.(*pebble.DB), []byte{byte(b)})
		if err != nil {
			return err
		}
		items = append(items, []string{b.String(), bucketItem.Size.String(), fmt.Sprintf("%d", bucketItem.Count)})

		totalSize += bucketItem.Size
		totalCount += bucketItem.Count

		if utils.AnyOf(b, db.StateTrie, db.ContractStorage, db.Class, db.ContractNonce, db.ContractDeploymentHeight) {
			withoutHistorySize += bucketItem.Size
			withHistorySize += bucketItem.Size

			withoutHistoryCount += bucketItem.Count
			withHistoryCount += bucketItem.Count
		}

		if utils.AnyOf(b, db.ContractStorageHistory, db.ContractNonceHistory, db.ContractClassHashHistory) {
			withHistorySize += bucketItem.Size
			withHistoryCount += bucketItem.Count
		}
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"Bucket", "Size", "Count"})
	table.AppendBulk(items)
	table.SetFooter([]string{"Total", totalSize.String(), fmt.Sprintf("%d", totalCount)})
	table.Render()

	tableState := tablewriter.NewWriter(os.Stdout)
	tableState.SetHeader([]string{"State", "Size", "Count"})
	tableState.Append([]string{"Without history", withoutHistorySize.String(), fmt.Sprintf("%d", withoutHistoryCount)})
	tableState.Append([]string{"With history", withHistorySize.String(), fmt.Sprintf("%d", withHistoryCount)})
	tableState.Render()

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
