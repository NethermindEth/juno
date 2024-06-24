package utils_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkStrings = map[utils.Network]string{
	utils.Mainnet:            "mainnet",
	utils.Sepolia:            "sepolia",
	utils.SepoliaIntegration: "sepolia-integration",
}

func TestNetwork(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		for network, str := range networkStrings {
			assert.Equal(t, str, network.String())
		}
	})
	t.Run("feeder and gateway url", func(t *testing.T) {
		testCases := []struct {
			network    utils.Network
			feederURL  string
			gatewayURL string
		}{
			{utils.Mainnet, "https://alpha-mainnet.starknet.io/feeder_gateway/", "https://alpha-mainnet.starknet.io/gateway/"},
			{utils.Sepolia, "https://alpha-sepolia.starknet.io/feeder_gateway/", "https://alpha-sepolia.starknet.io/gateway/"},
			{utils.SepoliaIntegration, "https://integration-sepolia.starknet.io/feeder_gateway/", "https://integration-sepolia.starknet.io/gateway/"},
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
			case utils.Mainnet:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.L2ChainIDFelt())
			case utils.Sepolia:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_SEPOLIA")), n.L2ChainIDFelt())
			case utils.SepoliaIntegration:
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
			case utils.Mainnet:
				assert.Equal(t, big.NewInt(1), got)
			case utils.Sepolia, utils.SepoliaIntegration:
				assert.Equal(t, big.NewInt(11155111), got)
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
}

//nolint:dupl // see comment in utils/log_test.go
func TestNetworkSet(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) { //nolint:goconst
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
		n := utils.Network{}
		require.Error(t, n.Set("blah"))
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
	assert.Equal(t, "Network", new(utils.Network).Type())
}

func TestCoreContractAddress(t *testing.T) {
	addresses := map[utils.Network]common.Address{
		utils.Mainnet:            common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		utils.Sepolia:            common.HexToAddress("0xE2Bb56ee936fd6433DC0F6e7e3b8365C906AA057"),
		utils.SepoliaIntegration: common.HexToAddress("0x4737c0c1B4D5b1A687B42610DdabEE781152359c"),
	}

	for n := range networkStrings {
		t.Run("core contract for "+n.String(), func(t *testing.T) {
			want := addresses[n]
			assert.Equal(t, want, n.CoreContractAddress)
		})
	}
}
