package testchain

import (
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/core/address"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/stretchr/testify/require"
)

type deployedContract struct {
	t       *testing.T
	address address.ContractAddress
	balance felt.Felt
}

func (c *deployedContract) Address() *address.ContractAddress {
	return &c.address
}

func (c *deployedContract) Balance() *felt.Felt {
	return &c.balance
}

func (a *deployedContract) BalanceKey() felt.Felt {
	return feltFromNameAndKey(a.t, "ERC20_balances", a.address.AsFelt())
}

// https://github.com/eqlabs/pathfinder/blob/7664cba5145d8100ba1b6b2e2980432bc08d72a2/crates/common/src/lib.rs#L124
func feltFromNameAndKey(t *testing.T, name string, key *felt.Felt) felt.Felt {
	// TODO: The use of Big ints is not necessary at all. I am leaving it here because it is not critical
	//       but it should be change to using the felt implementation directly
	t.Helper()

	intermediate := crypto.StarknetKeccak([]byte(name))
	byteArr := crypto.Pedersen(intermediate, key).Bytes()
	value := new(big.Int).SetBytes(byteArr[:])

	maxAddr, ok := new(big.Int).SetString("0x7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff00", 0)
	require.True(t, ok)

	value = value.Rem(value, maxAddr)

	res := felt.Felt{}
	res.SetBigInt(value)

	return res
}
