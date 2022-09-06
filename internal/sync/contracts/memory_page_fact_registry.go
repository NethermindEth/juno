// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contracts

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// MemoryPageFactRegistryMetaData contains all meta data concerning the MemoryPageFactRegistry contract.
var MemoryPageFactRegistryMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"factHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"memoryHash\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"prod\",\"type\":\"uint256\"}],\"name\":\"LogMemoryPageFactContinuous\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"factHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"memoryHash\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"prod\",\"type\":\"uint256\"}],\"name\":\"LogMemoryPageFactRegular\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"hasRegisteredFact\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fact\",\"type\":\"bytes32\"}],\"name\":\"isValid\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"startAddr\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"values\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"z\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"alpha\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"prime\",\"type\":\"uint256\"}],\"name\":\"registerContinuousMemoryPage\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"factHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"memoryHash\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"prod\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"memoryPairs\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"z\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"alpha\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"prime\",\"type\":\"uint256\"}],\"name\":\"registerRegularMemoryPage\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"factHash\",\"type\":\"bytes32\"},{\"internalType\":\"uint256\",\"name\":\"memoryHash\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"prod\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b506004361061004c5760003560e01c8063405a6362146100515780635578ceae146100eb5780636a938567146101a0578063d6354e15146101d1575b600080fd5b6100cd6004803603608081101561006757600080fd5b81019060208101813564010000000081111561008257600080fd5b82018360208201111561009457600080fd5b803590602001918460208302840111640100000000831117156100b657600080fd5b9193509150803590602081013590604001356101d9565b60408051938452602084019290925282820152519081900360600190f35b6100cd600480360360a081101561010157600080fd5b8135919081019060408101602082013564010000000081111561012357600080fd5b82018360208201111561013557600080fd5b8035906020019184602083028401116401000000008311171561015757600080fd5b9190808060200260200160405190810160405280939291908181526020018383602002808284376000920191909152509295505082359350505060208101359060400135610422565b6101bd600480360360208110156101b657600080fd5b5035610821565b604080519115158252519081900360200190f35b6101bd610832565b6000808062100000871061024e57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f546f6f206d616e79206d656d6f72792076616c7565732e000000000000000000604482015290519081900360640190fd5b60028706156102a8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260218152602001806109856021913960400191505060405180910390fd5b83861061031657604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601360248201527f496e76616c69642076616c7565206f66207a2e00000000000000000000000000604482015290519081900360640190fd5b83851061038457604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f496e76616c69642076616c7565206f6620616c7068612e000000000000000000604482015290519081900360640190fd5b6103c58888808060200260200160405190810160405280939291908181526020018383602002808284376000920191909152508a925089915088905061083b565b6040805184815260208101849052808201839052905193965091945092507f98fd0d40bd3e226c28fb29ff2d386bd8f9e19f2f8436441e6b854651d3b687b3919081900360600190a1610417836108ff565b955095509592505050565b60008060006210000087511061049957604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f546f6f206d616e79206d656d6f72792076616c7565732e000000000000000000604482015290519081900360640190fd5b7f40000000000000000000000000000000000000000000000000000000000000008410610511576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260388152602001806109a66038913960400191505060405180910390fd5b83861061057f57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601360248201527f496e76616c69642076616c7565206f66207a2e00000000000000000000000000604482015290519081900360640190fd5b8385106105ed57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f496e76616c69642076616c7565206f6620616c7068612e000000000000000000604482015290519081900360640190fd5b680100000000000000008810801561060457508388105b61066f57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601b60248201527f496e76616c69642076616c7565206f66207374617274416464722e0000000000604482015290519081900360640190fd5b5085516001906020880187860386900660078b018b84015b8082101561072f578889848b8d602089015109600686030101858c8e89510960078703010109870995508889848b8d606089015109600486030101858c8e60408a01510960058703010109870995508889848b8d60a089015109600286030101858c8e60808a01510960038703010109870995508889848b8d60e089015109850101858c8e60c08a015109600187030101098709955061010084019350600882019150610687565b6007820391505b808210156107635788898b8651098308925088838a038c0187099550602084019350600182019150610736565b50505060208083028a820120604080516001818501528082018a905260608101869052608081018c905260a081018b905260c0810187905260e081018390526101008082018f905282518083039091018152610120820180845281519190950120938490526101408101839052610160810187905290519297509095507fb8b9c39aeba1cfd98c38dfeebe11c2f7e02b334cbe9f05f22b442a5d9c1ea0c592508190036101800190a1610815846108ff565b50955095509592505050565b600061082c8261096f565b92915050565b60015460ff1690565b600080600080600288518161084c57fe5b0490506001915060208801604082028101815b818110156108865787888a60208401510982510888818a038c01870995505060400161085f565b5050816040028120935050600085828989868860006040516020018089815260200188815260200187815260200186815260200185815260200184815260200183815260200182815260200198505050505050505050604051602081830303815290604052805190602001209350509450945094915050565b600081815260208190526040902080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660019081179091555460ff1661096c57600180547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0016811790555b50565b60009081526020819052604090205460ff169056fe53697a65206f66206d656d6f72795061697273206d757374206265206576656e2e7072696d6520697320746f6f2062696720666f7220746865206f7074696d697a6174696f6e7320696e20746869732066756e6374696f6e2ea2646970667358221220ad57db530950b473eb8ddf831c79a963f910146fab4b8d61da87a811f12e0fd464736f6c634300060c0033",
}

// MemoryPageFactRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use MemoryPageFactRegistryMetaData.ABI instead.
var MemoryPageFactRegistryABI = MemoryPageFactRegistryMetaData.ABI

// MemoryPageFactRegistryBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use MemoryPageFactRegistryMetaData.Bin instead.
var MemoryPageFactRegistryBin = MemoryPageFactRegistryMetaData.Bin

// DeployMemoryPageFactRegistry deploys a new Ethereum contract, binding an instance of MemoryPageFactRegistry to it.
func DeployMemoryPageFactRegistry(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *MemoryPageFactRegistry, error) {
	parsed, err := MemoryPageFactRegistryMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(MemoryPageFactRegistryBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &MemoryPageFactRegistry{MemoryPageFactRegistryCaller: MemoryPageFactRegistryCaller{contract: contract}, MemoryPageFactRegistryTransactor: MemoryPageFactRegistryTransactor{contract: contract}, MemoryPageFactRegistryFilterer: MemoryPageFactRegistryFilterer{contract: contract}}, nil
}

// MemoryPageFactRegistry is an auto generated Go binding around an Ethereum contract.
type MemoryPageFactRegistry struct {
	MemoryPageFactRegistryCaller     // Read-only binding to the contract
	MemoryPageFactRegistryTransactor // Write-only binding to the contract
	MemoryPageFactRegistryFilterer   // Log filterer for contract events
}

// MemoryPageFactRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type MemoryPageFactRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MemoryPageFactRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MemoryPageFactRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MemoryPageFactRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type MemoryPageFactRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MemoryPageFactRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MemoryPageFactRegistrySession struct {
	Contract     *MemoryPageFactRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts           // Call options to use throughout this session
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// MemoryPageFactRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MemoryPageFactRegistryCallerSession struct {
	Contract *MemoryPageFactRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                 // Call options to use throughout this session
}

// MemoryPageFactRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MemoryPageFactRegistryTransactorSession struct {
	Contract     *MemoryPageFactRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                 // Transaction auth options to use throughout this session
}

// MemoryPageFactRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type MemoryPageFactRegistryRaw struct {
	Contract *MemoryPageFactRegistry // Generic contract binding to access the raw methods on
}

// MemoryPageFactRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MemoryPageFactRegistryCallerRaw struct {
	Contract *MemoryPageFactRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// MemoryPageFactRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MemoryPageFactRegistryTransactorRaw struct {
	Contract *MemoryPageFactRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMemoryPageFactRegistry creates a new instance of MemoryPageFactRegistry, bound to a specific deployed contract.
func NewMemoryPageFactRegistry(address common.Address, backend bind.ContractBackend) (*MemoryPageFactRegistry, error) {
	contract, err := bindMemoryPageFactRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &MemoryPageFactRegistry{MemoryPageFactRegistryCaller: MemoryPageFactRegistryCaller{contract: contract}, MemoryPageFactRegistryTransactor: MemoryPageFactRegistryTransactor{contract: contract}, MemoryPageFactRegistryFilterer: MemoryPageFactRegistryFilterer{contract: contract}}, nil
}

// NewMemoryPageFactRegistryCaller creates a new read-only instance of MemoryPageFactRegistry, bound to a specific deployed contract.
func NewMemoryPageFactRegistryCaller(address common.Address, caller bind.ContractCaller) (*MemoryPageFactRegistryCaller, error) {
	contract, err := bindMemoryPageFactRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &MemoryPageFactRegistryCaller{contract: contract}, nil
}

