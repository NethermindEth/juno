package utils

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/spf13/pflag"
)

var (
	ErrUnknownNetwork        = errors.New("unknown network (known: mainnet, goerli, goerli2, integration, custom)")
	ErrNetworkNoFallbackAddr = errors.New("the FallBackSequencerAddress (felt) parameter must be set")
	ErrNetworkNoUnverifRange = errors.New("the unverifiableRangeStart,unverifiableRangeEnd (unint64,uint64) parameters must be correctly 	set")
)

type Network struct {
	Name                string             `json:"name"`
	BaseURL             string             `json:"base_url"`
	ChainID             string             `json:"chain_id"`
	L1ChainID           *big.Int           `json:"l1_chain_id"`
	CoreContractAddress common.Address     `json:"core_contract_address"`
	BlockHashMetaInfo   *blockHashMetaInfo `json:"block_hash_meta_info"`
}

type blockHashMetaInfo struct {
	// The sequencer address to use for blocks that do not have one
	FallBackSequencerAddress *felt.Felt `json:"fallback_sequencer_address"`
	// First block that uses the post-0.7.0 block hash algorithm
	First07Block uint64 `json:"first_07_block"`
	// Range of blocks that are not verifiable
	UnverifiableRange []uint64 `json:"unverifiable_range"`
}

var (
	//nolint:lll
	fallBackSequencerAddress, _ = new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
	// The following are necessary for Cobra and Viper, respectively, to unmarshal log level CLI/config parameters properly.
	_ pflag.Value              = (*Network)(nil)
	_ encoding.TextUnmarshaler = (*Network)(nil)

	// The docs states the addresses for each network: https://docs.starknet.io/documentation/useful_info/
	MAINNET = Network{
		Name:                "mainnet",
		BaseURL:             "https://alpha-mainnet.starknet.io/",
		ChainID:             "SN_MAIN",
		L1ChainID:           big.NewInt(1),
		CoreContractAddress: common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             833,
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	GOERLI = Network{
		Name:    "goerli",
		BaseURL: "https://alpha4.starknet.io/",
		ChainID: "SN_GOERLI",
		//nolint:gomnd
		L1ChainID:           big.NewInt(5),
		CoreContractAddress: common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             47028,
			UnverifiableRange:        []uint64{119802, 148428},
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	GOERLI2 = Network{
		Name:    "goerli2",
		BaseURL: "https://alpha4-2.starknet.io/",
		ChainID: "SN_GOERLI2",
		//nolint:gomnd
		L1ChainID:           big.NewInt(5),
		CoreContractAddress: common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             0,
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	INTEGRATION = Network{
		Name:    "integration",
		BaseURL: "https://external.integration.starknet.io/",
		ChainID: "SN_GOERLI",
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             110511,
			UnverifiableRange:        []uint64{0, 110511},
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
)

func (n Network) String() string {
	return n.Name
}

func (n Network) MarshalYAML() (interface{}, error) {
	return n.String(), nil
}

func (n Network) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + n.String() + `"`), nil
}

// Unmarshals a json string into a Network struct, and requires all fields to be present
func (n *Network) UnmarshalJSON(data []byte) error {
	jsonMap := make(map[string]any)
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		return err
	}
	name, ok := jsonMap["name"].(string)
	if !ok {
		return errors.New("no name field")
	}
	baseURL, ok := jsonMap["base_url"].(string)
	if !ok {
		return errors.New("no base_url field")
	}
	chainID, ok := jsonMap["chain_id"].(string)
	if !ok {
		return errors.New("no chain_id field")
	}
	l1ChainID, ok := jsonMap["l1_chain_id"].(string)
	if !ok {
		return errors.New("no l1_chain_id field")
	}
	l1ChainIDBigInt, ok := new(big.Int).SetString(l1ChainID, 0)
	if !ok {
		return fmt.Errorf("failed to parse l1_chain_id into big.Int")
	}
	coreContractAddressStr, ok := jsonMap["core_contract_address"].(string)
	if !ok {
		return errors.New("no core_contract_address field")
	}

	blockHashMetaInfoData, ok := jsonMap["block_hash_meta_info"]
	if !ok {
		return errors.New("no block_hash_meta_info field")
	}

	var blockHashMetaInfo blockHashMetaInfo
	blockHashMetaInfoJSON, err := json.Marshal(blockHashMetaInfoData)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(blockHashMetaInfoJSON, &blockHashMetaInfo); err != nil {
		return err
	}

	n.Name = name
	n.BaseURL = baseURL
	n.ChainID = chainID
	n.L1ChainID = l1ChainIDBigInt
	n.CoreContractAddress = common.HexToAddress(coreContractAddressStr)
	n.BlockHashMetaInfo = &blockHashMetaInfo
	return nil
}

func (n *Network) Set(s string) error {
	predefinedNetworks := map[string]Network{
		"MAINNET":     MAINNET,
		"mainnet":     MAINNET,
		"GOERLI":      GOERLI,
		"goerli":      GOERLI,
		"GOERLI2":     GOERLI2,
		"goerli2":     GOERLI2,
		"INTEGRATION": INTEGRATION,
		"integration": INTEGRATION,
	}
	if network, ok := predefinedNetworks[s]; ok {
		*n = network
		return nil
	}
	return n.setCustomNetwork(s)
}

// setCustomNetwork tries to unmarshal the json string and performs some basic validation checks
func (n *Network) setCustomNetwork(s string) error {
	*n = Network{}
	if err := n.UnmarshalJSON([]byte(s)); err != nil {
		return errors.New("failed to unmarhsal the json string to a custom Network struct - " + err.Error())
	}

	if !(n.Name == "custom" || n.Name == "CUSTOM") {
		return ErrUnknownNetwork
	}

	if len(n.BlockHashMetaInfo.UnverifiableRange) != 2 {
		return ErrNetworkNoUnverifRange
	}

	return nil
}

func (n Network) Type() string {
	return "Network"
}

func (n *Network) UnmarshalText(text []byte) error {
	return n.Set(string(text))
}

// FeederURL returns URL for read commands
func (n Network) FeederURL() string {
	return n.BaseURL + "feeder_gateway/"
}

// GatewayURL returns URL for write commands
func (n Network) GatewayURL() string {
	return n.BaseURL + "gateway/"
}

func (n Network) DefaultL1ChainID() *big.Int {
	return n.L1ChainID
}

func (n Network) ChainIDFelt() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(n.ChainID))
}

func (n Network) ProtocolID() protocol.ID {
	return protocol.ID(fmt.Sprintf("/starknet/%s", n))
}

func (n Network) MetaInfo() *blockHashMetaInfo {
	return n.BlockHashMetaInfo
}
