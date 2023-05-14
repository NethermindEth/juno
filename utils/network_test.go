package utils_test

import (
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkStrings = map[utils.Network]string{
	utils.MAINNET:     "mainnet",
	utils.GOERLI:      "goerli",
	utils.GOERLI2:     "goerli2",
	utils.INTEGRATION: "integration",
}

func TestNetwork(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		for network, str := range networkStrings {
			assert.Equal(t, str, network.String())
		}
	})
	t.Run("url", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case utils.GOERLI:
				assert.Equal(t, "https://alpha4.starknet.io/feeder_gateway/", n.FeederURL())
			case utils.MAINNET:
				assert.Equal(t, "https://alpha-mainnet.starknet.io/feeder_gateway/", n.FeederURL())
			case utils.GOERLI2:
				assert.Equal(t, "https://alpha4-2.starknet.io/feeder_gateway/", n.FeederURL())
			case utils.INTEGRATION:
				assert.Equal(t, "https://external.integration.starknet.io/feeder_gateway/", n.FeederURL())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
	t.Run("chainId", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case utils.GOERLI, utils.INTEGRATION:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI")), n.ChainID())
			case utils.MAINNET:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.ChainID())
			case utils.GOERLI2:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI2")), n.ChainID())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
}

//nolint:dupl // see comment in utils/log_test.go
func TestNetworkSet(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.Set(str))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.Set(uppercase))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		n := new(utils.Network)
		require.ErrorIs(t, n.Set("blah"), utils.ErrUnknownNetwork)
	})
}

func TestNetworkUnmarshalText(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.UnmarshalText([]byte(str)))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(utils.Network)
			require.NoError(t, n.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		l := new(utils.Network)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), utils.ErrUnknownNetwork)
	})
}

func TestNetworkType(t *testing.T) {
	assert.Equal(t, "Network", new(utils.Network).Type())
}

func TestCoreContractAddress(t *testing.T) {
	addresses := map[utils.Network]common.Address{
		utils.MAINNET: common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		utils.GOERLI:  common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		utils.GOERLI2: common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
	}

	for n := range networkStrings {
		t.Run("core contract for "+n.String(), func(t *testing.T) {
			switch n {
			case utils.INTEGRATION:
				_, err := n.CoreContractAddress()
				require.Error(t, err)
			default:
				got, err := n.CoreContractAddress()
				require.NoError(t, err)
				want := addresses[n]
				assert.Equal(t, want, got)
			}
		})
	}
}
