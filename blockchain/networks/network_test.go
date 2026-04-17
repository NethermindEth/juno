package networks_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkStrings = map[networks.Network]string{
	networks.Mainnet:            "mainnet",
	networks.Sepolia:            "sepolia",
	networks.SepoliaIntegration: "sepolia-integration",
}

func TestNetwork(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		for network, str := range networkStrings {
			assert.Equal(t, str, network.String())
		}
	})
	t.Run("feeder and gateway url", func(t *testing.T) {
		testCases := []struct {
			network    networks.Network
			feederURL  string
			gatewayURL string
		}{
			{networks.Mainnet, "https://feeder.alpha-mainnet.starknet.io/feeder_gateway/", "https://alpha-mainnet.starknet.io/gateway/"},
			{networks.Goerli, "https://alpha4.starknet.io/feeder_gateway/", "https://alpha4.starknet.io/gateway/"},
			{networks.Goerli2, "https://alpha4-2.starknet.io/feeder_gateway/", "https://alpha4-2.starknet.io/gateway/"},
			{networks.Integration, "https://external.integration.starknet.io/feeder_gateway/", "https://external.integration.starknet.io/gateway/"},
			{networks.Sepolia, "https://feeder.alpha-sepolia.starknet.io/feeder_gateway/", "https://alpha-sepolia.starknet.io/gateway/"},
			{networks.SepoliaIntegration, "https://feeder.integration-sepolia.starknet.io/feeder_gateway/", "https://integration-sepolia.starknet.io/gateway/"},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("Network %v", tc.network), func(t *testing.T) {
				assert.Equal(t, tc.feederURL, tc.network.FeederURL)
				assert.Equal(t, tc.gatewayURL, tc.network.GatewayURL)
			})
		}
	})

	t.Run("chainId", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case networks.Goerli, networks.Integration:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI")), n.L2ChainIDFelt())
			case networks.Mainnet:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.L2ChainIDFelt())
			case networks.Goerli2:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI2")), n.L2ChainIDFelt())
			case networks.Sepolia:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_SEPOLIA")), n.L2ChainIDFelt())
			case networks.SepoliaIntegration:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_INTEGRATION_SEPOLIA")), n.L2ChainIDFelt())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
	t.Run("default L1 chainId", func(t *testing.T) {
		for n := range networkStrings {
			got := n.L1ChainID
			switch n {
			case networks.Mainnet:
				assert.Equal(t, big.NewInt(1), got)
			case networks.Goerli, networks.Goerli2, networks.Integration:
				assert.Equal(t, big.NewInt(5), got)
			case networks.Sepolia, networks.SepoliaIntegration:
				assert.Equal(t, big.NewInt(11155111), got)
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
}

func TestNetworkSet(t *testing.T) {
	for net, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(networks.Network)
			require.NoError(t, n.Set(str))
			assert.Equal(t, net, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(networks.Network)
			require.NoError(t, n.Set(uppercase))
			assert.Equal(t, net, *n)
		})
	}
	t.Run("unknown network", func(t *testing.T) {
		n := networks.Network{}
		require.Error(t, n.Set("blah"))
	})
}

func TestNetworkUnmarshalText(t *testing.T) {
	for net, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(networks.Network)
			require.NoError(t, n.UnmarshalText([]byte(str)))
			assert.Equal(t, net, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(networks.Network)
			require.NoError(t, n.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, net, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		l := new(networks.Network)
		require.Error(t, l.UnmarshalText([]byte("blah")))
	})
}

func TestNetworkMarshalJSON(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			nb, err := json.Marshal(&network)
			require.NoError(t, err)

			expectedStr := `"` + str + `"`
			assert.Equal(t, expectedStr, string(nb))
		})
	}
}

func TestNetworkType(t *testing.T) {
	assert.Equal(t, "Network", new(networks.Network).Type())
}

func TestCoreContractAddress(t *testing.T) {
	addresses := map[networks.Network]common.Address{
		networks.Mainnet:            common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		networks.Goerli:             common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		networks.Goerli2:            common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
		networks.Integration:        common.HexToAddress("0xd5c325D183C592C94998000C5e0EED9e6655c020"),
		networks.Sepolia:            common.HexToAddress("0xE2Bb56ee936fd6433DC0F6e7e3b8365C906AA057"),
		networks.SepoliaIntegration: common.HexToAddress("0x4737c0c1B4D5b1A687B42610DdabEE781152359c"),
	}

	for n := range networkStrings {
		t.Run("core contract for "+n.String(), func(t *testing.T) {
			want := addresses[n]
			assert.Equal(t, want, n.CoreContractAddress)
		})
	}
}
