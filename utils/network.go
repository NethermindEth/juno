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
	ErrUnknownNetwork             = errors.New("unknown network (known: mainnet, goerli, goerli2, integration)")
	ErrInvalidCoreContractAddress = errors.New("invalid core contract address")
)

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*NetworkKnown)(nil)
	_ encoding.TextUnmarshaler = (*NetworkKnown)(nil)
	_ Network                  = (*NetworkKnown)(nil)
	_ Network                  = (*NetworkCustom)(nil)
	_ Network                  = Mainnet
)

type Network interface {
	String() string
	MarshalYAML() (interface{}, error)
	MarshalJSON() ([]byte, error)
	// Set(s string) error
	Type() string
	// UnmarshalText(text []byte) error
	FeederURL() string
	GatewayURL() string
	ChainIDString() string
	DefaultL1ChainID() *big.Int
	CoreContractAddress() (common.Address, error)
	ChainID() *felt.Felt
	ProtocolID() protocol.ID
}

type NetworkKnown int

const (
	Mainnet NetworkKnown = iota + 1
	Goerli
	Goerli2
	Integration
	Sepolia
	SepoliaIntegration
	Custom
)

func (n NetworkKnown) String() string {
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
	case Custom:
		return "custom"
	default:
		// Should not happen.
		panic(ErrUnknownNetwork)
	}
}

func (n NetworkKnown) MarshalYAML() (interface{}, error) {
	return n.String(), nil
}

func (n NetworkKnown) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + n.String() + `"`), nil
}

func (n *NetworkKnown) Set(s string) error {
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

func (n NetworkKnown) Type() string {
	return "NetworkKnown"
}

func (n *NetworkKnown) UnmarshalText(text []byte) error {
	return n.Set(string(text))
}

// baseURL returns the base URL without endpoint
func (n NetworkKnown) baseURL() string {
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
func (n NetworkKnown) FeederURL() string {
	return n.baseURL() + "feeder_gateway/"
}

// GatewayURL returns URL for write commands
func (n NetworkKnown) GatewayURL() string {
	return n.baseURL() + "gateway/"
}

func (n NetworkKnown) ChainIDString() string {
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

func (n NetworkKnown) DefaultL1ChainID() *big.Int {
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

func (n NetworkKnown) CoreContractAddress() (common.Address, error) {
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
		address = common.HexToAddress("0xd5c325D183C592C94998000C5e0EED9e6655c020")
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

func (n NetworkKnown) ChainID() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(n.ChainIDString()))
}

func (n NetworkKnown) ProtocolID() protocol.ID {
	return protocol.ID(fmt.Sprintf("/starknet/%s", n))
}

type NetworkCustom struct {
	FeederURLVal           string          `yaml:"feeder_url" validate:"required"`
	GatewayURLVal          string          `yaml:"gateway_url" validate:"required"`
	ChainIDVal             string          `yaml:"chain_id" validate:"required"`
	L1ChainIDVal           *big.Int        `yaml:"l1_chain_id" validate:"required"`
	ProtocolIDVal          int             `yaml:"protocol_id" validate:"required"`
	CoreContractAddressVal *common.Address `yaml:"core_contract_address" validate:"required"`
}

func (cn *NetworkCustom) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type Alias NetworkCustom
	aux := &struct {
		CoreContractAddress string `yaml:"core_contract_address"`
		*Alias
	}{
		Alias: (*Alias)(cn),
	}
	if err := unmarshal(aux); err != nil {
		return err
	}
	if !common.IsHexAddress(aux.CoreContractAddress) {
		return errors.New("invalid core contract address")
	}
	coreContractAddress := common.HexToAddress(aux.CoreContractAddress)
	cn.CoreContractAddressVal = &coreContractAddress
	return nil
}

func (cn NetworkCustom) Validate() error {
	validate := validator.Validator()
	if cn.CoreContractAddressVal.String() == "0x0000000000000000000000000000000000000000" {
		return ErrInvalidCoreContractAddress
	}
	return validate.Struct(cn)
}

func (cn NetworkCustom) String() string {
	return "custom"
}

func (cn NetworkCustom) MarshalYAML() (interface{}, error) {
	return cn.String(), nil
}

func (cn NetworkCustom) MarshalJSON() ([]byte, error) {
	return json.RawMessage(`"` + cn.String() + `"`), nil
}

func (cn NetworkCustom) Set(s string) error {
	return errors.New("custom networks cannot be set")
}

func (cn NetworkCustom) Type() string {
	return "NetworkCustom"
}

func (cn NetworkCustom) UnmarshalText(text []byte) error {
	return cn.Set(string(text))
}

func (cn NetworkCustom) FeederURL() string {
	return cn.FeederURLVal
}

func (cn NetworkCustom) GatewayURL() string {
	return cn.GatewayURLVal
}

func (cn NetworkCustom) ChainIDString() string {
	return cn.ChainIDVal
}

func (cn NetworkCustom) DefaultL1ChainID() *big.Int {
	return cn.L1ChainIDVal
}

func (cn NetworkCustom) CoreContractAddress() (common.Address, error) {
	if cn.CoreContractAddressVal == nil {
		return common.Address{}, errors.New("core contract address is nil")
	}
	return *cn.CoreContractAddressVal, nil
}

func (cn NetworkCustom) ChainID() *felt.Felt {
	return new(felt.Felt).SetBytes([]byte(cn.ChainIDVal))
}

func (cn NetworkCustom) ProtocolID() protocol.ID {
	return protocol.ID(fmt.Sprintf("/starknet/%q", cn.ProtocolIDVal))
}
