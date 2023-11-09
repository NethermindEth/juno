package utils

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/spf13/pflag"
)

var (
	ErrUnknownNetwork              = errors.New("unknown network (known: mainnet, goerli, goerli2, integration, custom)")
	ErrNetworkNoFallbackAddr       = errors.New("the FallBackSequencerAddress (felt) parameter must be set")
	ErrNetworkNoFirst07Block       = errors.New("the First07Block (uint64) parameter must be set")
	ErrNetworkNoUnverifRange       = errors.New("the unverifiableRangeStart,unverifiableRangeEnd (unint64,uint64) parameters must be set")
	ErrNetworkSetFallbackAddr      = errors.New("failed to set FallBackSequencerAddress as a felt")
	ErrNetworkSetFirst07Block      = errors.New("failed to set First07Block as a uint64")
	ErrNetworkSetUnverifRangeStart = errors.New("failed to set UnverifiableRangeStart as a uint64")
	ErrNetworkSetUnverifRangeEnd   = errors.New("failed to set UnverifiableRangeEnd as a uint64")
	ErrNetworkParamsNotSet         = errors.New("all parameters must be specified for a custom network")
)

type Network struct {
	name                string
	baseURL             string
	chainID             string
	l1ChainID           *big.Int
	coreContractAddress common.Address
	blockHashMetaInfo   *blockHashMetaInfo
}

type blockHashMetaInfo struct {
	FallBackSequencerAddress *felt.Felt // The sequencer address to use for blocks that do not have one
	First07Block             uint64     // First block that uses the post-0.7.0 block hash algorithm
	UnverifiableRange        []uint64   // Range of blocks that are not verifiable
}

// The following are necessary for Cobra and Viper, respectively, to unmarshal log level
// CLI/config parameters properly.
var (
	//nolint:lll
	fallBackSequencerAddress, _                          = new(felt.Felt).SetString("0x046a89ae102987331d369645031b49c27738ed096f2789c24449966da4c6de6b")
	_                           pflag.Value              = (*Network)(nil)
	_                           encoding.TextUnmarshaler = (*Network)(nil)

	// The docs states the addresses for each network: https://docs.starknet.io/documentation/useful_info/
	MAINNET = Network{
		name:                "mainnet",
		baseURL:             "https://alpha-mainnet.starknet.io/",
		chainID:             "SN_MAIN",
		l1ChainID:           big.NewInt(1),
		coreContractAddress: common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		blockHashMetaInfo:   &MetaInfoMAINNET,
	}
	GOERLI = Network{
		name:    "goerli",
		baseURL: "https://alpha4.starknet.io/",
		chainID: "SN_GOERLI",
		//nolint:gomnd
		l1ChainID:           big.NewInt(5),
		coreContractAddress: common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		blockHashMetaInfo:   &MetaInfoGOERLI,
	}
	GOERLI2 = Network{
		name:    "goerli2",
		baseURL: "https://alpha4-2.starknet.io/",
		chainID: "SN_GOERLI2",
		//nolint:gomnd
		l1ChainID:           big.NewInt(5),
		coreContractAddress: common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
		blockHashMetaInfo:   &MetaInfoGOERLI2,
	}
	INTEGRATION = Network{
		name:              "integration",
		baseURL:           "https://external.integration.starknet.io/",
		chainID:           "SN_GOERLI",
		blockHashMetaInfo: &MetaInfoINTEGRATION,
	}
	MetaInfoMAINNET = blockHashMetaInfo{
		First07Block:             833,
		FallBackSequencerAddress: fallBackSequencerAddress,
	}
	MetaInfoGOERLI = blockHashMetaInfo{
		First07Block:             47028,
		UnverifiableRange:        []uint64{119802, 148428},
		FallBackSequencerAddress: fallBackSequencerAddress,
	}
	MetaInfoGOERLI2 = blockHashMetaInfo{
		First07Block:             0,
		FallBackSequencerAddress: fallBackSequencerAddress,
	}
	MetaInfoINTEGRATION = blockHashMetaInfo{
		First07Block:             110511,
		UnverifiableRange:        []uint64{0, 110511},
		FallBackSequencerAddress: fallBackSequencerAddress,
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

func (n *Network) setCustomNetwork(s string) error {
	*n = Network{}
	elems := strings.Split(s, ",")

	if !(elems[0] == "custom" || elems[0] == "CUSTOM") {
		return ErrUnknownNetwork
	}

	if len(elems) != 9 { /* number of required fields in Network struct */ //nolint:gomnd
		return ErrNetworkParamsNotSet
	}

	n.name = elems[0]
	n.baseURL = elems[1]
	n.chainID = elems[2]

	if elems[3] != "" {
		l1ChainID, success := new(big.Int).SetString(elems[3], 10) //nolint:gomnd
		if !success {
			return errors.New("L1 Chain ID must be an integer (base 10)")
		}
		n.l1ChainID = l1ChainID
	}

	if elems[4] != "" {
		n.coreContractAddress = common.HexToAddress(elems[4])
	}
	requiredFields := []struct {
		value string
		err   error
	}{
		{elems[5], ErrNetworkNoFallbackAddr},
		{elems[6], ErrNetworkNoFirst07Block},
		{elems[7], ErrNetworkNoUnverifRange},
		{elems[8], ErrNetworkNoUnverifRange},
	}

	for _, field := range requiredFields {
		if field.value == "" {
			return field.err
		}
	}

	n.blockHashMetaInfo = &blockHashMetaInfo{}

	var err error
	if n.blockHashMetaInfo.FallBackSequencerAddress, err = new(felt.Felt).SetString(elems[5]); err != nil {
		return ErrNetworkSetFallbackAddr
	}
	if n.blockHashMetaInfo.First07Block, err = strconv.ParseUint(elems[6], 10, 64); err != nil {
		return ErrNetworkSetFirst07Block
	}
	var start, end uint64
	if start, err = strconv.ParseUint(elems[7], 10, 64); err != nil {
		return ErrNetworkSetUnverifRangeStart
	}
	if end, err = strconv.ParseUint(elems[8], 10, 64); err != nil {
		return ErrNetworkSetUnverifRangeEnd
	}
	n.blockHashMetaInfo.UnverifiableRange = []uint64{start, end}
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

func (n Network) MetaInfo() *blockHashMetaInfo {
	return n.blockHashMetaInfo
}
