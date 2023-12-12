package utils_test

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkStrings = map[utils.NetworkKnown]string{
	utils.Mainnet:     "mainnet",
	utils.Goerli:      "goerli",
	utils.Goerli2:     "goerli2",
	utils.Integration: "integration",
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
			case utils.Goerli:
				assert.Equal(t, "https://alpha4.starknet.io/feeder_gateway/", n.FeederURL())
			case utils.Mainnet:
				assert.Equal(t, "https://alpha-mainnet.starknet.io/feeder_gateway/", n.FeederURL())
			case utils.Goerli2:
				assert.Equal(t, "https://alpha4-2.starknet.io/feeder_gateway/", n.FeederURL())
			case utils.Integration:
				assert.Equal(t, "https://external.integration.starknet.io/feeder_gateway/", n.FeederURL())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
	t.Run("chainId", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case utils.Goerli, utils.Integration:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI")), n.ChainID())
			case utils.Mainnet:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.ChainID())
			case utils.Goerli2:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI2")), n.ChainID())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
	t.Run("default L1 chainId", func(t *testing.T) {
		for n := range networkStrings {
			got := n.DefaultL1ChainID()
			switch n {
			case utils.Mainnet:
				assert.Equal(t, big.NewInt(1), got)
			case utils.Goerli, utils.Goerli2, utils.Integration:
				assert.Equal(t, big.NewInt(5), got)
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
			n := new(utils.NetworkKnown)
			require.NoError(t, n.Set(str))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(utils.NetworkKnown)
			require.NoError(t, n.Set(uppercase))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		n := new(utils.NetworkKnown)
		require.ErrorIs(t, n.Set("blah"), utils.ErrUnknownNetwork)
	})
}

func TestNetworkUnmarshalText(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(utils.NetworkKnown)
			require.NoError(t, n.UnmarshalText([]byte(str)))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(utils.NetworkKnown)
			require.NoError(t, n.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		l := new(utils.NetworkKnown)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), utils.ErrUnknownNetwork)
	})
}

func TestNetworkMarshalJSON(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			nb, err := network.MarshalJSON()
			require.NoError(t, err)

			expectedStr := `"` + str + `"`
			assert.Equal(t, expectedStr, string(nb))
		})
	}
}

func TestNetworkType(t *testing.T) {
	assert.Equal(t, "NetworkKnown", new(utils.NetworkKnown).Type())
}

func TestCoreContractAddress(t *testing.T) {
	addresses := map[utils.NetworkKnown]common.Address{
		utils.Mainnet:     common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		utils.Goerli:      common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		utils.Goerli2:     common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
		utils.Integration: common.HexToAddress("0xd5c325D183C592C94998000C5e0EED9e6655c020"),
	}

	for n := range networkStrings {
		t.Run("core contract for "+n.String(), func(t *testing.T) {
			got, err := n.CoreContractAddress()
			require.NoError(t, err)
			want := addresses[n]
			assert.Equal(t, want, got)
		})
	}
}

func TestUnmarshalJSON(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		data := []byte(`{
            "core_contract_address": "0x0000000000000000000000000000000000000000"
        }`)

		var nc utils.NetworkCustom
		err := json.Unmarshal(data, &nc)
		require.NoError(t, err)
		require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000000000"), *nc.CoreContractAddressVal)
	})

	t.Run("fail", func(t *testing.T) {
		data := []byte(`{
            "core_contract_address": "invalid_address"
        }`)

		var nc utils.NetworkCustom
		err := json.Unmarshal(data, &nc)
		require.Error(t, err)
	})
}
