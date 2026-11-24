package rpctest

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/NethermindEth/juno/blockchain/networks"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	starkcurve "github.com/consensys/gnark-crypto/ecc/stark-curve"
	"github.com/consensys/gnark-crypto/ecc/stark-curve/ecdsa"
	"github.com/stretchr/testify/require"
)

// AccountClassHash is the class hash of the genesis account contracts.
var AccountClassHash = felt.UnsafeFromString[felt.ClassHash](
	"0x61dac032f228abef9c6626f995015233097ae253a7f72d68552db02f2971b8f",
)

// Resource bounds of the transactions. The devnet sequencer sets every gas price to 1 and
// sends the state diff as calldata, so the state diff costs L1 gas.
const (
	maxL1Gas        = 0x2000
	maxL1DataGas    = 0x2000
	maxL2Gas        = 0x5f5e100
	maxPricePerUnit = 0x10
)

// Account is an account contract with the key that signs its transactions.
type Account struct {
	Address   felt.Address
	PublicKey felt.Felt
	key       *ecdsa.PrivateKey
	network   *networks.Network
}

// Account returns the genesis bootstrap account at the address.
func (d *Devnet) Account(t testing.TB, address string) *Account {
	t.Helper()
	want := felt.UnsafeFromString[felt.Felt](address)
	for _, account := range d.accounts {
		if account.Address.Equal(&want) {
			return &Account{
				Address:   felt.Address(account.Address),
				PublicKey: account.PubKey,
				key:       privateKey(t, &account.PrivKey),
				network:   d.network,
			}
		}
	}
	require.FailNow(t, "no genesis account", address)
	return nil
}

// NewAccount returns an account with a new key, at the address of its deploy account
// transaction. The account is not deployed.
func (d *Devnet) NewAccount(t testing.TB) *Account {
	t.Helper()
	key, err := ecdsa.GenerateKey(rand.Reader)
	require.NoError(t, err)
	publicKey := felt.Felt(key.PublicKey.A.X)
	salt := publicKey
	return &Account{
		Address: core.ContractAddress(
			new(felt.Zero[felt.Address]()), &AccountClassHash, &salt, []felt.Felt{publicKey},
		),
		PublicKey: publicKey,
		key:       key,
		network:   d.network,
	}
}

func privateKey(t testing.TB, scalar *felt.Felt) *ecdsa.PrivateKey {
	t.Helper()
	scalarBytes := scalar.Bytes()
	var publicKey starkcurve.G1Affine
	publicKey.ScalarMultiplicationBase(new(big.Int).SetBytes(scalarBytes[:]))
	publicKeyBytes := publicKey.Bytes()

	var key ecdsa.PrivateKey
	_, err := key.SetBytes(append(publicKeyBytes[:], scalarBytes[:]...))
	require.NoError(t, err)
	return &key
}

func (a *Account) sign(t testing.TB, hash *felt.TransactionHash) []string {
	t.Helper()
	hashBytes := hash.Bytes()
	signature, err := a.key.Sign(hashBytes[:], nil)
	require.NoError(t, err)
	r := felt.FromBytes[felt.Felt](signature[:felt.Bytes])
	s := felt.FromBytes[felt.Felt](signature[felt.Bytes:])
	return []string{r.String(), s.String()}
}

// Invocation is one call that the __execute__ entry point of an account makes.
type Invocation struct {
	To       felt.Address
	Selector felt.Felt
	Calldata []felt.Felt
}

// TransferSTRK returns the invocation that sends the amount of STRK, in FRI, to the address.
func TransferSTRK(to *felt.Address, amount uint64) Invocation {
	return Invocation{
		To:       networks.DefaultFeeTokenAddresses.StrkL2TokenAddress,
		Selector: crypto.StarknetKeccak([]byte("transfer")),
		Calldata: []felt.Felt{felt.Felt(*to), felt.FromUint64[felt.Felt](amount), {}},
	}
}

// Txn is a broadcasted transaction.
type Txn struct {
	Hash   felt.TransactionHash
	fields map[string]any
}

// String returns the transaction as JSON.
func (tx Txn) String() string {
	data, err := json.Marshal(tx.fields)
	if err != nil {
		panic(err) // the fields hold only strings, slices and maps
	}
	return string(data)
}

