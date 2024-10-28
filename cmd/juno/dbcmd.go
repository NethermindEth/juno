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
	pebbleDB "github.com/cockroachdb/pebble"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	dbRevertToBlockF = "to-block"
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
	dbCmd.AddCommand(DBInfoCmd(), DBSizeCmd(), DBRevertCmd(), DBIterateTrieCmd(), DBIterateTrieTrie(), DBIterateTrieExperiment())
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

func DBRevertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revert",
		Short: "Revert current head to given position",
		Long:  `This subcommand revert all data related to all blocks till given so it becomes new head.`,
		RunE:  dbRevert,
	}
	cmd.Flags().Uint64(dbRevertToBlockF, 0, "New head (this block won't be reverted)")

	return cmd
}

func DBIterateTrieCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iter-trie",
		Short: "Iterate over the trie",
		RunE:  dbIterateTrieDB,
	}
	cmd.Flags().Uint64("limit", 100, "Limit the number of items to iterate over")

	return cmd
}

func DBIterateTrieTrie() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iter-trie-trie",
		Short: "Iterate over the trie",
		RunE:  dbIterateTrieTrie,
	}
	cmd.Flags().Uint64("limit", 100, "Limit the number of items to iterate over")

	return cmd
}

func DBIterateTrieExperiment() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iter-trie-exp",
		Short: "Iterate over the trie",
		RunE:  dbIterateTrieExperiment,
	}

	return cmd
}

func dbInfo(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	database, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

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

func dbRevert(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	revertToBlock, err := cmd.Flags().GetUint64(dbRevertToBlockF)
	if err != nil {
		return err
	}

	if revertToBlock == 0 {
		return fmt.Errorf("--%v cannot be 0", dbRevertToBlockF)
	}

	database, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	for {
		chain := blockchain.New(database, nil)
		head, err := chain.Head()
		if err != nil {
			return fmt.Errorf("failed to get the latest block information: %v", err)
		}

		if head.Number == revertToBlock {
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully reverted all blocks to %d\n", revertToBlock)
			break
		}

		err = chain.RevertHead()
		if err != nil {
			return fmt.Errorf("failed to revert head at block %d: %v", head.Number, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Reverted head at block %d\n", head.Number)
	}

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

	pebbleDB, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer pebbleDB.Close()

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

func dbIterateTrieDB(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	limit, err := cmd.Flags().GetUint64("limit")
	if err != nil {
		return err
	}

	pDB, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer pDB.Close()

	lowerBound := []byte{byte(db.StateTrie), 0xfb}
	it, err := pDB.Impl().(*pebbleDB.DB).NewIter(&pebbleDB.IterOptions{
		LowerBound: lowerBound,
		UpperBound: upperBound([]byte{byte(db.StateTrie)}),
	})

	count := uint64(0)
	start := time.Now()
	for it.First(); it.Valid(); it.Next() {
		if count >= limit {
			break
		}

		key := it.Key()

		key = key[1:] // omit prefix

		fmt.Fprintf(cmd.OutOrStdout(), "key: %x\n", key)

		count++
	}

	elapsed := time.Since(start)
	fmt.Fprintf(cmd.OutOrStdout(), "count: %d, elapsed: %s\n", count, elapsed)

	return nil
}

func dbIterateTrieTrie(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	pDB, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer pDB.Close()

	bc := blockchain.New(pDB, nil)
	s, _, err := bc.HeadState()
	if err != nil {
		return err
	}

	state := s.(*core.State)

	stateTrie, _, err := state.StorageTrie()
	if err != nil {
		return err
	}

	count := 0
	start := time.Now()
	_, err = stateTrie.Iterate(nil, func(key, value *felt.Felt) (bool, error) {
		count++
		fmt.Fprintf(cmd.OutOrStdout(), "key: %s\n", key)
		return true, nil
	})
	if err != nil {
		return err
	}

	elapsed := time.Since(start)
	fmt.Fprintf(cmd.OutOrStdout(), "count: %d, elapsed: %s\n", count, elapsed)

	return nil
}

func dbIterateTrieExperiment(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}

	pDB, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer pDB.Close()

	bc := blockchain.New(pDB, nil)
	s, _, err := bc.HeadState()
	if err != nil {
		return err
	}

	state := s.(*core.State)

	stateTrie, _, err := state.StorageTrie()
	if err != nil {
		return err
	}

	count := 0
	start := time.Now()
	leafIt, err := stateTrie.NewLeafIterator()
	if err != nil {
		return err
	}

	for leafIt.Next() {
		if !leafIt.Valid() {
			break
		}

		count++
		fmt.Fprintf(cmd.OutOrStdout(), "key: %x\n", leafIt.Key())
	}

	elapsed := time.Since(start)
	fmt.Fprintf(cmd.OutOrStdout(), "count: %d, elapsed: %s\n", count, elapsed)

	return nil
}

func getNetwork(head *core.Block, stateDiff *core.StateDiff) string {
	networks := []*utils.Network{
		&utils.Mainnet,
		&utils.Sepolia,
		&utils.Goerli,
		&utils.Goerli2,
		&utils.Integration,
		&utils.SepoliaIntegration,
	}

	for _, network := range networks {
		if _, err := core.VerifyBlockHash(head, network, stateDiff); err == nil {
			return network.Name
		}
	}

	return "unknown"
}

func upperBound(prefix []byte) []byte {
	var ub []byte

	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] == byte(0xff) {
			continue
		}
		ub = make([]byte, i+1)
		copy(ub, prefix)
		ub[i]++
		return ub
	}

	return nil
}

func openDB(path string) (db.DB, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("database path does not exist")
	}

	database, err := pebble.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return database, nil
}
