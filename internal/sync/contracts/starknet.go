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

// StarknetMetaData contains all meta data concerning the Starknet contract.
var StarknetMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"fromAddress\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"toAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"}],\"name\":\"ConsumedMessageToL1\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"fromAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"ConsumedMessageToL2\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"fromAddress\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"toAddress\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"}],\"name\":\"LogMessageToL1\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"fromAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"LogMessageToL2\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"acceptedGovernor\",\"type\":\"address\"}],\"name\":\"LogNewGovernorAccepted\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"nominatedGovernor\",\"type\":\"address\"}],\"name\":\"LogNominatedGovernor\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"LogNominationCancelled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"LogOperatorAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"operator\",\"type\":\"address\"}],\"name\":\"LogOperatorRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"removedGovernor\",\"type\":\"address\"}],\"name\":\"LogRemovedGovernor\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"stateTransitionFact\",\"type\":\"bytes32\"}],\"name\":\"LogStateTransitionFact\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"globalRoot\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"int256\",\"name\":\"blockNumber\",\"type\":\"int256\"}],\"name\":\"LogStateUpdate\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"fromAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"MessageToL2Canceled\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"fromAddress\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"indexed\":true,\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"MessageToL2CancellationStarted\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"cancelL1ToL2Message\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"configHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"fromAddress\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"}],\"name\":\"consumeMessageFromL2\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"finalize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"identify\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"initialize\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isFinalized\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"isFrozen\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"testedOperator\",\"type\":\"address\"}],\"name\":\"isOperator\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"msgHash\",\"type\":\"bytes32\"}],\"name\":\"l1ToL2MessageCancellations\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"l1ToL2MessageNonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"msgHash\",\"type\":\"bytes32\"}],\"name\":\"l1ToL2Messages\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"msgHash\",\"type\":\"bytes32\"}],\"name\":\"l2ToL1Messages\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"messageCancellationDelay\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"programHash\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOperator\",\"type\":\"address\"}],\"name\":\"registerOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"}],\"name\":\"sendMessageToL2\",\"outputs\":[{\"internalType\":\"bytes32\",\"name\":\"\",\"type\":\"bytes32\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newConfigHash\",\"type\":\"uint256\"}],\"name\":\"setConfigHash\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"delayInSeconds\",\"type\":\"uint256\"}],\"name\":\"setMessageCancellationDelay\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"newProgramHash\",\"type\":\"uint256\"}],\"name\":\"setProgramHash\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"starknetAcceptGovernance\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"starknetCancelNomination\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"testGovernor\",\"type\":\"address\"}],\"name\":\"starknetIsGovernor\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newGovernor\",\"type\":\"address\"}],\"name\":\"starknetNominateNewGovernor\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"governorForRemoval\",\"type\":\"address\"}],\"name\":\"starknetRemoveGovernor\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"toAddress\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"selector\",\"type\":\"uint256\"},{\"internalType\":\"uint256[]\",\"name\":\"payload\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"startL1ToL2MessageCancellation\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"stateBlockNumber\",\"outputs\":[{\"internalType\":\"int256\",\"name\":\"\",\"type\":\"int256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"stateRoot\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"removedOperator\",\"type\":\"address\"}],\"name\":\"unregisterOperator\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"programOutput\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"onchainDataHash\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"onchainDataSize\",\"type\":\"uint256\"}],\"name\":\"updateState\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// StarknetABI is the input ABI used to generate the binding from.
// Deprecated: Use StarknetMetaData.ABI instead.
var StarknetABI = StarknetMetaData.ABI

// Starknet is an auto generated Go binding around an Ethereum contract.
type Starknet struct {
	StarknetCaller     // Read-only binding to the contract
	StarknetTransactor // Write-only binding to the contract
	StarknetFilterer   // Log filterer for contract events
}

// StarknetCaller is an auto generated read-only Go binding around an Ethereum contract.
type StarknetCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StarknetTransactor is an auto generated write-only Go binding around an Ethereum contract.
type StarknetTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StarknetFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type StarknetFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// StarknetSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type StarknetSession struct {
	Contract     *Starknet         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// StarknetCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type StarknetCallerSession struct {
	Contract *StarknetCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// StarknetTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type StarknetTransactorSession struct {
	Contract     *StarknetTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// StarknetRaw is an auto generated low-level Go binding around an Ethereum contract.
type StarknetRaw struct {
	Contract *Starknet // Generic contract binding to access the raw methods on
}

// StarknetCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type StarknetCallerRaw struct {
	Contract *StarknetCaller // Generic read-only contract binding to access the raw methods on
}

// StarknetTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type StarknetTransactorRaw struct {
	Contract *StarknetTransactor // Generic write-only contract binding to access the raw methods on
}