func resourceBounds() map[core.Resource]core.ResourceBounds {
	price := felt.FromUint64[felt.Felt](maxPricePerUnit)
	return map[core.Resource]core.ResourceBounds{
		core.ResourceL1Gas:     {MaxAmount: maxL1Gas, MaxPricePerUnit: &price},
		core.ResourceL1DataGas: {MaxAmount: maxL1DataGas, MaxPricePerUnit: &price},
		core.ResourceL2Gas:     {MaxAmount: maxL2Gas, MaxPricePerUnit: &price},
	}
}

func resourceBoundsJSON() map[string]any {
	bound := func(amount uint64) map[string]any {
		maxAmount := felt.FromUint64[felt.Felt](amount)
		price := felt.FromUint64[felt.Felt](maxPricePerUnit)
		return map[string]any{"max_amount": maxAmount.String(), "max_price_per_unit": price.String()}
	}
	return map[string]any{
		"l1_gas":      bound(maxL1Gas),
		"l1_data_gas": bound(maxL1DataGas),
		"l2_gas":      bound(maxL2Gas),
	}
}

func hexStrings(values []felt.Felt) []string {
	strings := make([]string, len(values))
	for i := range values {
		strings[i] = values[i].String()
	}
	return strings
}

// InvokeV3 returns a v3 invoke transaction from the account, signed by the account.
func (a *Account) InvokeV3(t testing.TB, nonce uint64, invocations ...Invocation) Txn {
	t.Helper()
	calldata := []felt.Felt{felt.FromUint64[felt.Felt](uint64(len(invocations)))}
	for _, invocation := range invocations {
		calldata = append(calldata,
			felt.Felt(invocation.To),
			invocation.Selector,
			felt.FromUint64[felt.Felt](uint64(len(invocation.Calldata))),
		)
		calldata = append(calldata, invocation.Calldata...)
	}

	nonceFelt := felt.FromUint64[felt.Felt](nonce)
	hash, err := core.TransactionHash(&core.InvokeTransaction{
		CallData:              calldata,
		Version:               new(core.TransactionVersion).SetUint64(3),
		Nonce:                 &nonceFelt,
		SenderAddress:         &a.Address,
		ResourceBounds:        resourceBounds(),
		PaymasterData:         []felt.Felt{},
		AccountDeploymentData: []felt.Felt{},
		NonceDAMode:           core.DAModeL1,
		FeeDAMode:             core.DAModeL1,
	}, a.network)
	require.NoError(t, err)

	return Txn{Hash: hash, fields: map[string]any{
		"type":                         "INVOKE",
		"version":                      "0x3",
		"sender_address":               a.Address.String(),
		"calldata":                     hexStrings(calldata),
		"signature":                    a.sign(t, &hash),
		"nonce":                        nonceFelt.String(),
		"resource_bounds":              resourceBoundsJSON(),
		"tip":                          "0x0",
		"paymaster_data":               []string{},
		"account_deployment_data":      []string{},
		"nonce_data_availability_mode": "L1",
		"fee_data_availability_mode":   "L1",
	}}
}

// DeployAccountV3 returns the v3 deploy account transaction of the account, signed by the
// account. The account must come from NewAccount.
func (a *Account) DeployAccountV3(t testing.TB) Txn {
	t.Helper()
	constructorCalldata := []felt.Felt{a.PublicKey}
	salt := a.PublicKey
	hash, err := core.TransactionHash(&core.DeployAccountTransaction{
		DeployTransaction: core.DeployTransaction{
			ContractAddressSalt: &salt,
			ContractAddress:     &a.Address,
			ClassHash:           &AccountClassHash,
			ConstructorCallData: constructorCalldata,
			Version:             new(core.TransactionVersion).SetUint64(3),
		},
		Nonce:          new(felt.Zero[felt.Felt]()),
		ResourceBounds: resourceBounds(),
		PaymasterData:  []felt.Felt{},
		NonceDAMode:    core.DAModeL1,
		FeeDAMode:      core.DAModeL1,
	}, a.network)
	require.NoError(t, err)

	return Txn{Hash: hash, fields: map[string]any{
		"type":                         "DEPLOY_ACCOUNT",
		"version":                      "0x3",
		"signature":                    a.sign(t, &hash),
		"nonce":                        "0x0",
		"contract_address_salt":        salt.String(),
		"constructor_calldata":         hexStrings(constructorCalldata),
		"class_hash":                   AccountClassHash.String(),
		"resource_bounds":              resourceBoundsJSON(),
		"tip":                          "0x0",
		"paymaster_data":               []string{},
		"nonce_data_availability_mode": "L1",
		"fee_data_availability_mode":   "L1",
	}}
}