// NewMemoryPageFactRegistryTransactor creates a new write-only instance of MemoryPageFactRegistry, bound to a specific deployed contract.
func NewMemoryPageFactRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*MemoryPageFactRegistryTransactor, error) {
	contract, err := bindMemoryPageFactRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &MemoryPageFactRegistryTransactor{contract: contract}, nil
}

// NewMemoryPageFactRegistryFilterer creates a new log filterer instance of MemoryPageFactRegistry, bound to a specific deployed contract.
func NewMemoryPageFactRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*MemoryPageFactRegistryFilterer, error) {
	contract, err := bindMemoryPageFactRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &MemoryPageFactRegistryFilterer{contract: contract}, nil
}

// bindMemoryPageFactRegistry binds a generic wrapper to an already deployed contract.
func bindMemoryPageFactRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(MemoryPageFactRegistryABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MemoryPageFactRegistry *MemoryPageFactRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MemoryPageFactRegistry.Contract.MemoryPageFactRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MemoryPageFactRegistry *MemoryPageFactRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.MemoryPageFactRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MemoryPageFactRegistry *MemoryPageFactRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.MemoryPageFactRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MemoryPageFactRegistry *MemoryPageFactRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MemoryPageFactRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MemoryPageFactRegistry *MemoryPageFactRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MemoryPageFactRegistry *MemoryPageFactRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.contract.Transact(opts, method, params...)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryCaller) HasRegisteredFact(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _MemoryPageFactRegistry.contract.Call(opts, &out, "hasRegisteredFact")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_MemoryPageFactRegistry *MemoryPageFactRegistrySession) HasRegisteredFact() (bool, error) {
	return _MemoryPageFactRegistry.Contract.HasRegisteredFact(&_MemoryPageFactRegistry.CallOpts)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryCallerSession) HasRegisteredFact() (bool, error) {
	return _MemoryPageFactRegistry.Contract.HasRegisteredFact(&_MemoryPageFactRegistry.CallOpts)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryCaller) IsValid(opts *bind.CallOpts, fact [32]byte) (bool, error) {
	var out []interface{}
	err := _MemoryPageFactRegistry.contract.Call(opts, &out, "isValid", fact)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_MemoryPageFactRegistry *MemoryPageFactRegistrySession) IsValid(fact [32]byte) (bool, error) {
	return _MemoryPageFactRegistry.Contract.IsValid(&_MemoryPageFactRegistry.CallOpts, fact)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryCallerSession) IsValid(fact [32]byte) (bool, error) {
	return _MemoryPageFactRegistry.Contract.IsValid(&_MemoryPageFactRegistry.CallOpts, fact)
}

// RegisterContinuousMemoryPage is a paid mutator transaction binding the contract method 0x5578ceae.
//
// Solidity: function registerContinuousMemoryPage(uint256 startAddr, uint256[] values, uint256 z, uint256 alpha, uint256 prime) returns(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryTransactor) RegisterContinuousMemoryPage(opts *bind.TransactOpts, startAddr *big.Int, values []*big.Int, z *big.Int, alpha *big.Int, prime *big.Int) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.contract.Transact(opts, "registerContinuousMemoryPage", startAddr, values, z, alpha, prime)
}

// RegisterContinuousMemoryPage is a paid mutator transaction binding the contract method 0x5578ceae.
//
// Solidity: function registerContinuousMemoryPage(uint256 startAddr, uint256[] values, uint256 z, uint256 alpha, uint256 prime) returns(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistrySession) RegisterContinuousMemoryPage(startAddr *big.Int, values []*big.Int, z *big.Int, alpha *big.Int, prime *big.Int) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.RegisterContinuousMemoryPage(&_MemoryPageFactRegistry.TransactOpts, startAddr, values, z, alpha, prime)
}

// RegisterContinuousMemoryPage is a paid mutator transaction binding the contract method 0x5578ceae.
//
// Solidity: function registerContinuousMemoryPage(uint256 startAddr, uint256[] values, uint256 z, uint256 alpha, uint256 prime) returns(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryTransactorSession) RegisterContinuousMemoryPage(startAddr *big.Int, values []*big.Int, z *big.Int, alpha *big.Int, prime *big.Int) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.RegisterContinuousMemoryPage(&_MemoryPageFactRegistry.TransactOpts, startAddr, values, z, alpha, prime)
}

