package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db/pebble"
	"github.com/NethermindEth/juno/utils"
	"github.com/spf13/cobra"
)

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database related operations",
	Long:  `This command allows you to perform database operations.`,
}

func InitDBCommand() {
	dbCmd.AddCommand(GetDBInfoCmd())
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

func GetDBInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Retrieve database information",
		Long:  `This subcommand retrieves and displays blockchain information stored in the database.`,
		RunE:  dbInfo,
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