// NewStarknet creates a new instance of Starknet, bound to a specific deployed contract.
func NewStarknet(address common.Address, backend bind.ContractBackend) (*Starknet, error) {
	contract, err := bindStarknet(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Starknet{StarknetCaller: StarknetCaller{contract: contract}, StarknetTransactor: StarknetTransactor{contract: contract}, StarknetFilterer: StarknetFilterer{contract: contract}}, nil
}

// NewStarknetCaller creates a new read-only instance of Starknet, bound to a specific deployed contract.
func NewStarknetCaller(address common.Address, caller bind.ContractCaller) (*StarknetCaller, error) {
	contract, err := bindStarknet(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &StarknetCaller{contract: contract}, nil
}

// NewStarknetTransactor creates a new write-only instance of Starknet, bound to a specific deployed contract.
func NewStarknetTransactor(address common.Address, transactor bind.ContractTransactor) (*StarknetTransactor, error) {
	contract, err := bindStarknet(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &StarknetTransactor{contract: contract}, nil
}

// NewStarknetFilterer creates a new log filterer instance of Starknet, bound to a specific deployed contract.
func NewStarknetFilterer(address common.Address, filterer bind.ContractFilterer) (*StarknetFilterer, error) {
	contract, err := bindStarknet(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &StarknetFilterer{contract: contract}, nil
}

// bindStarknet binds a generic wrapper to an already deployed contract.
func bindStarknet(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(StarknetABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Starknet *StarknetRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Starknet.Contract.StarknetCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Starknet *StarknetRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Starknet.Contract.StarknetTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Starknet *StarknetRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Starknet.Contract.StarknetTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Starknet *StarknetCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Starknet.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Starknet *StarknetTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Starknet.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Starknet *StarknetTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Starknet.Contract.contract.Transact(opts, method, params...)
}

// ConfigHash is a free data retrieval call binding the contract method 0xe1f1176d.
//
// Solidity: function configHash() view returns(uint256)
func (_Starknet *StarknetCaller) ConfigHash(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "configHash")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ConfigHash is a free data retrieval call binding the contract method 0xe1f1176d.
//
// Solidity: function configHash() view returns(uint256)
func (_Starknet *StarknetSession) ConfigHash() (*big.Int, error) {
	return _Starknet.Contract.ConfigHash(&_Starknet.CallOpts)
}

// ConfigHash is a free data retrieval call binding the contract method 0xe1f1176d.
//
// Solidity: function configHash() view returns(uint256)
func (_Starknet *StarknetCallerSession) ConfigHash() (*big.Int, error) {
	return _Starknet.Contract.ConfigHash(&_Starknet.CallOpts)
}

// Identify is a free data retrieval call binding the contract method 0xeeb72866.
//
// Solidity: function identify() pure returns(string)
func (_Starknet *StarknetCaller) Identify(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "identify")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Identify is a free data retrieval call binding the contract method 0xeeb72866.
//
// Solidity: function identify() pure returns(string)
func (_Starknet *StarknetSession) Identify() (string, error) {
	return _Starknet.Contract.Identify(&_Starknet.CallOpts)
}

// Identify is a free data retrieval call binding the contract method 0xeeb72866.
//
// Solidity: function identify() pure returns(string)
func (_Starknet *StarknetCallerSession) Identify() (string, error) {
	return _Starknet.Contract.Identify(&_Starknet.CallOpts)
}

// IsFinalized is a free data retrieval call binding the contract method 0x8d4e4083.
//
// Solidity: function isFinalized() view returns(bool)
func (_Starknet *StarknetCaller) IsFinalized(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "isFinalized")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsFinalized is a free data retrieval call binding the contract method 0x8d4e4083.
//
// Solidity: function isFinalized() view returns(bool)
func (_Starknet *StarknetSession) IsFinalized() (bool, error) {
	return _Starknet.Contract.IsFinalized(&_Starknet.CallOpts)
}

// IsFinalized is a free data retrieval call binding the contract method 0x8d4e4083.
//
// Solidity: function isFinalized() view returns(bool)
func (_Starknet *StarknetCallerSession) IsFinalized() (bool, error) {
	return _Starknet.Contract.IsFinalized(&_Starknet.CallOpts)
}

// IsFrozen is a free data retrieval call binding the contract method 0x33eeb147.
//
// Solidity: function isFrozen() view returns(bool)
func (_Starknet *StarknetCaller) IsFrozen(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "isFrozen")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsFrozen is a free data retrieval call binding the contract method 0x33eeb147.
//
// Solidity: function isFrozen() view returns(bool)
func (_Starknet *StarknetSession) IsFrozen() (bool, error) {
	return _Starknet.Contract.IsFrozen(&_Starknet.CallOpts)
}

// IsFrozen is a free data retrieval call binding the contract method 0x33eeb147.
//
// Solidity: function isFrozen() view returns(bool)
func (_Starknet *StarknetCallerSession) IsFrozen() (bool, error) {
	return _Starknet.Contract.IsFrozen(&_Starknet.CallOpts)
}

// IsOperator is a free data retrieval call binding the contract method 0x6d70f7ae.
//
// Solidity: function isOperator(address testedOperator) view returns(bool)
func (_Starknet *StarknetCaller) IsOperator(opts *bind.CallOpts, testedOperator common.Address) (bool, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "isOperator", testedOperator)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsOperator is a free data retrieval call binding the contract method 0x6d70f7ae.
//
// Solidity: function isOperator(address testedOperator) view returns(bool)
func (_Starknet *StarknetSession) IsOperator(testedOperator common.Address) (bool, error) {
	return _Starknet.Contract.IsOperator(&_Starknet.CallOpts, testedOperator)
}

// IsOperator is a free data retrieval call binding the contract method 0x6d70f7ae.
//
// Solidity: function isOperator(address testedOperator) view returns(bool)
func (_Starknet *StarknetCallerSession) IsOperator(testedOperator common.Address) (bool, error) {
	return _Starknet.Contract.IsOperator(&_Starknet.CallOpts, testedOperator)
}

// L1ToL2MessageCancellations is a free data retrieval call binding the contract method 0x9be446bf.
//
// Solidity: function l1ToL2MessageCancellations(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetCaller) L1ToL2MessageCancellations(opts *bind.CallOpts, msgHash [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "l1ToL2MessageCancellations", msgHash)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// L1ToL2MessageCancellations is a free data retrieval call binding the contract method 0x9be446bf.
//
// Solidity: function l1ToL2MessageCancellations(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetSession) L1ToL2MessageCancellations(msgHash [32]byte) (*big.Int, error) {
	return _Starknet.Contract.L1ToL2MessageCancellations(&_Starknet.CallOpts, msgHash)
}

// L1ToL2MessageCancellations is a free data retrieval call binding the contract method 0x9be446bf.
//
// Solidity: function l1ToL2MessageCancellations(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetCallerSession) L1ToL2MessageCancellations(msgHash [32]byte) (*big.Int, error) {
	return _Starknet.Contract.L1ToL2MessageCancellations(&_Starknet.CallOpts, msgHash)
}

// L1ToL2MessageNonce is a free data retrieval call binding the contract method 0x018cccdf.
//
// Solidity: function l1ToL2MessageNonce() view returns(uint256)
func (_Starknet *StarknetCaller) L1ToL2MessageNonce(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "l1ToL2MessageNonce")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// L1ToL2MessageNonce is a free data retrieval call binding the contract method 0x018cccdf.
//
// Solidity: function l1ToL2MessageNonce() view returns(uint256)
func (_Starknet *StarknetSession) L1ToL2MessageNonce() (*big.Int, error) {
	return _Starknet.Contract.L1ToL2MessageNonce(&_Starknet.CallOpts)
}

// L1ToL2MessageNonce is a free data retrieval call binding the contract method 0x018cccdf.
//
// Solidity: function l1ToL2MessageNonce() view returns(uint256)
func (_Starknet *StarknetCallerSession) L1ToL2MessageNonce() (*big.Int, error) {
	return _Starknet.Contract.L1ToL2MessageNonce(&_Starknet.CallOpts)
}

// L1ToL2Messages is a free data retrieval call binding the contract method 0x77c7d7a9.
//
// Solidity: function l1ToL2Messages(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetCaller) L1ToL2Messages(opts *bind.CallOpts, msgHash [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "l1ToL2Messages", msgHash)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// L1ToL2Messages is a free data retrieval call binding the contract method 0x77c7d7a9.
//
// Solidity: function l1ToL2Messages(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetSession) L1ToL2Messages(msgHash [32]byte) (*big.Int, error) {
	return _Starknet.Contract.L1ToL2Messages(&_Starknet.CallOpts, msgHash)
}

// L1ToL2Messages is a free data retrieval call binding the contract method 0x77c7d7a9.
//
// Solidity: function l1ToL2Messages(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetCallerSession) L1ToL2Messages(msgHash [32]byte) (*big.Int, error) {
	return _Starknet.Contract.L1ToL2Messages(&_Starknet.CallOpts, msgHash)
}

// L2ToL1Messages is a free data retrieval call binding the contract method 0xa46efaf3.
//
// Solidity: function l2ToL1Messages(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetCaller) L2ToL1Messages(opts *bind.CallOpts, msgHash [32]byte) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "l2ToL1Messages", msgHash)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// L2ToL1Messages is a free data retrieval call binding the contract method 0xa46efaf3.
//
// Solidity: function l2ToL1Messages(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetSession) L2ToL1Messages(msgHash [32]byte) (*big.Int, error) {
	return _Starknet.Contract.L2ToL1Messages(&_Starknet.CallOpts, msgHash)
}

// L2ToL1Messages is a free data retrieval call binding the contract method 0xa46efaf3.
//
// Solidity: function l2ToL1Messages(bytes32 msgHash) view returns(uint256)
func (_Starknet *StarknetCallerSession) L2ToL1Messages(msgHash [32]byte) (*big.Int, error) {
	return _Starknet.Contract.L2ToL1Messages(&_Starknet.CallOpts, msgHash)
}

// MessageCancellationDelay is a free data retrieval call binding the contract method 0x8303bd8a.
//
// Solidity: function messageCancellationDelay() view returns(uint256)
func (_Starknet *StarknetCaller) MessageCancellationDelay(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "messageCancellationDelay")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// MessageCancellationDelay is a free data retrieval call binding the contract method 0x8303bd8a.
//
// Solidity: function messageCancellationDelay() view returns(uint256)
func (_Starknet *StarknetSession) MessageCancellationDelay() (*big.Int, error) {
	return _Starknet.Contract.MessageCancellationDelay(&_Starknet.CallOpts)
}

// MessageCancellationDelay is a free data retrieval call binding the contract method 0x8303bd8a.
//
// Solidity: function messageCancellationDelay() view returns(uint256)
func (_Starknet *StarknetCallerSession) MessageCancellationDelay() (*big.Int, error) {
	return _Starknet.Contract.MessageCancellationDelay(&_Starknet.CallOpts)
}

// ProgramHash is a free data retrieval call binding the contract method 0x8a9bf090.
//
// Solidity: function programHash() view returns(uint256)
func (_Starknet *StarknetCaller) ProgramHash(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "programHash")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// ProgramHash is a free data retrieval call binding the contract method 0x8a9bf090.
//
// Solidity: function programHash() view returns(uint256)
func (_Starknet *StarknetSession) ProgramHash() (*big.Int, error) {
	return _Starknet.Contract.ProgramHash(&_Starknet.CallOpts)
}

// ProgramHash is a free data retrieval call binding the contract method 0x8a9bf090.
//
// Solidity: function programHash() view returns(uint256)
func (_Starknet *StarknetCallerSession) ProgramHash() (*big.Int, error) {
	return _Starknet.Contract.ProgramHash(&_Starknet.CallOpts)
}

// StarknetIsGovernor is a free data retrieval call binding the contract method 0x01a01590.
//
// Solidity: function starknetIsGovernor(address testGovernor) view returns(bool)
func (_Starknet *StarknetCaller) StarknetIsGovernor(opts *bind.CallOpts, testGovernor common.Address) (bool, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "starknetIsGovernor", testGovernor)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// StarknetIsGovernor is a free data retrieval call binding the contract method 0x01a01590.
//
// Solidity: function starknetIsGovernor(address testGovernor) view returns(bool)
func (_Starknet *StarknetSession) StarknetIsGovernor(testGovernor common.Address) (bool, error) {
	return _Starknet.Contract.StarknetIsGovernor(&_Starknet.CallOpts, testGovernor)
}

// StarknetIsGovernor is a free data retrieval call binding the contract method 0x01a01590.
//
// Solidity: function starknetIsGovernor(address testGovernor) view returns(bool)
func (_Starknet *StarknetCallerSession) StarknetIsGovernor(testGovernor common.Address) (bool, error) {
	return _Starknet.Contract.StarknetIsGovernor(&_Starknet.CallOpts, testGovernor)
}

// StateBlockNumber is a free data retrieval call binding the contract method 0x35befa5d.
//
// Solidity: function stateBlockNumber() view returns(int256)
func (_Starknet *StarknetCaller) StateBlockNumber(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "stateBlockNumber")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StateBlockNumber is a free data retrieval call binding the contract method 0x35befa5d.
//
// Solidity: function stateBlockNumber() view returns(int256)
func (_Starknet *StarknetSession) StateBlockNumber() (*big.Int, error) {
	return _Starknet.Contract.StateBlockNumber(&_Starknet.CallOpts)
}

// StateBlockNumber is a free data retrieval call binding the contract method 0x35befa5d.
//
// Solidity: function stateBlockNumber() view returns(int256)
func (_Starknet *StarknetCallerSession) StateBlockNumber() (*big.Int, error) {
	return _Starknet.Contract.StateBlockNumber(&_Starknet.CallOpts)
}

