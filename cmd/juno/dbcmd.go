package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
	"github.com/NethermindEth/juno/db/pebblev2"
	"github.com/NethermindEth/juno/encoder"
	"github.com/NethermindEth/juno/migration"
	"github.com/NethermindEth/juno/migration/deprecated" //nolint:staticcheck,nolintlint // deprecated package will be removed later
	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/juno/utils/log"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

const (
	dbRevertToBlockF       = "to-block"
	dbDumpCairo0OutputF    = "output"
	dbDumpCairo0OutputUsg  = "Output file path. If empty, writes to stdout."
)

type DBInfo struct {
	Network         string     `json:"network"`
	SchemaVersion   uint64     `json:"schema_version"`
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
	dbCmd.PersistentFlags().Bool(newStateF, defaultNewState, newStateUsage)
	dbCmd.AddCommand(DBInfoCmd(), DBSizeCmd(), DBRevertCmd(), DBDumpCairo0Cmd())
	return dbCmd
}

func DBDumpCairo0Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump-cairo0",
		Short: "Dump all deprecated Cairo 0 class hashes",
		Long: `Iterates the Class bucket and writes the hex class hash of every
DeprecatedCairoClass (Cairo 0) entry, one per line. Useful for diffing
starknet_getClass responses across RPC versions.`,
		RunE: dbDumpCairo0,
	}
	cmd.Flags().String(dbDumpCairo0OutputF, "", dbDumpCairo0OutputUsg)
	return cmd
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
	var info DBInfo

	// Get the latest block information
	headBlock, err := chain.Head()
	if err != nil {
		return fmt.Errorf("failed to get the latest block information: %v", err)
	}

	stateUpdate, err := chain.StateUpdateByNumber(headBlock.Number)
	if err != nil {
		return fmt.Errorf("failed to get the state update: %v", err)
	}

	// Try new migration system metadata first, fall back to deprecated if not found
	var schemaVersion uint64
	metadata, err := migration.GetSchemaMetadata(database)
	if err == nil {
		// Use the raw uint64 value of the bitset to preserve all migration information
		schemaVersion = uint64(metadata.CurrentVersion)
	} else if errors.Is(err, db.ErrKeyNotFound) {
		// Fall back to deprecated migration system
		deprecatedMetadata, err := deprecated.SchemaMetadata(
			log.NewNopZapLogger(),
			database,
		)
		if err != nil {
			return fmt.Errorf("failed to get deprecated schema metadata: %v", err)
		}
		schemaVersion = deprecatedMetadata.Version
	} else {
		return fmt.Errorf("failed to get schema metadata: %v", err)
	}

	newState, err := cmd.Flags().GetBool(newStateF)
	if err != nil {
		return err
	}
	trieBackend := core.DeprecatedTrieBackend
	if newState {
		trieBackend = core.TrieBackend
	}

	info.SchemaVersion = schemaVersion
	info.Network = getNetwork(headBlock, stateUpdate.StateDiff, trieBackend)
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

	newState, err := cmd.Flags().GetBool(newStateF)
	if err != nil {
		return err
	}

	database, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	for {
		chain := blockchain.New(
			database,
			nil,
			blockchain.WithNewState(newState),
		)
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

//nolint:funlen // todo(rdr): refactor this function
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
		totalSize  db.DataSize
		totalCount uint

		withHistorySize    db.DataSize
		withoutHistorySize db.DataSize

		withHistoryCount    uint
		withoutHistoryCount uint
	)

	historyBuckets := []db.Bucket{
		db.DeprecatedContractStorageHistory,
		db.DeprecatedContractNonceHistory,
		db.DeprecatedContractClassHashHistory,
		db.ContractStorageHistory,
		db.ContractNonceHistory,
		db.ContractClassHashHistory,
	}

	buckets := db.BucketValues()
	items := make([][]string, 0, len(buckets)+3)
	for _, b := range buckets {
		fmt.Fprintf(cmd.OutOrStdout(), "Calculating size of %s, remaining buckets: %d\n", b, len(db.BucketValues())-int(b)-1)
		bucketItem, err := pebblev2.CalculatePrefixSize(
			cmd.Context(),
			pebbleDB.(*pebblev2.DB),
			[]byte{byte(b)},
			true,
		)
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

		if utils.AnyOf(b, historyBuckets...) {
			withHistorySize += bucketItem.Size
			withHistoryCount += bucketItem.Count
		}
	}

	// check if there is any data left in the db
	lastBucket := buckets[len(buckets)-1]
	fmt.Fprintln(cmd.OutOrStdout(), "Calculating remaining data in the db")
	lastBucketItem, err := pebblev2.CalculatePrefixSize(
		cmd.Context(),
		pebbleDB.(*pebblev2.DB),
		[]byte{byte(lastBucket + 1)},
		false,
	)
	if err != nil {
		return err
	}

	if lastBucketItem.Count > 0 {
		items = append(items, []string{"Unknown", lastBucketItem.Size.String(), fmt.Sprintf("%d", lastBucketItem.Count)})
		totalSize += lastBucketItem.Size
		totalCount += lastBucketItem.Count
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header([]string{"Bucket", "Size", "Count"})
	err = table.Bulk(items)
	if err != nil {
		return err
	}
	table.Footer([]string{"Total", totalSize.String(), fmt.Sprintf("%d", totalCount)})
	err = table.Render()
	if err != nil {
		return err
	}

	tableState := tablewriter.NewWriter(os.Stdout)
	tableState.Header([]string{"State", "Size", "Count"})
	err = tableState.Append(
		[]string{"Without history", withoutHistorySize.String(), fmt.Sprintf("%d", withoutHistoryCount)},
	)
	if err != nil {
		return err
	}
	err = tableState.Append([]string{"With history", withHistorySize.String(), fmt.Sprintf("%d", withHistoryCount)})
	if err != nil {
		return err
	}
	err = tableState.Render()
	if err != nil {
		return err
	}

	return nil
}

