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

var ErrUnknownNetwork = errors.New("unknown network (known: mainnet, goerli, goerli2, integration)")

type Network struct {
	name                string
	baseURL             string
	chainID             string
	l1ChainID           *big.Int
	coreContractAddress common.Address
}

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*Network)(nil)
	_ encoding.TextUnmarshaler = (*Network)(nil)

	// The docs states the addresses for each network: https://docs.starknet.io/documentation/useful_info/
	MAINNET = Network{
		name:                "mainnet",
		baseURL:             "https://alpha-mainnet.starknet.io/",
		chainID:             "SN_MAIN",
		l1ChainID:           big.NewInt(1),
		coreContractAddress: common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
	}
	GOERLI = Network{
		name:                "goerli",
		baseURL:             "https://alpha4.starknet.io/",
		chainID:             "SN_GOERLI",
		l1ChainID:           big.NewInt(5),
		coreContractAddress: common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
	}
	GOERLI2 = Network{
		name:                "goerli2",
		baseURL:             "https://alpha4-2.starknet.io/",
		chainID:             "SN_GOERLI2",
		l1ChainID:           big.NewInt(5),
		coreContractAddress: common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
	}
	INTEGRATION = Network{
		name:    "integration",
		baseURL: "https://external.integration.starknet.io/",
		chainID: "SN_GOERLI",
	}
)

func (n Network) String() string {
	return n.name
}

func (n Network) MarshalYAML() (interface{}, error) {
	return n.String(), nil
}

func (n Network) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + n.String() + `"`), nil
}

func (n *Network) Set(s string) error {
	switch s {
	case "MAINNET", "mainnet":
		*n = MAINNET
	case "GOERLI", "goerli":
		*n = GOERLI
	case "GOERLI2", "goerli2":
		*n = GOERLI2
	case "INTEGRATION", "integration":
		*n = INTEGRATION
	default:
		return ErrUnknownNetwork
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
	return n.baseURL + "feeder_gateway/"
}

// GatewayURL returns URL for write commands
func (n Network) GatewayURL() string {
	return n.baseURL + "gateway/"
}

func (n Network) ChainIDString() string {
	return n.chainID
}

func (n Network) DefaultL1ChainID() *big.Int {
	return n.l1ChainID
}

func (n Network) CoreContractAddress() (common.Address, error) {
	if n.l1ChainID == nil {
		return common.Address{}, errors.New("l1 contract is not available on this network")
	}
	return n.coreContractAddress, nil
}

func (n Network) ChainID() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(n.ChainIDString()))
}

func (n Network) ProtocolID() protocol.ID {
	return protocol.ID(fmt.Sprintf("/starknet/%s", n))
}
