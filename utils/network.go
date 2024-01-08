package utils

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/validator"
	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/spf13/pflag"
)

var (
	ErrUnknownNetwork              = errors.New("unknown network (known: mainnet, goerli, goerli2, integration)")
	ErrInvalidCustomNetworkJSONStr = errors.New("invalid custom-network json string")
)

type Network struct {
	Name                string             `json:"name" validate:"required"`
	FeederURL           string             `json:"feeder_url" validate:"required"`
	GatewayURL          string             `json:"gateway_url" validate:"required"`
	L1ChainID           *big.Int           `json:"l1_chain_id" validate:"required"`
	L2ChainID           string             `json:"l2_chain_id" validate:"required"`
	CoreContractAddress common.Address     `json:"core_contract_address" validate:"required"`
	BlockHashMetaInfo   *blockHashMetaInfo `json:"block_hash_meta_info"`
}

type blockHashMetaInfo struct {
	// The sequencer address to use for blocks that do not have one
	FallBackSequencerAddress *felt.Felt `json:"fallback_sequencer_address" validate:"required"`
	// First block that uses the post-0.7.0 block hash algorithm
	First07Block uint64 `json:"first_07_block" validate:"required"`
	// Range of blocks that are not verifiable
	UnverifiableRange []uint64 `json:"unverifiable_range" validate:"required"`
}

var (
	fallBackSequencerAddressMainnet, _ = new(felt.Felt).SetString("0x021f4b90b0377c82bf330b7b5295820769e72d79d8acd0effa0ebde6e9988bc5")
	fallBackSequencerAddress, _        = new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
	// The following are necessary for Cobra and Viper, respectively, to unmarshal log level CLI/config parameters properly.
	_ pflag.Value              = (*Network)(nil)
	_ encoding.TextUnmarshaler = (*Network)(nil)

	// The docs states the addresses for each network: https://docs.starknet.io/documentation/useful_info/
	Mainnet = Network{
		Name:                "mainnet",
		FeederURL:           "https://alpha-mainnet.starknet.io/feeder_gateway/",
		GatewayURL:          "https://alpha-mainnet.starknet.io/gateway/",
		L2ChainID:           "SN_MAIN",
		L1ChainID:           big.NewInt(1),
		CoreContractAddress: common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             833,
			FallBackSequencerAddress: fallBackSequencerAddressMainnet,
		},
	}
	Goerli = Network{
		Name:       "goerli",
		FeederURL:  "https://alpha4.starknet.io/feeder_gateway/",
		GatewayURL: "https://alpha4.starknet.io/gateway/",
		L2ChainID:  "SN_GOERLI",
		//nolint:gomnd
		L1ChainID:           big.NewInt(5),
		CoreContractAddress: common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             47028,
			UnverifiableRange:        []uint64{119802, 148428},
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	Goerli2 = Network{
		Name:       "goerli2",
		FeederURL:  "https://alpha4-2.starknet.io/feeder_gateway/",
		GatewayURL: "https://alpha4-2.starknet.io/gateway/",
		L2ChainID:  "SN_GOERLI2",
		//nolint:gomnd
		L1ChainID:           big.NewInt(5),
		CoreContractAddress: common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             0,
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	Integration = Network{
		Name:       "integration",
		FeederURL:  "https://external.integration.starknet.io/feeder_gateway/",
		GatewayURL: "https://external.integration.starknet.io/gateway/",
		L2ChainID:  "SN_GOERLI",
		//nolint:gomnd
		L1ChainID:           big.NewInt(5),
		CoreContractAddress: common.HexToAddress("0xd5c325D183C592C94998000C5e0EED9e6655c020"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             110511,
			UnverifiableRange:        []uint64{0, 110511},
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	Sepolia = Network{
		Name:       "sepolia",
		FeederURL:  "https://alpha-sepolia.starknet.io/feeder_gateway/",
		GatewayURL: "https://alpha-sepolia.starknet.io/gateway/",
		L2ChainID:  "SN_SEPOLIA",
		//nolint:gomnd
		L1ChainID:           big.NewInt(11155111),
		CoreContractAddress: common.HexToAddress("0xE2Bb56ee936fd6433DC0F6e7e3b8365C906AA057"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             0,
			FallBackSequencerAddress: fallBackSequencerAddress,
		},
	}
	SepoliaIntegration = Network{
		Name:       "sepolia-integration",
		FeederURL:  "https://integration-sepolia.starknet.io/feeder_gateway/",
		GatewayURL: "https://integration-sepolia.starknet.io/gateway/",
		L2ChainID:  "SN_INTEGRATION_SEPOLIA",
		//nolint:gomnd
		L1ChainID:           big.NewInt(11155111),
		CoreContractAddress: common.HexToAddress("0x4737c0c1B4D5b1A687B42610DdabEE781152359c"),
		BlockHashMetaInfo: &blockHashMetaInfo{
			First07Block:             0,
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

func (n *Network) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + n.String() + `"`), nil
}

func (n *Network) Set(s string) error {
	predefinedNetworks := map[string]Network{
		"MAINNET":             Mainnet,
		"mainnet":             Mainnet,
		"GOERLI":              Goerli,
		"goerli":              Goerli,
		"GOERLI2":             Goerli2,
		"goerli2":             Goerli2,
		"INTEGRATION":         Integration,
		"integration":         Integration,
		"SEPOLIA":             Sepolia,
		"sepolia":             Sepolia,
		"SEPOLIA-INTEGRATION": SepoliaIntegration,
		"sepolia-integration": SepoliaIntegration,
	}

	if network, ok := predefinedNetworks[s]; ok {
		*n = network
		return nil
	}
	return ErrUnknownNetwork
}

func (n *Network) SetCustomNetwork(s string) error {
	*n = Network{}
	if err := n.UnmarshalJSON([]byte(s)); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidCustomNetworkJSONStr, err)
	}
	return n.Validate()
}

func (n *Network) Validate() error {
	if !(n.Name == "CUSTOM" || n.Name == "custom") {
		return ErrUnknownNetwork
	}

	validate := validator.Validator()
	return validate.Struct(n)
}

func (n *Network) UnmarshalJSON(data []byte) error {
	type Alias Network
	aux := &struct {
		CoreContractAddress string `json:"core_contract_address"`
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	n.CoreContractAddress = common.HexToAddress(aux.CoreContractAddress)
	return nil
}

func (n *Network) Type() string {
	return "Network"
}

func (n *Network) UnmarshalText(text []byte) error {
	return n.Set(string(text))
}

func (n Network) ChainIDFelt() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(n.L2ChainID))
}

func (n Network) ProtocolID() protocol.ID {
	return protocol.ID(fmt.Sprintf("/starknet/%s", n))
}