// StateRoot is a free data retrieval call binding the contract method 0x9588eca2.
//
// Solidity: function stateRoot() view returns(uint256)
func (_Starknet *StarknetCaller) StateRoot(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _Starknet.contract.Call(opts, &out, "stateRoot")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// StateRoot is a free data retrieval call binding the contract method 0x9588eca2.
//
// Solidity: function stateRoot() view returns(uint256)
func (_Starknet *StarknetSession) StateRoot() (*big.Int, error) {
	return _Starknet.Contract.StateRoot(&_Starknet.CallOpts)
}

// StateRoot is a free data retrieval call binding the contract method 0x9588eca2.
//
// Solidity: function stateRoot() view returns(uint256)
func (_Starknet *StarknetCallerSession) StateRoot() (*big.Int, error) {
	return _Starknet.Contract.StateRoot(&_Starknet.CallOpts)
}

// CancelL1ToL2Message is a paid mutator transaction binding the contract method 0x6170ff1b.
//
// Solidity: function cancelL1ToL2Message(uint256 toAddress, uint256 selector, uint256[] payload, uint256 nonce) returns()
func (_Starknet *StarknetTransactor) CancelL1ToL2Message(opts *bind.TransactOpts, toAddress *big.Int, selector *big.Int, payload []*big.Int, nonce *big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "cancelL1ToL2Message", toAddress, selector, payload, nonce)
}

// CancelL1ToL2Message is a paid mutator transaction binding the contract method 0x6170ff1b.
//
// Solidity: function cancelL1ToL2Message(uint256 toAddress, uint256 selector, uint256[] payload, uint256 nonce) returns()
func (_Starknet *StarknetSession) CancelL1ToL2Message(toAddress *big.Int, selector *big.Int, payload []*big.Int, nonce *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.CancelL1ToL2Message(&_Starknet.TransactOpts, toAddress, selector, payload, nonce)
}

// CancelL1ToL2Message is a paid mutator transaction binding the contract method 0x6170ff1b.
//
// Solidity: function cancelL1ToL2Message(uint256 toAddress, uint256 selector, uint256[] payload, uint256 nonce) returns()
func (_Starknet *StarknetTransactorSession) CancelL1ToL2Message(toAddress *big.Int, selector *big.Int, payload []*big.Int, nonce *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.CancelL1ToL2Message(&_Starknet.TransactOpts, toAddress, selector, payload, nonce)
}

// ConsumeMessageFromL2 is a paid mutator transaction binding the contract method 0x2c9dd5c0.
//
// Solidity: function consumeMessageFromL2(uint256 fromAddress, uint256[] payload) returns(bytes32)
func (_Starknet *StarknetTransactor) ConsumeMessageFromL2(opts *bind.TransactOpts, fromAddress *big.Int, payload []*big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "consumeMessageFromL2", fromAddress, payload)
}

// ConsumeMessageFromL2 is a paid mutator transaction binding the contract method 0x2c9dd5c0.
//
// Solidity: function consumeMessageFromL2(uint256 fromAddress, uint256[] payload) returns(bytes32)
func (_Starknet *StarknetSession) ConsumeMessageFromL2(fromAddress *big.Int, payload []*big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.ConsumeMessageFromL2(&_Starknet.TransactOpts, fromAddress, payload)
}

// ConsumeMessageFromL2 is a paid mutator transaction binding the contract method 0x2c9dd5c0.
//
// Solidity: function consumeMessageFromL2(uint256 fromAddress, uint256[] payload) returns(bytes32)
func (_Starknet *StarknetTransactorSession) ConsumeMessageFromL2(fromAddress *big.Int, payload []*big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.ConsumeMessageFromL2(&_Starknet.TransactOpts, fromAddress, payload)
}

// Finalize is a paid mutator transaction binding the contract method 0x4bb278f3.
//
// Solidity: function finalize() returns()
func (_Starknet *StarknetTransactor) Finalize(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "finalize")
}

// Finalize is a paid mutator transaction binding the contract method 0x4bb278f3.
//
// Solidity: function finalize() returns()
func (_Starknet *StarknetSession) Finalize() (*types.Transaction, error) {
	return _Starknet.Contract.Finalize(&_Starknet.TransactOpts)
}

// Finalize is a paid mutator transaction binding the contract method 0x4bb278f3.
//
// Solidity: function finalize() returns()
func (_Starknet *StarknetTransactorSession) Finalize() (*types.Transaction, error) {
	return _Starknet.Contract.Finalize(&_Starknet.TransactOpts)
}

// Initialize is a paid mutator transaction binding the contract method 0x439fab91.
//
// Solidity: function initialize(bytes data) returns()
func (_Starknet *StarknetTransactor) Initialize(opts *bind.TransactOpts, data []byte) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "initialize", data)
}

// Initialize is a paid mutator transaction binding the contract method 0x439fab91.
//
// Solidity: function initialize(bytes data) returns()
func (_Starknet *StarknetSession) Initialize(data []byte) (*types.Transaction, error) {
	return _Starknet.Contract.Initialize(&_Starknet.TransactOpts, data)
}

// Initialize is a paid mutator transaction binding the contract method 0x439fab91.
//
// Solidity: function initialize(bytes data) returns()
func (_Starknet *StarknetTransactorSession) Initialize(data []byte) (*types.Transaction, error) {
	return _Starknet.Contract.Initialize(&_Starknet.TransactOpts, data)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address newOperator) returns()
func (_Starknet *StarknetTransactor) RegisterOperator(opts *bind.TransactOpts, newOperator common.Address) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "registerOperator", newOperator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address newOperator) returns()
func (_Starknet *StarknetSession) RegisterOperator(newOperator common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.RegisterOperator(&_Starknet.TransactOpts, newOperator)
}

// RegisterOperator is a paid mutator transaction binding the contract method 0x3682a450.
//
// Solidity: function registerOperator(address newOperator) returns()
func (_Starknet *StarknetTransactorSession) RegisterOperator(newOperator common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.RegisterOperator(&_Starknet.TransactOpts, newOperator)
}

// SendMessageToL2 is a paid mutator transaction binding the contract method 0x3e3aa6c5.
//
// Solidity: function sendMessageToL2(uint256 toAddress, uint256 selector, uint256[] payload) returns(bytes32)
func (_Starknet *StarknetTransactor) SendMessageToL2(opts *bind.TransactOpts, toAddress *big.Int, selector *big.Int, payload []*big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "sendMessageToL2", toAddress, selector, payload)
}

// SendMessageToL2 is a paid mutator transaction binding the contract method 0x3e3aa6c5.
//
// Solidity: function sendMessageToL2(uint256 toAddress, uint256 selector, uint256[] payload) returns(bytes32)
func (_Starknet *StarknetSession) SendMessageToL2(toAddress *big.Int, selector *big.Int, payload []*big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SendMessageToL2(&_Starknet.TransactOpts, toAddress, selector, payload)
}

// SendMessageToL2 is a paid mutator transaction binding the contract method 0x3e3aa6c5.
//
// Solidity: function sendMessageToL2(uint256 toAddress, uint256 selector, uint256[] payload) returns(bytes32)
func (_Starknet *StarknetTransactorSession) SendMessageToL2(toAddress *big.Int, selector *big.Int, payload []*big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SendMessageToL2(&_Starknet.TransactOpts, toAddress, selector, payload)
}

// SetConfigHash is a paid mutator transaction binding the contract method 0x3d07b336.
//
// Solidity: function setConfigHash(uint256 newConfigHash) returns()
func (_Starknet *StarknetTransactor) SetConfigHash(opts *bind.TransactOpts, newConfigHash *big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "setConfigHash", newConfigHash)
}

// SetConfigHash is a paid mutator transaction binding the contract method 0x3d07b336.
//
// Solidity: function setConfigHash(uint256 newConfigHash) returns()
func (_Starknet *StarknetSession) SetConfigHash(newConfigHash *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SetConfigHash(&_Starknet.TransactOpts, newConfigHash)
}

// SetConfigHash is a paid mutator transaction binding the contract method 0x3d07b336.
//
// Solidity: function setConfigHash(uint256 newConfigHash) returns()
func (_Starknet *StarknetTransactorSession) SetConfigHash(newConfigHash *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SetConfigHash(&_Starknet.TransactOpts, newConfigHash)
}

// SetMessageCancellationDelay is a paid mutator transaction binding the contract method 0xc99d397f.
//
// Solidity: function setMessageCancellationDelay(uint256 delayInSeconds) returns()
func (_Starknet *StarknetTransactor) SetMessageCancellationDelay(opts *bind.TransactOpts, delayInSeconds *big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "setMessageCancellationDelay", delayInSeconds)
}

// SetMessageCancellationDelay is a paid mutator transaction binding the contract method 0xc99d397f.
//
// Solidity: function setMessageCancellationDelay(uint256 delayInSeconds) returns()
func (_Starknet *StarknetSession) SetMessageCancellationDelay(delayInSeconds *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SetMessageCancellationDelay(&_Starknet.TransactOpts, delayInSeconds)
}

