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

type Network int

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*Network)(nil)
	_ encoding.TextUnmarshaler = (*Network)(nil)
)

const (
	Mainnet Network = iota + 1
	Goerli
	Goerli2
	Integration
	Sepolia
	SepoliaIntegration
)

func (n Network) String() string {
	switch n {
	case Mainnet:
		return "mainnet"
	case Goerli:
		return "goerli"
	case Goerli2:
		return "goerli2"
	case Integration:
		return "integration"
	case Sepolia:
		return "sepolia"
	case SepoliaIntegration:
		return "sepolia-integration"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

func (n Network) MarshalYAML() (interface{}, error) {
	return n.String(), nil
}

func (n *Network) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + n.String() + `"`), nil
}

func (n *Network) Set(s string) error {
	switch s {
	case "MAINNET", "mainnet":
		*n = Mainnet
	case "GOERLI", "goerli":
		*n = Goerli
	case "GOERLI2", "goerli2":
		*n = Goerli2
	case "INTEGRATION", "integration":
		*n = Integration
	case "SEPOLIA", "sepolia":
		*n = Sepolia
	case "SEPOLIA_INTEGRATION", "sepolia-integration":
		*n = SepoliaIntegration
	default:
		return ErrUnknownNetwork
	}
	return nil
}

func (n *Network) Type() string {
	return "Network"
}

func (n *Network) UnmarshalText(text []byte) error {
	return n.Set(string(text))
}

// baseURL returns the base URL without endpoint
func (n Network) baseURL() string {
	switch n {
	case Goerli:
		return "https://alpha4.starknet.io/"
	case Mainnet:
		return "https://alpha-mainnet.starknet.io/"
	case Goerli2:
		return "https://alpha4-2.starknet.io/"
	case Integration:
		return "https://external.integration.starknet.io/"
	case Sepolia:
		return "https://alpha-sepolia.starknet.io/"
	case SepoliaIntegration:
		return "https://integration-sepolia.starknet.io/"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

// FeederURL returns URL for read commands
func (n Network) FeederURL() string {
	return n.baseURL() + "feeder_gateway/"
}

// GatewayURL returns URL for write commands
func (n Network) GatewayURL() string {
	return n.baseURL() + "gateway/"
}

func (n Network) ChainIDString() string {
	switch n {
	case Goerli, Integration:
		return "SN_GOERLI"
	case Mainnet:
		return "SN_MAIN"
	case Goerli2:
		return "SN_GOERLI2"
	case Sepolia:
		return "SN_SEPOLIA"
	case SepoliaIntegration:
		return "SN_INTEGRATION_SEPOLIA"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

func (n Network) DefaultL1ChainID() *big.Int {
	var chainID int64
	switch n {
	case Mainnet:
		chainID = 1
	case Goerli, Goerli2, Integration:
		chainID = 5
	case Sepolia, SepoliaIntegration:
		chainID = 11155111
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
	return big.NewInt(chainID)
}

func (n Network) CoreContractAddress() (common.Address, error) {
	var address common.Address
	// The docs states the addresses for each network: https://docs.starknet.io/documentation/useful_info/
	switch n {
	case Mainnet:
		address = common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4")
	case Goerli:
		address = common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e")
	case Goerli2:
		address = common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c")
	case Integration:
		return common.Address{}, errors.New("l1 contract is not available on the integration network")
	case Sepolia:
		return common.HexToAddress("0xE2Bb56ee936fd6433DC0F6e7e3b8365C906AA057"), nil
	case SepoliaIntegration:
		return common.HexToAddress("0x4737c0c1B4D5b1A687B42610DdabEE781152359c"), nil
	default:
		// Should not happen.
		return common.Address{}, ErrUnknownNetwork
	}
	return address, nil
}

func (n Network) ChainID() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(n.ChainIDString()))
}

func (n Network) ProtocolID() protocol.ID {
	return protocol.ID(fmt.Sprintf("/starknet/%s", n))
}