// RegisterRegularMemoryPage is a paid mutator transaction binding the contract method 0x405a6362.
//
// Solidity: function registerRegularMemoryPage(uint256[] memoryPairs, uint256 z, uint256 alpha, uint256 prime) returns(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryTransactor) RegisterRegularMemoryPage(opts *bind.TransactOpts, memoryPairs []*big.Int, z *big.Int, alpha *big.Int, prime *big.Int) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.contract.Transact(opts, "registerRegularMemoryPage", memoryPairs, z, alpha, prime)
}

// RegisterRegularMemoryPage is a paid mutator transaction binding the contract method 0x405a6362.
//
// Solidity: function registerRegularMemoryPage(uint256[] memoryPairs, uint256 z, uint256 alpha, uint256 prime) returns(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistrySession) RegisterRegularMemoryPage(memoryPairs []*big.Int, z *big.Int, alpha *big.Int, prime *big.Int) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.RegisterRegularMemoryPage(&_MemoryPageFactRegistry.TransactOpts, memoryPairs, z, alpha, prime)
}

// RegisterRegularMemoryPage is a paid mutator transaction binding the contract method 0x405a6362.
//
// Solidity: function registerRegularMemoryPage(uint256[] memoryPairs, uint256 z, uint256 alpha, uint256 prime) returns(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryTransactorSession) RegisterRegularMemoryPage(memoryPairs []*big.Int, z *big.Int, alpha *big.Int, prime *big.Int) (*types.Transaction, error) {
	return _MemoryPageFactRegistry.Contract.RegisterRegularMemoryPage(&_MemoryPageFactRegistry.TransactOpts, memoryPairs, z, alpha, prime)
}