// SetMessageCancellationDelay is a paid mutator transaction binding the contract method 0xc99d397f.
//
// Solidity: function setMessageCancellationDelay(uint256 delayInSeconds) returns()
func (_Starknet *StarknetTransactorSession) SetMessageCancellationDelay(delayInSeconds *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SetMessageCancellationDelay(&_Starknet.TransactOpts, delayInSeconds)
}

// SetProgramHash is a paid mutator transaction binding the contract method 0xe87e7332.
//
// Solidity: function setProgramHash(uint256 newProgramHash) returns()
func (_Starknet *StarknetTransactor) SetProgramHash(opts *bind.TransactOpts, newProgramHash *big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "setProgramHash", newProgramHash)
}

// SetProgramHash is a paid mutator transaction binding the contract method 0xe87e7332.
//
// Solidity: function setProgramHash(uint256 newProgramHash) returns()
func (_Starknet *StarknetSession) SetProgramHash(newProgramHash *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SetProgramHash(&_Starknet.TransactOpts, newProgramHash)
}

// SetProgramHash is a paid mutator transaction binding the contract method 0xe87e7332.
//
// Solidity: function setProgramHash(uint256 newProgramHash) returns()
func (_Starknet *StarknetTransactorSession) SetProgramHash(newProgramHash *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.SetProgramHash(&_Starknet.TransactOpts, newProgramHash)
}

// StarknetAcceptGovernance is a paid mutator transaction binding the contract method 0x946be3ed.
//
// Solidity: function starknetAcceptGovernance() returns()
func (_Starknet *StarknetTransactor) StarknetAcceptGovernance(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "starknetAcceptGovernance")
}

// StarknetAcceptGovernance is a paid mutator transaction binding the contract method 0x946be3ed.
//
// Solidity: function starknetAcceptGovernance() returns()
func (_Starknet *StarknetSession) StarknetAcceptGovernance() (*types.Transaction, error) {
	return _Starknet.Contract.StarknetAcceptGovernance(&_Starknet.TransactOpts)
}

// StarknetAcceptGovernance is a paid mutator transaction binding the contract method 0x946be3ed.
//
// Solidity: function starknetAcceptGovernance() returns()
func (_Starknet *StarknetTransactorSession) StarknetAcceptGovernance() (*types.Transaction, error) {
	return _Starknet.Contract.StarknetAcceptGovernance(&_Starknet.TransactOpts)
}

// StarknetCancelNomination is a paid mutator transaction binding the contract method 0xe37fec25.
//
// Solidity: function starknetCancelNomination() returns()
func (_Starknet *StarknetTransactor) StarknetCancelNomination(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "starknetCancelNomination")
}

// StarknetCancelNomination is a paid mutator transaction binding the contract method 0xe37fec25.
//
// Solidity: function starknetCancelNomination() returns()
func (_Starknet *StarknetSession) StarknetCancelNomination() (*types.Transaction, error) {
	return _Starknet.Contract.StarknetCancelNomination(&_Starknet.TransactOpts)
}

// StarknetCancelNomination is a paid mutator transaction binding the contract method 0xe37fec25.
//
// Solidity: function starknetCancelNomination() returns()
func (_Starknet *StarknetTransactorSession) StarknetCancelNomination() (*types.Transaction, error) {
	return _Starknet.Contract.StarknetCancelNomination(&_Starknet.TransactOpts)
}

// StarknetNominateNewGovernor is a paid mutator transaction binding the contract method 0x91a66a26.
//
// Solidity: function starknetNominateNewGovernor(address newGovernor) returns()
func (_Starknet *StarknetTransactor) StarknetNominateNewGovernor(opts *bind.TransactOpts, newGovernor common.Address) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "starknetNominateNewGovernor", newGovernor)
}

// StarknetNominateNewGovernor is a paid mutator transaction binding the contract method 0x91a66a26.
//
// Solidity: function starknetNominateNewGovernor(address newGovernor) returns()
func (_Starknet *StarknetSession) StarknetNominateNewGovernor(newGovernor common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.StarknetNominateNewGovernor(&_Starknet.TransactOpts, newGovernor)
}

// StarknetNominateNewGovernor is a paid mutator transaction binding the contract method 0x91a66a26.
//
// Solidity: function starknetNominateNewGovernor(address newGovernor) returns()
func (_Starknet *StarknetTransactorSession) StarknetNominateNewGovernor(newGovernor common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.StarknetNominateNewGovernor(&_Starknet.TransactOpts, newGovernor)
}

// StarknetRemoveGovernor is a paid mutator transaction binding the contract method 0x84f921cd.
//
// Solidity: function starknetRemoveGovernor(address governorForRemoval) returns()
func (_Starknet *StarknetTransactor) StarknetRemoveGovernor(opts *bind.TransactOpts, governorForRemoval common.Address) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "starknetRemoveGovernor", governorForRemoval)
}

// StarknetRemoveGovernor is a paid mutator transaction binding the contract method 0x84f921cd.
//
// Solidity: function starknetRemoveGovernor(address governorForRemoval) returns()
func (_Starknet *StarknetSession) StarknetRemoveGovernor(governorForRemoval common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.StarknetRemoveGovernor(&_Starknet.TransactOpts, governorForRemoval)
}

// StarknetRemoveGovernor is a paid mutator transaction binding the contract method 0x84f921cd.
//
// Solidity: function starknetRemoveGovernor(address governorForRemoval) returns()
func (_Starknet *StarknetTransactorSession) StarknetRemoveGovernor(governorForRemoval common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.StarknetRemoveGovernor(&_Starknet.TransactOpts, governorForRemoval)
}

// StartL1ToL2MessageCancellation is a paid mutator transaction binding the contract method 0x7a98660b.
//
// Solidity: function startL1ToL2MessageCancellation(uint256 toAddress, uint256 selector, uint256[] payload, uint256 nonce) returns()
func (_Starknet *StarknetTransactor) StartL1ToL2MessageCancellation(opts *bind.TransactOpts, toAddress *big.Int, selector *big.Int, payload []*big.Int, nonce *big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "startL1ToL2MessageCancellation", toAddress, selector, payload, nonce)
}

// StartL1ToL2MessageCancellation is a paid mutator transaction binding the contract method 0x7a98660b.
//
// Solidity: function startL1ToL2MessageCancellation(uint256 toAddress, uint256 selector, uint256[] payload, uint256 nonce) returns()
func (_Starknet *StarknetSession) StartL1ToL2MessageCancellation(toAddress *big.Int, selector *big.Int, payload []*big.Int, nonce *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.StartL1ToL2MessageCancellation(&_Starknet.TransactOpts, toAddress, selector, payload, nonce)
}

// StartL1ToL2MessageCancellation is a paid mutator transaction binding the contract method 0x7a98660b.
//
// Solidity: function startL1ToL2MessageCancellation(uint256 toAddress, uint256 selector, uint256[] payload, uint256 nonce) returns()
func (_Starknet *StarknetTransactorSession) StartL1ToL2MessageCancellation(toAddress *big.Int, selector *big.Int, payload []*big.Int, nonce *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.StartL1ToL2MessageCancellation(&_Starknet.TransactOpts, toAddress, selector, payload, nonce)
}

// UnregisterOperator is a paid mutator transaction binding the contract method 0x96115bc2.
//
// Solidity: function unregisterOperator(address removedOperator) returns()
func (_Starknet *StarknetTransactor) UnregisterOperator(opts *bind.TransactOpts, removedOperator common.Address) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "unregisterOperator", removedOperator)
}

// UnregisterOperator is a paid mutator transaction binding the contract method 0x96115bc2.
//
// Solidity: function unregisterOperator(address removedOperator) returns()
func (_Starknet *StarknetSession) UnregisterOperator(removedOperator common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.UnregisterOperator(&_Starknet.TransactOpts, removedOperator)
}

// UnregisterOperator is a paid mutator transaction binding the contract method 0x96115bc2.
//
// Solidity: function unregisterOperator(address removedOperator) returns()
func (_Starknet *StarknetTransactorSession) UnregisterOperator(removedOperator common.Address) (*types.Transaction, error) {
	return _Starknet.Contract.UnregisterOperator(&_Starknet.TransactOpts, removedOperator)
}

// UpdateState is a paid mutator transaction binding the contract method 0x77552641.
//
// Solidity: function updateState(uint256[] programOutput, uint256 onchainDataHash, uint256 onchainDataSize) returns()
func (_Starknet *StarknetTransactor) UpdateState(opts *bind.TransactOpts, programOutput []*big.Int, onchainDataHash *big.Int, onchainDataSize *big.Int) (*types.Transaction, error) {
	return _Starknet.contract.Transact(opts, "updateState", programOutput, onchainDataHash, onchainDataSize)
}

// UpdateState is a paid mutator transaction binding the contract method 0x77552641.
//
// Solidity: function updateState(uint256[] programOutput, uint256 onchainDataHash, uint256 onchainDataSize) returns()
func (_Starknet *StarknetSession) UpdateState(programOutput []*big.Int, onchainDataHash *big.Int, onchainDataSize *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.UpdateState(&_Starknet.TransactOpts, programOutput, onchainDataHash, onchainDataSize)
}

