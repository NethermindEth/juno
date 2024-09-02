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

type DBInfo struct {
	Network         string     `json:"network"`
	ChainHeight     uint64     `json:"chain_height"`
	LatestBlockHash *felt.Felt `json:"latest_block_hash"`
	LatestStateRoot *felt.Felt `json:"latest_state_root"`
	L1Height        uint64     `json:"l1_height"`
	L1BlockHash     *felt.Felt `json:"l1_block_hash"`
	L1StateRoot     *felt.Felt `json:"l1_state_root"`
}

func DBCmd(defaultDBPath string) *cobra.Command {
	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Database related operations",
		Long:  `This command allows you to perform database operations.`,
	}

	dbCmd.PersistentFlags().String(dbPathF, defaultDBPath, dbPathUsage)
	dbCmd.AddCommand(DBInfoCmd(), DBSizeCmd())
	return dbCmd
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
		fmt.Fprintln(cmd.OutOrStdout(), "Database path does not exist")
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
	if err != nil {
		return fmt.Errorf("failed to get the latest block information: %v", err)
	}

	stateUpdate, err := chain.StateUpdateByNumber(headBlock.Number)
	if err != nil {
		return fmt.Errorf("failed to get the state update: %v", err)
	}

	info.Network = getNetwork(headBlock, stateUpdate.StateDiff)
	info.ChainHeight = headBlock.Number
	info.LatestBlockHash = headBlock.Hash
	info.LatestStateRoot = headBlock.GlobalStateRoot

	// Get the latest L1 block information
	l1Head, err := chain.L1Head()
	if err == nil {
		info.L1Height = l1Head.BlockNumber
		info.L1BlockHash = l1Head.BlockHash
		info.L1StateRoot = l1Head.StateRoot
	} else {
		fmt.Printf("Failed to get the latest L1 block information: %v\n", err)
	}

	jsonData, err := json.MarshalIndent(info, "", "")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), string(jsonData))

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
		fmt.Fprintln(cmd.OutOrStdout(), "Database path does not exist")
		return nil
	}

	pebbleDB, err := pebble.New(dbPath)
	if err != nil {
		return err
	}

	var (
		totalSize  utils.DataSize
		totalCount uint

		withHistorySize    utils.DataSize
		withoutHistorySize utils.DataSize

		withHistoryCount    uint
		withoutHistoryCount uint

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

func getNetwork(head *core.Block, stateDiff *core.StateDiff) string {
	networks := []*utils.Network{
		&utils.Mainnet,
		&utils.Sepolia,
		&utils.SepoliaIntegration,
	}

	for _, network := range networks {
		if _, err := core.VerifyBlockHash(head, network, stateDiff); err == nil {
			return network.Name
		}
	}

	return "unknown"
}