// MemoryPageFactRegistryLogMemoryPageFactContinuousIterator is returned from FilterLogMemoryPageFactContinuous and is used to iterate over the raw logs and unpacked data for LogMemoryPageFactContinuous events raised by the MemoryPageFactRegistry contract.
type MemoryPageFactRegistryLogMemoryPageFactContinuousIterator struct {
	Event *MemoryPageFactRegistryLogMemoryPageFactContinuous // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *MemoryPageFactRegistryLogMemoryPageFactContinuousIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MemoryPageFactRegistryLogMemoryPageFactContinuous)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(MemoryPageFactRegistryLogMemoryPageFactContinuous)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *MemoryPageFactRegistryLogMemoryPageFactContinuousIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MemoryPageFactRegistryLogMemoryPageFactContinuousIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MemoryPageFactRegistryLogMemoryPageFactContinuous represents a LogMemoryPageFactContinuous event raised by the MemoryPageFactRegistry contract.
type MemoryPageFactRegistryLogMemoryPageFactContinuous struct {
	FactHash   [32]byte
	MemoryHash *big.Int
	Prod       *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterLogMemoryPageFactContinuous is a free log retrieval operation binding the contract event 0xb8b9c39aeba1cfd98c38dfeebe11c2f7e02b334cbe9f05f22b442a5d9c1ea0c5.
//
// Solidity: event LogMemoryPageFactContinuous(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryFilterer) FilterLogMemoryPageFactContinuous(opts *bind.FilterOpts) (*MemoryPageFactRegistryLogMemoryPageFactContinuousIterator, error) {

	logs, sub, err := _MemoryPageFactRegistry.contract.FilterLogs(opts, "LogMemoryPageFactContinuous")
	if err != nil {
		return nil, err
	}
	return &MemoryPageFactRegistryLogMemoryPageFactContinuousIterator{contract: _MemoryPageFactRegistry.contract, event: "LogMemoryPageFactContinuous", logs: logs, sub: sub}, nil
}

// WatchLogMemoryPageFactContinuous is a free log subscription operation binding the contract event 0xb8b9c39aeba1cfd98c38dfeebe11c2f7e02b334cbe9f05f22b442a5d9c1ea0c5.
//
// Solidity: event LogMemoryPageFactContinuous(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryFilterer) WatchLogMemoryPageFactContinuous(opts *bind.WatchOpts, sink chan<- *MemoryPageFactRegistryLogMemoryPageFactContinuous) (event.Subscription, error) {

	logs, sub, err := _MemoryPageFactRegistry.contract.WatchLogs(opts, "LogMemoryPageFactContinuous")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MemoryPageFactRegistryLogMemoryPageFactContinuous)
				if err := _MemoryPageFactRegistry.contract.UnpackLog(event, "LogMemoryPageFactContinuous", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLogMemoryPageFactContinuous is a log parse operation binding the contract event 0xb8b9c39aeba1cfd98c38dfeebe11c2f7e02b334cbe9f05f22b442a5d9c1ea0c5.
//
// Solidity: event LogMemoryPageFactContinuous(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryFilterer) ParseLogMemoryPageFactContinuous(log types.Log) (*MemoryPageFactRegistryLogMemoryPageFactContinuous, error) {
	event := new(MemoryPageFactRegistryLogMemoryPageFactContinuous)
	if err := _MemoryPageFactRegistry.contract.UnpackLog(event, "LogMemoryPageFactContinuous", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// MemoryPageFactRegistryLogMemoryPageFactRegularIterator is returned from FilterLogMemoryPageFactRegular and is used to iterate over the raw logs and unpacked data for LogMemoryPageFactRegular events raised by the MemoryPageFactRegistry contract.
type MemoryPageFactRegistryLogMemoryPageFactRegularIterator struct {
	Event *MemoryPageFactRegistryLogMemoryPageFactRegular // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *MemoryPageFactRegistryLogMemoryPageFactRegularIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(MemoryPageFactRegistryLogMemoryPageFactRegular)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(MemoryPageFactRegistryLogMemoryPageFactRegular)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *MemoryPageFactRegistryLogMemoryPageFactRegularIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *MemoryPageFactRegistryLogMemoryPageFactRegularIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// MemoryPageFactRegistryLogMemoryPageFactRegular represents a LogMemoryPageFactRegular event raised by the MemoryPageFactRegistry contract.
type MemoryPageFactRegistryLogMemoryPageFactRegular struct {
	FactHash   [32]byte
	MemoryHash *big.Int
	Prod       *big.Int
	Raw        types.Log // Blockchain specific contextual infos
}

// FilterLogMemoryPageFactRegular is a free log retrieval operation binding the contract event 0x98fd0d40bd3e226c28fb29ff2d386bd8f9e19f2f8436441e6b854651d3b687b3.
//
// Solidity: event LogMemoryPageFactRegular(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryFilterer) FilterLogMemoryPageFactRegular(opts *bind.FilterOpts) (*MemoryPageFactRegistryLogMemoryPageFactRegularIterator, error) {

	logs, sub, err := _MemoryPageFactRegistry.contract.FilterLogs(opts, "LogMemoryPageFactRegular")
	if err != nil {
		return nil, err
	}
	return &MemoryPageFactRegistryLogMemoryPageFactRegularIterator{contract: _MemoryPageFactRegistry.contract, event: "LogMemoryPageFactRegular", logs: logs, sub: sub}, nil
}

// WatchLogMemoryPageFactRegular is a free log subscription operation binding the contract event 0x98fd0d40bd3e226c28fb29ff2d386bd8f9e19f2f8436441e6b854651d3b687b3.
//
// Solidity: event LogMemoryPageFactRegular(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryFilterer) WatchLogMemoryPageFactRegular(opts *bind.WatchOpts, sink chan<- *MemoryPageFactRegistryLogMemoryPageFactRegular) (event.Subscription, error) {

	logs, sub, err := _MemoryPageFactRegistry.contract.WatchLogs(opts, "LogMemoryPageFactRegular")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(MemoryPageFactRegistryLogMemoryPageFactRegular)
				if err := _MemoryPageFactRegistry.contract.UnpackLog(event, "LogMemoryPageFactRegular", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseLogMemoryPageFactRegular is a log parse operation binding the contract event 0x98fd0d40bd3e226c28fb29ff2d386bd8f9e19f2f8436441e6b854651d3b687b3.
//
// Solidity: event LogMemoryPageFactRegular(bytes32 factHash, uint256 memoryHash, uint256 prod)
func (_MemoryPageFactRegistry *MemoryPageFactRegistryFilterer) ParseLogMemoryPageFactRegular(log types.Log) (*MemoryPageFactRegistryLogMemoryPageFactRegular, error) {
	event := new(MemoryPageFactRegistryLogMemoryPageFactRegular)
	if err := _MemoryPageFactRegistry.contract.UnpackLog(event, "LogMemoryPageFactRegular", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