// UpdateState is a paid mutator transaction binding the contract method 0x77552641.
//
// Solidity: function updateState(uint256[] programOutput, uint256 onchainDataHash, uint256 onchainDataSize) returns()
func (_Starknet *StarknetTransactorSession) UpdateState(programOutput []*big.Int, onchainDataHash *big.Int, onchainDataSize *big.Int) (*types.Transaction, error) {
	return _Starknet.Contract.UpdateState(&_Starknet.TransactOpts, programOutput, onchainDataHash, onchainDataSize)
}

// StarknetConsumedMessageToL1Iterator is returned from FilterConsumedMessageToL1 and is used to iterate over the raw logs and unpacked data for ConsumedMessageToL1 events raised by the Starknet contract.
type StarknetConsumedMessageToL1Iterator struct {
	Event *StarknetConsumedMessageToL1 // Event containing the contract specifics and raw log

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
func (it *StarknetConsumedMessageToL1Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetConsumedMessageToL1)
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
		it.Event = new(StarknetConsumedMessageToL1)
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
func (it *StarknetConsumedMessageToL1Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetConsumedMessageToL1Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetConsumedMessageToL1 represents a ConsumedMessageToL1 event raised by the Starknet contract.
type StarknetConsumedMessageToL1 struct {
	FromAddress *big.Int
	ToAddress   common.Address
	Payload     []*big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterConsumedMessageToL1 is a free log retrieval operation binding the contract event 0x7a06c571aa77f34d9706c51e5d8122b5595aebeaa34233bfe866f22befb973b1.
//
// Solidity: event ConsumedMessageToL1(uint256 indexed fromAddress, address indexed toAddress, uint256[] payload)
func (_Starknet *StarknetFilterer) FilterConsumedMessageToL1(opts *bind.FilterOpts, fromAddress []*big.Int, toAddress []common.Address) (*StarknetConsumedMessageToL1Iterator, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "ConsumedMessageToL1", fromAddressRule, toAddressRule)
	if err != nil {
		return nil, err
	}
	return &StarknetConsumedMessageToL1Iterator{contract: _Starknet.contract, event: "ConsumedMessageToL1", logs: logs, sub: sub}, nil
}

// WatchConsumedMessageToL1 is a free log subscription operation binding the contract event 0x7a06c571aa77f34d9706c51e5d8122b5595aebeaa34233bfe866f22befb973b1.
//
// Solidity: event ConsumedMessageToL1(uint256 indexed fromAddress, address indexed toAddress, uint256[] payload)
func (_Starknet *StarknetFilterer) WatchConsumedMessageToL1(opts *bind.WatchOpts, sink chan<- *StarknetConsumedMessageToL1, fromAddress []*big.Int, toAddress []common.Address) (event.Subscription, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "ConsumedMessageToL1", fromAddressRule, toAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetConsumedMessageToL1)
				if err := _Starknet.contract.UnpackLog(event, "ConsumedMessageToL1", log); err != nil {
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

// ParseConsumedMessageToL1 is a log parse operation binding the contract event 0x7a06c571aa77f34d9706c51e5d8122b5595aebeaa34233bfe866f22befb973b1.
//
// Solidity: event ConsumedMessageToL1(uint256 indexed fromAddress, address indexed toAddress, uint256[] payload)
func (_Starknet *StarknetFilterer) ParseConsumedMessageToL1(log types.Log) (*StarknetConsumedMessageToL1, error) {
	event := new(StarknetConsumedMessageToL1)
	if err := _Starknet.contract.UnpackLog(event, "ConsumedMessageToL1", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetConsumedMessageToL2Iterator is returned from FilterConsumedMessageToL2 and is used to iterate over the raw logs and unpacked data for ConsumedMessageToL2 events raised by the Starknet contract.
type StarknetConsumedMessageToL2Iterator struct {
	Event *StarknetConsumedMessageToL2 // Event containing the contract specifics and raw log

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
func (it *StarknetConsumedMessageToL2Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetConsumedMessageToL2)
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
		it.Event = new(StarknetConsumedMessageToL2)
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
func (it *StarknetConsumedMessageToL2Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetConsumedMessageToL2Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetConsumedMessageToL2 represents a ConsumedMessageToL2 event raised by the Starknet contract.
type StarknetConsumedMessageToL2 struct {
	FromAddress common.Address
	ToAddress   *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Nonce       *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterConsumedMessageToL2 is a free log retrieval operation binding the contract event 0x9592d37825c744e33fa80c469683bbd04d336241bb600b574758efd182abe26a.
//
// Solidity: event ConsumedMessageToL2(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) FilterConsumedMessageToL2(opts *bind.FilterOpts, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (*StarknetConsumedMessageToL2Iterator, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "ConsumedMessageToL2", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return &StarknetConsumedMessageToL2Iterator{contract: _Starknet.contract, event: "ConsumedMessageToL2", logs: logs, sub: sub}, nil
}

// WatchConsumedMessageToL2 is a free log subscription operation binding the contract event 0x9592d37825c744e33fa80c469683bbd04d336241bb600b574758efd182abe26a.
//
// Solidity: event ConsumedMessageToL2(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) WatchConsumedMessageToL2(opts *bind.WatchOpts, sink chan<- *StarknetConsumedMessageToL2, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (event.Subscription, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "ConsumedMessageToL2", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetConsumedMessageToL2)
				if err := _Starknet.contract.UnpackLog(event, "ConsumedMessageToL2", log); err != nil {
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

// ParseConsumedMessageToL2 is a log parse operation binding the contract event 0x9592d37825c744e33fa80c469683bbd04d336241bb600b574758efd182abe26a.
//
// Solidity: event ConsumedMessageToL2(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) ParseConsumedMessageToL2(log types.Log) (*StarknetConsumedMessageToL2, error) {
	event := new(StarknetConsumedMessageToL2)
	if err := _Starknet.contract.UnpackLog(event, "ConsumedMessageToL2", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogMessageToL1Iterator is returned from FilterLogMessageToL1 and is used to iterate over the raw logs and unpacked data for LogMessageToL1 events raised by the Starknet contract.
type StarknetLogMessageToL1Iterator struct {
	Event *StarknetLogMessageToL1 // Event containing the contract specifics and raw log

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
func (it *StarknetLogMessageToL1Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogMessageToL1)
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
		it.Event = new(StarknetLogMessageToL1)
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
func (it *StarknetLogMessageToL1Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogMessageToL1Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogMessageToL1 represents a LogMessageToL1 event raised by the Starknet contract.
type StarknetLogMessageToL1 struct {
	FromAddress *big.Int
	ToAddress   common.Address
	Payload     []*big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterLogMessageToL1 is a free log retrieval operation binding the contract event 0x4264ac208b5fde633ccdd42e0f12c3d6d443a4f3779bbf886925b94665b63a22.
//
// Solidity: event LogMessageToL1(uint256 indexed fromAddress, address indexed toAddress, uint256[] payload)
func (_Starknet *StarknetFilterer) FilterLogMessageToL1(opts *bind.FilterOpts, fromAddress []*big.Int, toAddress []common.Address) (*StarknetLogMessageToL1Iterator, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogMessageToL1", fromAddressRule, toAddressRule)
	if err != nil {
		return nil, err
	}
	return &StarknetLogMessageToL1Iterator{contract: _Starknet.contract, event: "LogMessageToL1", logs: logs, sub: sub}, nil
}

// WatchLogMessageToL1 is a free log subscription operation binding the contract event 0x4264ac208b5fde633ccdd42e0f12c3d6d443a4f3779bbf886925b94665b63a22.
//
// Solidity: event LogMessageToL1(uint256 indexed fromAddress, address indexed toAddress, uint256[] payload)
func (_Starknet *StarknetFilterer) WatchLogMessageToL1(opts *bind.WatchOpts, sink chan<- *StarknetLogMessageToL1, fromAddress []*big.Int, toAddress []common.Address) (event.Subscription, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogMessageToL1", fromAddressRule, toAddressRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogMessageToL1)
				if err := _Starknet.contract.UnpackLog(event, "LogMessageToL1", log); err != nil {
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

// ParseLogMessageToL1 is a log parse operation binding the contract event 0x4264ac208b5fde633ccdd42e0f12c3d6d443a4f3779bbf886925b94665b63a22.
//
// Solidity: event LogMessageToL1(uint256 indexed fromAddress, address indexed toAddress, uint256[] payload)
func (_Starknet *StarknetFilterer) ParseLogMessageToL1(log types.Log) (*StarknetLogMessageToL1, error) {
	event := new(StarknetLogMessageToL1)
	if err := _Starknet.contract.UnpackLog(event, "LogMessageToL1", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogMessageToL2Iterator is returned from FilterLogMessageToL2 and is used to iterate over the raw logs and unpacked data for LogMessageToL2 events raised by the Starknet contract.
type StarknetLogMessageToL2Iterator struct {
	Event *StarknetLogMessageToL2 // Event containing the contract specifics and raw log

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
func (it *StarknetLogMessageToL2Iterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogMessageToL2)
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
		it.Event = new(StarknetLogMessageToL2)
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
func (it *StarknetLogMessageToL2Iterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogMessageToL2Iterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogMessageToL2 represents a LogMessageToL2 event raised by the Starknet contract.
type StarknetLogMessageToL2 struct {
	FromAddress common.Address
	ToAddress   *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Nonce       *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterLogMessageToL2 is a free log retrieval operation binding the contract event 0x7d3450d4f5138e54dcb21a322312d50846ead7856426fb38778f8ef33aeccc01.
//
// Solidity: event LogMessageToL2(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) FilterLogMessageToL2(opts *bind.FilterOpts, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (*StarknetLogMessageToL2Iterator, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogMessageToL2", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return &StarknetLogMessageToL2Iterator{contract: _Starknet.contract, event: "LogMessageToL2", logs: logs, sub: sub}, nil
}

// WatchLogMessageToL2 is a free log subscription operation binding the contract event 0x7d3450d4f5138e54dcb21a322312d50846ead7856426fb38778f8ef33aeccc01.
//
// Solidity: event LogMessageToL2(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) WatchLogMessageToL2(opts *bind.WatchOpts, sink chan<- *StarknetLogMessageToL2, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (event.Subscription, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogMessageToL2", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogMessageToL2)
				if err := _Starknet.contract.UnpackLog(event, "LogMessageToL2", log); err != nil {
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

// ParseLogMessageToL2 is a log parse operation binding the contract event 0x7d3450d4f5138e54dcb21a322312d50846ead7856426fb38778f8ef33aeccc01.
//
// Solidity: event LogMessageToL2(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) ParseLogMessageToL2(log types.Log) (*StarknetLogMessageToL2, error) {
	event := new(StarknetLogMessageToL2)
	if err := _Starknet.contract.UnpackLog(event, "LogMessageToL2", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogNewGovernorAcceptedIterator is returned from FilterLogNewGovernorAccepted and is used to iterate over the raw logs and unpacked data for LogNewGovernorAccepted events raised by the Starknet contract.
type StarknetLogNewGovernorAcceptedIterator struct {
	Event *StarknetLogNewGovernorAccepted // Event containing the contract specifics and raw log

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
func (it *StarknetLogNewGovernorAcceptedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogNewGovernorAccepted)
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
		it.Event = new(StarknetLogNewGovernorAccepted)
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
func (it *StarknetLogNewGovernorAcceptedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogNewGovernorAcceptedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogNewGovernorAccepted represents a LogNewGovernorAccepted event raised by the Starknet contract.
type StarknetLogNewGovernorAccepted struct {
	AcceptedGovernor common.Address
	Raw              types.Log // Blockchain specific contextual infos
}

// FilterLogNewGovernorAccepted is a free log retrieval operation binding the contract event 0xcfb473e6c03f9a29ddaf990e736fa3de5188a0bd85d684f5b6e164ebfbfff5d2.
//
// Solidity: event LogNewGovernorAccepted(address acceptedGovernor)
func (_Starknet *StarknetFilterer) FilterLogNewGovernorAccepted(opts *bind.FilterOpts) (*StarknetLogNewGovernorAcceptedIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogNewGovernorAccepted")
	if err != nil {
		return nil, err
	}
	return &StarknetLogNewGovernorAcceptedIterator{contract: _Starknet.contract, event: "LogNewGovernorAccepted", logs: logs, sub: sub}, nil
}

// WatchLogNewGovernorAccepted is a free log subscription operation binding the contract event 0xcfb473e6c03f9a29ddaf990e736fa3de5188a0bd85d684f5b6e164ebfbfff5d2.
//
// Solidity: event LogNewGovernorAccepted(address acceptedGovernor)
func (_Starknet *StarknetFilterer) WatchLogNewGovernorAccepted(opts *bind.WatchOpts, sink chan<- *StarknetLogNewGovernorAccepted) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogNewGovernorAccepted")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogNewGovernorAccepted)
				if err := _Starknet.contract.UnpackLog(event, "LogNewGovernorAccepted", log); err != nil {
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

// ParseLogNewGovernorAccepted is a log parse operation binding the contract event 0xcfb473e6c03f9a29ddaf990e736fa3de5188a0bd85d684f5b6e164ebfbfff5d2.
//
// Solidity: event LogNewGovernorAccepted(address acceptedGovernor)
func (_Starknet *StarknetFilterer) ParseLogNewGovernorAccepted(log types.Log) (*StarknetLogNewGovernorAccepted, error) {
	event := new(StarknetLogNewGovernorAccepted)
	if err := _Starknet.contract.UnpackLog(event, "LogNewGovernorAccepted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogNominatedGovernorIterator is returned from FilterLogNominatedGovernor and is used to iterate over the raw logs and unpacked data for LogNominatedGovernor events raised by the Starknet contract.
type StarknetLogNominatedGovernorIterator struct {
	Event *StarknetLogNominatedGovernor // Event containing the contract specifics and raw log

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
func (it *StarknetLogNominatedGovernorIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogNominatedGovernor)
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
		it.Event = new(StarknetLogNominatedGovernor)
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
func (it *StarknetLogNominatedGovernorIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogNominatedGovernorIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogNominatedGovernor represents a LogNominatedGovernor event raised by the Starknet contract.
type StarknetLogNominatedGovernor struct {
	NominatedGovernor common.Address
	Raw               types.Log // Blockchain specific contextual infos
}

// FilterLogNominatedGovernor is a free log retrieval operation binding the contract event 0x6166272c8d3f5f579082f2827532732f97195007983bb5b83ac12c56700b01a6.
//
// Solidity: event LogNominatedGovernor(address nominatedGovernor)
func (_Starknet *StarknetFilterer) FilterLogNominatedGovernor(opts *bind.FilterOpts) (*StarknetLogNominatedGovernorIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogNominatedGovernor")
	if err != nil {
		return nil, err
	}
	return &StarknetLogNominatedGovernorIterator{contract: _Starknet.contract, event: "LogNominatedGovernor", logs: logs, sub: sub}, nil
}

// WatchLogNominatedGovernor is a free log subscription operation binding the contract event 0x6166272c8d3f5f579082f2827532732f97195007983bb5b83ac12c56700b01a6.
//
// Solidity: event LogNominatedGovernor(address nominatedGovernor)
func (_Starknet *StarknetFilterer) WatchLogNominatedGovernor(opts *bind.WatchOpts, sink chan<- *StarknetLogNominatedGovernor) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogNominatedGovernor")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogNominatedGovernor)
				if err := _Starknet.contract.UnpackLog(event, "LogNominatedGovernor", log); err != nil {
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

// ParseLogNominatedGovernor is a log parse operation binding the contract event 0x6166272c8d3f5f579082f2827532732f97195007983bb5b83ac12c56700b01a6.
//
// Solidity: event LogNominatedGovernor(address nominatedGovernor)
func (_Starknet *StarknetFilterer) ParseLogNominatedGovernor(log types.Log) (*StarknetLogNominatedGovernor, error) {
	event := new(StarknetLogNominatedGovernor)
	if err := _Starknet.contract.UnpackLog(event, "LogNominatedGovernor", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogNominationCancelledIterator is returned from FilterLogNominationCancelled and is used to iterate over the raw logs and unpacked data for LogNominationCancelled events raised by the Starknet contract.
type StarknetLogNominationCancelledIterator struct {
	Event *StarknetLogNominationCancelled // Event containing the contract specifics and raw log

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
func (it *StarknetLogNominationCancelledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogNominationCancelled)
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
		it.Event = new(StarknetLogNominationCancelled)
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
func (it *StarknetLogNominationCancelledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogNominationCancelledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogNominationCancelled represents a LogNominationCancelled event raised by the Starknet contract.
type StarknetLogNominationCancelled struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterLogNominationCancelled is a free log retrieval operation binding the contract event 0x7a8dc7dd7fffb43c4807438fa62729225156941e641fd877938f4edade3429f5.
//
// Solidity: event LogNominationCancelled()
func (_Starknet *StarknetFilterer) FilterLogNominationCancelled(opts *bind.FilterOpts) (*StarknetLogNominationCancelledIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogNominationCancelled")
	if err != nil {
		return nil, err
	}
	return &StarknetLogNominationCancelledIterator{contract: _Starknet.contract, event: "LogNominationCancelled", logs: logs, sub: sub}, nil
}

// WatchLogNominationCancelled is a free log subscription operation binding the contract event 0x7a8dc7dd7fffb43c4807438fa62729225156941e641fd877938f4edade3429f5.
//
// Solidity: event LogNominationCancelled()
func (_Starknet *StarknetFilterer) WatchLogNominationCancelled(opts *bind.WatchOpts, sink chan<- *StarknetLogNominationCancelled) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogNominationCancelled")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogNominationCancelled)
				if err := _Starknet.contract.UnpackLog(event, "LogNominationCancelled", log); err != nil {
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

// ParseLogNominationCancelled is a log parse operation binding the contract event 0x7a8dc7dd7fffb43c4807438fa62729225156941e641fd877938f4edade3429f5.
//
// Solidity: event LogNominationCancelled()
func (_Starknet *StarknetFilterer) ParseLogNominationCancelled(log types.Log) (*StarknetLogNominationCancelled, error) {
	event := new(StarknetLogNominationCancelled)
	if err := _Starknet.contract.UnpackLog(event, "LogNominationCancelled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogOperatorAddedIterator is returned from FilterLogOperatorAdded and is used to iterate over the raw logs and unpacked data for LogOperatorAdded events raised by the Starknet contract.
type StarknetLogOperatorAddedIterator struct {
	Event *StarknetLogOperatorAdded // Event containing the contract specifics and raw log

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
func (it *StarknetLogOperatorAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogOperatorAdded)
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
		it.Event = new(StarknetLogOperatorAdded)
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
func (it *StarknetLogOperatorAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogOperatorAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogOperatorAdded represents a LogOperatorAdded event raised by the Starknet contract.
type StarknetLogOperatorAdded struct {
	Operator common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterLogOperatorAdded is a free log retrieval operation binding the contract event 0x50a18c352ee1c02ffe058e15c2eb6e58be387c81e73cc1e17035286e54c19a57.
//
// Solidity: event LogOperatorAdded(address operator)
func (_Starknet *StarknetFilterer) FilterLogOperatorAdded(opts *bind.FilterOpts) (*StarknetLogOperatorAddedIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogOperatorAdded")
	if err != nil {
		return nil, err
	}
	return &StarknetLogOperatorAddedIterator{contract: _Starknet.contract, event: "LogOperatorAdded", logs: logs, sub: sub}, nil
}

// WatchLogOperatorAdded is a free log subscription operation binding the contract event 0x50a18c352ee1c02ffe058e15c2eb6e58be387c81e73cc1e17035286e54c19a57.
//
// Solidity: event LogOperatorAdded(address operator)
func (_Starknet *StarknetFilterer) WatchLogOperatorAdded(opts *bind.WatchOpts, sink chan<- *StarknetLogOperatorAdded) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogOperatorAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogOperatorAdded)
				if err := _Starknet.contract.UnpackLog(event, "LogOperatorAdded", log); err != nil {
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

// ParseLogOperatorAdded is a log parse operation binding the contract event 0x50a18c352ee1c02ffe058e15c2eb6e58be387c81e73cc1e17035286e54c19a57.
//
// Solidity: event LogOperatorAdded(address operator)
func (_Starknet *StarknetFilterer) ParseLogOperatorAdded(log types.Log) (*StarknetLogOperatorAdded, error) {
	event := new(StarknetLogOperatorAdded)
	if err := _Starknet.contract.UnpackLog(event, "LogOperatorAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogOperatorRemovedIterator is returned from FilterLogOperatorRemoved and is used to iterate over the raw logs and unpacked data for LogOperatorRemoved events raised by the Starknet contract.
type StarknetLogOperatorRemovedIterator struct {
	Event *StarknetLogOperatorRemoved // Event containing the contract specifics and raw log

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
func (it *StarknetLogOperatorRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogOperatorRemoved)
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
		it.Event = new(StarknetLogOperatorRemoved)
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
func (it *StarknetLogOperatorRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogOperatorRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogOperatorRemoved represents a LogOperatorRemoved event raised by the Starknet contract.
type StarknetLogOperatorRemoved struct {
	Operator common.Address
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterLogOperatorRemoved is a free log retrieval operation binding the contract event 0xec5f6c3a91a1efb1f9a308bb33c6e9e66bf9090fad0732f127dfdbf516d0625d.
//
// Solidity: event LogOperatorRemoved(address operator)
func (_Starknet *StarknetFilterer) FilterLogOperatorRemoved(opts *bind.FilterOpts) (*StarknetLogOperatorRemovedIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogOperatorRemoved")
	if err != nil {
		return nil, err
	}
	return &StarknetLogOperatorRemovedIterator{contract: _Starknet.contract, event: "LogOperatorRemoved", logs: logs, sub: sub}, nil
}

// WatchLogOperatorRemoved is a free log subscription operation binding the contract event 0xec5f6c3a91a1efb1f9a308bb33c6e9e66bf9090fad0732f127dfdbf516d0625d.
//
// Solidity: event LogOperatorRemoved(address operator)
func (_Starknet *StarknetFilterer) WatchLogOperatorRemoved(opts *bind.WatchOpts, sink chan<- *StarknetLogOperatorRemoved) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogOperatorRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogOperatorRemoved)
				if err := _Starknet.contract.UnpackLog(event, "LogOperatorRemoved", log); err != nil {
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

// ParseLogOperatorRemoved is a log parse operation binding the contract event 0xec5f6c3a91a1efb1f9a308bb33c6e9e66bf9090fad0732f127dfdbf516d0625d.
//
// Solidity: event LogOperatorRemoved(address operator)
func (_Starknet *StarknetFilterer) ParseLogOperatorRemoved(log types.Log) (*StarknetLogOperatorRemoved, error) {
	event := new(StarknetLogOperatorRemoved)
	if err := _Starknet.contract.UnpackLog(event, "LogOperatorRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogRemovedGovernorIterator is returned from FilterLogRemovedGovernor and is used to iterate over the raw logs and unpacked data for LogRemovedGovernor events raised by the Starknet contract.
type StarknetLogRemovedGovernorIterator struct {
	Event *StarknetLogRemovedGovernor // Event containing the contract specifics and raw log

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
func (it *StarknetLogRemovedGovernorIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogRemovedGovernor)
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
		it.Event = new(StarknetLogRemovedGovernor)
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
func (it *StarknetLogRemovedGovernorIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogRemovedGovernorIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogRemovedGovernor represents a LogRemovedGovernor event raised by the Starknet contract.
type StarknetLogRemovedGovernor struct {
	RemovedGovernor common.Address
	Raw             types.Log // Blockchain specific contextual infos
}

// FilterLogRemovedGovernor is a free log retrieval operation binding the contract event 0xd75f94825e770b8b512be8e74759e252ad00e102e38f50cce2f7c6f868a29599.
//
// Solidity: event LogRemovedGovernor(address removedGovernor)
func (_Starknet *StarknetFilterer) FilterLogRemovedGovernor(opts *bind.FilterOpts) (*StarknetLogRemovedGovernorIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogRemovedGovernor")
	if err != nil {
		return nil, err
	}
	return &StarknetLogRemovedGovernorIterator{contract: _Starknet.contract, event: "LogRemovedGovernor", logs: logs, sub: sub}, nil
}

// WatchLogRemovedGovernor is a free log subscription operation binding the contract event 0xd75f94825e770b8b512be8e74759e252ad00e102e38f50cce2f7c6f868a29599.
//
// Solidity: event LogRemovedGovernor(address removedGovernor)
func (_Starknet *StarknetFilterer) WatchLogRemovedGovernor(opts *bind.WatchOpts, sink chan<- *StarknetLogRemovedGovernor) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogRemovedGovernor")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogRemovedGovernor)
				if err := _Starknet.contract.UnpackLog(event, "LogRemovedGovernor", log); err != nil {
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

// ParseLogRemovedGovernor is a log parse operation binding the contract event 0xd75f94825e770b8b512be8e74759e252ad00e102e38f50cce2f7c6f868a29599.
//
// Solidity: event LogRemovedGovernor(address removedGovernor)
func (_Starknet *StarknetFilterer) ParseLogRemovedGovernor(log types.Log) (*StarknetLogRemovedGovernor, error) {
	event := new(StarknetLogRemovedGovernor)
	if err := _Starknet.contract.UnpackLog(event, "LogRemovedGovernor", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogStateTransitionFactIterator is returned from FilterLogStateTransitionFact and is used to iterate over the raw logs and unpacked data for LogStateTransitionFact events raised by the Starknet contract.
type StarknetLogStateTransitionFactIterator struct {
	Event *StarknetLogStateTransitionFact // Event containing the contract specifics and raw log

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
func (it *StarknetLogStateTransitionFactIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogStateTransitionFact)
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
		it.Event = new(StarknetLogStateTransitionFact)
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
func (it *StarknetLogStateTransitionFactIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogStateTransitionFactIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogStateTransitionFact represents a LogStateTransitionFact event raised by the Starknet contract.
type StarknetLogStateTransitionFact struct {
	StateTransitionFact [32]byte
	Raw                 types.Log // Blockchain specific contextual infos
}

// FilterLogStateTransitionFact is a free log retrieval operation binding the contract event 0x9866f8ddfe70bb512b2f2b28b49d4017c43f7ba775f1a20c61c13eea8cdac111.
//
// Solidity: event LogStateTransitionFact(bytes32 stateTransitionFact)
func (_Starknet *StarknetFilterer) FilterLogStateTransitionFact(opts *bind.FilterOpts) (*StarknetLogStateTransitionFactIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogStateTransitionFact")
	if err != nil {
		return nil, err
	}
	return &StarknetLogStateTransitionFactIterator{contract: _Starknet.contract, event: "LogStateTransitionFact", logs: logs, sub: sub}, nil
}

// WatchLogStateTransitionFact is a free log subscription operation binding the contract event 0x9866f8ddfe70bb512b2f2b28b49d4017c43f7ba775f1a20c61c13eea8cdac111.
//
// Solidity: event LogStateTransitionFact(bytes32 stateTransitionFact)
func (_Starknet *StarknetFilterer) WatchLogStateTransitionFact(opts *bind.WatchOpts, sink chan<- *StarknetLogStateTransitionFact) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogStateTransitionFact")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogStateTransitionFact)
				if err := _Starknet.contract.UnpackLog(event, "LogStateTransitionFact", log); err != nil {
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

// ParseLogStateTransitionFact is a log parse operation binding the contract event 0x9866f8ddfe70bb512b2f2b28b49d4017c43f7ba775f1a20c61c13eea8cdac111.
//
// Solidity: event LogStateTransitionFact(bytes32 stateTransitionFact)
func (_Starknet *StarknetFilterer) ParseLogStateTransitionFact(log types.Log) (*StarknetLogStateTransitionFact, error) {
	event := new(StarknetLogStateTransitionFact)
	if err := _Starknet.contract.UnpackLog(event, "LogStateTransitionFact", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetLogStateUpdateIterator is returned from FilterLogStateUpdate and is used to iterate over the raw logs and unpacked data for LogStateUpdate events raised by the Starknet contract.
type StarknetLogStateUpdateIterator struct {
	Event *StarknetLogStateUpdate // Event containing the contract specifics and raw log

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
func (it *StarknetLogStateUpdateIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetLogStateUpdate)
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
		it.Event = new(StarknetLogStateUpdate)
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
func (it *StarknetLogStateUpdateIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetLogStateUpdateIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetLogStateUpdate represents a LogStateUpdate event raised by the Starknet contract.
type StarknetLogStateUpdate struct {
	GlobalRoot  *big.Int
	BlockNumber *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterLogStateUpdate is a free log retrieval operation binding the contract event 0xe8012213bb931d3efa0a954cfb0d7b75f2a5e2358ba5f7d3edfb0154f6e7a568.
//
// Solidity: event LogStateUpdate(uint256 globalRoot, int256 blockNumber)
func (_Starknet *StarknetFilterer) FilterLogStateUpdate(opts *bind.FilterOpts) (*StarknetLogStateUpdateIterator, error) {

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "LogStateUpdate")
	if err != nil {
		return nil, err
	}
	return &StarknetLogStateUpdateIterator{contract: _Starknet.contract, event: "LogStateUpdate", logs: logs, sub: sub}, nil
}

// WatchLogStateUpdate is a free log subscription operation binding the contract event 0xe8012213bb931d3efa0a954cfb0d7b75f2a5e2358ba5f7d3edfb0154f6e7a568.
//
// Solidity: event LogStateUpdate(uint256 globalRoot, int256 blockNumber)
func (_Starknet *StarknetFilterer) WatchLogStateUpdate(opts *bind.WatchOpts, sink chan<- *StarknetLogStateUpdate) (event.Subscription, error) {

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "LogStateUpdate")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetLogStateUpdate)
				if err := _Starknet.contract.UnpackLog(event, "LogStateUpdate", log); err != nil {
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

// ParseLogStateUpdate is a log parse operation binding the contract event 0xe8012213bb931d3efa0a954cfb0d7b75f2a5e2358ba5f7d3edfb0154f6e7a568.
//
// Solidity: event LogStateUpdate(uint256 globalRoot, int256 blockNumber)
func (_Starknet *StarknetFilterer) ParseLogStateUpdate(log types.Log) (*StarknetLogStateUpdate, error) {
	event := new(StarknetLogStateUpdate)
	if err := _Starknet.contract.UnpackLog(event, "LogStateUpdate", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetMessageToL2CanceledIterator is returned from FilterMessageToL2Canceled and is used to iterate over the raw logs and unpacked data for MessageToL2Canceled events raised by the Starknet contract.
type StarknetMessageToL2CanceledIterator struct {
	Event *StarknetMessageToL2Canceled // Event containing the contract specifics and raw log

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
func (it *StarknetMessageToL2CanceledIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetMessageToL2Canceled)
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
		it.Event = new(StarknetMessageToL2Canceled)
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
func (it *StarknetMessageToL2CanceledIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetMessageToL2CanceledIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetMessageToL2Canceled represents a MessageToL2Canceled event raised by the Starknet contract.
type StarknetMessageToL2Canceled struct {
	FromAddress common.Address
	ToAddress   *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Nonce       *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterMessageToL2Canceled is a free log retrieval operation binding the contract event 0x8abd2ec2e0a10c82f5b60ea00455fa96c41fd144f225fcc52b8d83d94f803ed8.
//
// Solidity: event MessageToL2Canceled(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) FilterMessageToL2Canceled(opts *bind.FilterOpts, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (*StarknetMessageToL2CanceledIterator, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "MessageToL2Canceled", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return &StarknetMessageToL2CanceledIterator{contract: _Starknet.contract, event: "MessageToL2Canceled", logs: logs, sub: sub}, nil
}

// WatchMessageToL2Canceled is a free log subscription operation binding the contract event 0x8abd2ec2e0a10c82f5b60ea00455fa96c41fd144f225fcc52b8d83d94f803ed8.
//
// Solidity: event MessageToL2Canceled(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) WatchMessageToL2Canceled(opts *bind.WatchOpts, sink chan<- *StarknetMessageToL2Canceled, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (event.Subscription, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "MessageToL2Canceled", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetMessageToL2Canceled)
				if err := _Starknet.contract.UnpackLog(event, "MessageToL2Canceled", log); err != nil {
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

// ParseMessageToL2Canceled is a log parse operation binding the contract event 0x8abd2ec2e0a10c82f5b60ea00455fa96c41fd144f225fcc52b8d83d94f803ed8.
//
// Solidity: event MessageToL2Canceled(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) ParseMessageToL2Canceled(log types.Log) (*StarknetMessageToL2Canceled, error) {
	event := new(StarknetMessageToL2Canceled)
	if err := _Starknet.contract.UnpackLog(event, "MessageToL2Canceled", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// StarknetMessageToL2CancellationStartedIterator is returned from FilterMessageToL2CancellationStarted and is used to iterate over the raw logs and unpacked data for MessageToL2CancellationStarted events raised by the Starknet contract.
type StarknetMessageToL2CancellationStartedIterator struct {
	Event *StarknetMessageToL2CancellationStarted // Event containing the contract specifics and raw log

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
func (it *StarknetMessageToL2CancellationStartedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(StarknetMessageToL2CancellationStarted)
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
		it.Event = new(StarknetMessageToL2CancellationStarted)
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
func (it *StarknetMessageToL2CancellationStartedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *StarknetMessageToL2CancellationStartedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// StarknetMessageToL2CancellationStarted represents a MessageToL2CancellationStarted event raised by the Starknet contract.
type StarknetMessageToL2CancellationStarted struct {
	FromAddress common.Address
	ToAddress   *big.Int
	Selector    *big.Int
	Payload     []*big.Int
	Nonce       *big.Int
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterMessageToL2CancellationStarted is a free log retrieval operation binding the contract event 0x2e00dccd686fd6823ec7dc3e125582aa82881b6ff5f6b5a73856e1ea8338a3be.
//
// Solidity: event MessageToL2CancellationStarted(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) FilterMessageToL2CancellationStarted(opts *bind.FilterOpts, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (*StarknetMessageToL2CancellationStartedIterator, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.FilterLogs(opts, "MessageToL2CancellationStarted", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return &StarknetMessageToL2CancellationStartedIterator{contract: _Starknet.contract, event: "MessageToL2CancellationStarted", logs: logs, sub: sub}, nil
}

// WatchMessageToL2CancellationStarted is a free log subscription operation binding the contract event 0x2e00dccd686fd6823ec7dc3e125582aa82881b6ff5f6b5a73856e1ea8338a3be.
//
// Solidity: event MessageToL2CancellationStarted(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) WatchMessageToL2CancellationStarted(opts *bind.WatchOpts, sink chan<- *StarknetMessageToL2CancellationStarted, fromAddress []common.Address, toAddress []*big.Int, selector []*big.Int) (event.Subscription, error) {

	var fromAddressRule []interface{}
	for _, fromAddressItem := range fromAddress {
		fromAddressRule = append(fromAddressRule, fromAddressItem)
	}
	var toAddressRule []interface{}
	for _, toAddressItem := range toAddress {
		toAddressRule = append(toAddressRule, toAddressItem)
	}
	var selectorRule []interface{}
	for _, selectorItem := range selector {
		selectorRule = append(selectorRule, selectorItem)
	}

	logs, sub, err := _Starknet.contract.WatchLogs(opts, "MessageToL2CancellationStarted", fromAddressRule, toAddressRule, selectorRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(StarknetMessageToL2CancellationStarted)
				if err := _Starknet.contract.UnpackLog(event, "MessageToL2CancellationStarted", log); err != nil {
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

// ParseMessageToL2CancellationStarted is a log parse operation binding the contract event 0x2e00dccd686fd6823ec7dc3e125582aa82881b6ff5f6b5a73856e1ea8338a3be.
//
// Solidity: event MessageToL2CancellationStarted(address indexed fromAddress, uint256 indexed toAddress, uint256 indexed selector, uint256[] payload, uint256 nonce)
func (_Starknet *StarknetFilterer) ParseMessageToL2CancellationStarted(log types.Log) (*StarknetMessageToL2CancellationStarted, error) {
	event := new(StarknetMessageToL2CancellationStarted)
	if err := _Starknet.contract.UnpackLog(event, "MessageToL2CancellationStarted", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
