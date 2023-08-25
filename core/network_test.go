package core_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var networkStrings = map[core.Network]string{
	core.MAINNET:     "mainnet",
	core.GOERLI:      "goerli",
	core.GOERLI2:     "goerli2",
	core.INTEGRATION: "integration",
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
			case core.GOERLI:
				assert.Equal(t, "https://alpha4.starknet.io/feeder_gateway/", n.FeederURL())
			case core.MAINNET:
				assert.Equal(t, "https://alpha-mainnet.starknet.io/feeder_gateway/", n.FeederURL())
			case core.GOERLI2:
				assert.Equal(t, "https://alpha4-2.starknet.io/feeder_gateway/", n.FeederURL())
			case core.INTEGRATION:
				assert.Equal(t, "https://external.integration.starknet.io/feeder_gateway/", n.FeederURL())
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
	t.Run("chainId", func(t *testing.T) {
		for n := range networkStrings {
			switch n {
			case core.GOERLI, core.INTEGRATION:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_GOERLI")), n.ChainID())
			case core.MAINNET:
				assert.Equal(t, new(felt.Felt).SetBytes([]byte("SN_MAIN")), n.ChainID())
			case core.GOERLI2:
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
			case core.MAINNET:
				assert.Equal(t, big.NewInt(1), got)
			case core.GOERLI, core.GOERLI2, core.INTEGRATION:
				assert.Equal(t, big.NewInt(5), got)
			default:
				assert.Fail(t, "unexpected network")
			}
		}
	})
}

// see comment in utils/log_test.go
func TestNetworkSet(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(core.Network)
			require.NoError(t, n.Set(str))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(core.Network)
			require.NoError(t, n.Set(uppercase))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		n := new(core.Network)
		require.ErrorIs(t, n.Set("blah"), core.ErrUnknownNetwork)
	})
}

func TestNetworkUnmarshalText(t *testing.T) {
	for network, str := range networkStrings {
		t.Run("network "+str, func(t *testing.T) {
			n := new(core.Network)
			require.NoError(t, n.UnmarshalText([]byte(str)))
			assert.Equal(t, network, *n)
		})
		uppercase := strings.ToUpper(str)
		t.Run("network "+uppercase, func(t *testing.T) {
			n := new(core.Network)
			require.NoError(t, n.UnmarshalText([]byte(uppercase)))
			assert.Equal(t, network, *n)
		})
	}

	t.Run("unknown network", func(t *testing.T) {
		l := new(core.Network)
		require.ErrorIs(t, l.UnmarshalText([]byte("blah")), core.ErrUnknownNetwork)
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
	assert.Equal(t, "Network", new(core.Network).Type())
}

func TestCoreContractAddress(t *testing.T) {
	addresses := map[core.Network]common.Address{
		core.MAINNET: common.HexToAddress("0xc662c410C0ECf747543f5bA90660f6ABeBD9C8c4"),
		core.GOERLI:  common.HexToAddress("0xde29d060D45901Fb19ED6C6e959EB22d8626708e"),
		core.GOERLI2: common.HexToAddress("0xa4eD3aD27c294565cB0DCc993BDdCC75432D498c"),
	}

	for n := range networkStrings {
		t.Run("core contract for "+n.String(), func(t *testing.T) {
			switch n {
			case core.INTEGRATION:
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