func getNetwork(
	head *core.Block,
	stateDiff *core.StateDiff,
	trieBackend core.TempTrieBackend,
) string {
	networks := []*networks.Network{
		&networks.Mainnet,
		&networks.Sepolia,
		&networks.Goerli,
		&networks.Goerli2,
		&networks.Integration,
		&networks.SepoliaIntegration,
	}

	for _, network := range networks {
		if _, err := core.VerifyBlockHash(head, network, stateDiff, trieBackend); err == nil {
			return network.Name
		}
	}

	return "unknown"
}

func dbDumpCairo0(cmd *cobra.Command, args []string) error {
	dbPath, err := cmd.Flags().GetString(dbPathF)
	if err != nil {
		return err
	}
	outPath, err := cmd.Flags().GetString(dbDumpCairo0OutputF)
	if err != nil {
		return err
	}

	database, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	var out *bufio.Writer
	if outPath == "" {
		out = bufio.NewWriter(cmd.OutOrStdout())
	} else {
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer f.Close()
		out = bufio.NewWriter(f)
	}
	defer out.Flush()

	classPrefix := db.Class.Key()
	it, err := database.NewIterator(classPrefix, false)
	if err != nil {
		return fmt.Errorf("failed to open iterator: %w", err)
	}
	defer it.Close()

	var total, cairo0Count uint64
	for it.Seek(classPrefix); it.Valid(); it.Next() {
		k := it.Key()
		if !bytes.HasPrefix(k, classPrefix) {
			break
		}
		hashBytes := k[len(classPrefix):]
		if len(hashBytes) != felt.Bytes {
			continue
		}
		total++

		v, err := it.Value()
		if err != nil {
			return fmt.Errorf("failed to read value: %w", err)
		}

		var dc core.DeclaredClassDefinition
		if err := encoder.Unmarshal(v, &dc); err != nil {
			return fmt.Errorf("failed to unmarshal class: %w", err)
		}

		if _, ok := dc.Class.(*core.DeprecatedCairoClass); !ok {
			continue
		}

		classHash := felt.FromBytes[felt.Felt](hashBytes)
		if _, err := fmt.Fprintln(out, classHash.String()); err != nil {
			return fmt.Errorf("failed to write hash: %w", err)
		}
		cairo0Count++
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Scanned %d classes, dumped %d deprecated Cairo 0 hashes\n", total, cairo0Count)
	return nil
}

func openDB(path string) (db.KeyValueStore, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, errors.New("database path does not exist")
	}

	database, err := pebblev2.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return database, nil
}
