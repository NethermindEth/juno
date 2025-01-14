// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

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
	_ = abi.ConvertType
)

// BootnodeRegistryMetaData contains all meta data concerning the BootnodeRegistry contract.
var BootnodeRegistryMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_updater\",\"type\":\"address\"}],\"name\":\"addAuthorizedUpdater\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_ipAddress\",\"type\":\"string\"}],\"name\":\"addIPAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ipAddress\",\"type\":\"string\"}],\"name\":\"IPAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ipAddress\",\"type\":\"string\"}],\"name\":\"IPRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_updater\",\"type\":\"address\"}],\"name\":\"removeAuthorizedUpdater\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"removeIPAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"updater\",\"type\":\"address\"}],\"name\":\"UpdaterAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"updater\",\"type\":\"address\"}],\"name\":\"UpdaterRemoved\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"getIPAddresses\",\"outputs\":[{\"internalType\":\"string[]\",\"name\":\"\",\"type\":\"string[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_updater\",\"type\":\"address\"}],\"name\":\"isAuthorizedUpdater\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// BootnodeRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use BootnodeRegistryMetaData.ABI instead.
var BootnodeRegistryABI = BootnodeRegistryMetaData.ABI

// BootnodeRegistry is an auto generated Go binding around an Ethereum contract.
type BootnodeRegistry struct {
	BootnodeRegistryCaller     // Read-only binding to the contract
	BootnodeRegistryTransactor // Write-only binding to the contract
	BootnodeRegistryFilterer   // Log filterer for contract events
}

// BootnodeRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type BootnodeRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BootnodeRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type BootnodeRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BootnodeRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type BootnodeRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// BootnodeRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type BootnodeRegistrySession struct {
	Contract     *BootnodeRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// BootnodeRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type BootnodeRegistryCallerSession struct {
	Contract *BootnodeRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts           // Call options to use throughout this session
}

// BootnodeRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type BootnodeRegistryTransactorSession struct {
	Contract     *BootnodeRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts           // Transaction auth options to use throughout this session
}

// BootnodeRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type BootnodeRegistryRaw struct {
	Contract *BootnodeRegistry // Generic contract binding to access the raw methods on
}

// BootnodeRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type BootnodeRegistryCallerRaw struct {
	Contract *BootnodeRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// BootnodeRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type BootnodeRegistryTransactorRaw struct {
	Contract *BootnodeRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewBootnodeRegistry creates a new instance of BootnodeRegistry, bound to a specific deployed contract.
func NewBootnodeRegistry(address common.Address, backend bind.ContractBackend) (*BootnodeRegistry, error) {
	contract, err := bindBootnodeRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistry{BootnodeRegistryCaller: BootnodeRegistryCaller{contract: contract}, BootnodeRegistryTransactor: BootnodeRegistryTransactor{contract: contract}, BootnodeRegistryFilterer: BootnodeRegistryFilterer{contract: contract}}, nil
}

// NewBootnodeRegistryCaller creates a new read-only instance of BootnodeRegistry, bound to a specific deployed contract.
func NewBootnodeRegistryCaller(address common.Address, caller bind.ContractCaller) (*BootnodeRegistryCaller, error) {
	contract, err := bindBootnodeRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryCaller{contract: contract}, nil
}

// NewBootnodeRegistryTransactor creates a new write-only instance of BootnodeRegistry, bound to a specific deployed contract.
func NewBootnodeRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*BootnodeRegistryTransactor, error) {
	contract, err := bindBootnodeRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryTransactor{contract: contract}, nil
}

// NewBootnodeRegistryFilterer creates a new log filterer instance of BootnodeRegistry, bound to a specific deployed contract.
func NewBootnodeRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*BootnodeRegistryFilterer, error) {
	contract, err := bindBootnodeRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryFilterer{contract: contract}, nil
}

// bindBootnodeRegistry binds a generic wrapper to an already deployed contract.
func bindBootnodeRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := BootnodeRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BootnodeRegistry *BootnodeRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BootnodeRegistry.Contract.BootnodeRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BootnodeRegistry *BootnodeRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.BootnodeRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BootnodeRegistry *BootnodeRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.BootnodeRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_BootnodeRegistry *BootnodeRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _BootnodeRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_BootnodeRegistry *BootnodeRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_BootnodeRegistry *BootnodeRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.contract.Transact(opts, method, params...)
}

// GetIPAddresses is a free data retrieval call binding the contract method 0xd42c014a.
//
// Solidity: function getIPAddresses() view returns(string[])
func (_BootnodeRegistry *BootnodeRegistryCaller) GetIPAddresses(opts *bind.CallOpts) ([]string, error) {
	var out []interface{}
	err := _BootnodeRegistry.contract.Call(opts, &out, "getIPAddresses")

	if err != nil {
		return *new([]string), err
	}

	out0 := *abi.ConvertType(out[0], new([]string)).(*[]string)

	return out0, err

}

// GetIPAddresses is a free data retrieval call binding the contract method 0xd42c014a.
//
// Solidity: function getIPAddresses() view returns(string[])
func (_BootnodeRegistry *BootnodeRegistrySession) GetIPAddresses() ([]string, error) {
	return _BootnodeRegistry.Contract.GetIPAddresses(&_BootnodeRegistry.CallOpts)
}

// GetIPAddresses is a free data retrieval call binding the contract method 0xd42c014a.
//
// Solidity: function getIPAddresses() view returns(string[])
func (_BootnodeRegistry *BootnodeRegistryCallerSession) GetIPAddresses() ([]string, error) {
	return _BootnodeRegistry.Contract.GetIPAddresses(&_BootnodeRegistry.CallOpts)
}

// IsAuthorizedUpdater is a free data retrieval call binding the contract method 0xb865bccc.
//
// Solidity: function isAuthorizedUpdater(address _updater) view returns(bool)
func (_BootnodeRegistry *BootnodeRegistryCaller) IsAuthorizedUpdater(opts *bind.CallOpts, _updater common.Address) (bool, error) {
	var out []interface{}
	err := _BootnodeRegistry.contract.Call(opts, &out, "isAuthorizedUpdater", _updater)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsAuthorizedUpdater is a free data retrieval call binding the contract method 0xb865bccc.
//
// Solidity: function isAuthorizedUpdater(address _updater) view returns(bool)
func (_BootnodeRegistry *BootnodeRegistrySession) IsAuthorizedUpdater(_updater common.Address) (bool, error) {
	return _BootnodeRegistry.Contract.IsAuthorizedUpdater(&_BootnodeRegistry.CallOpts, _updater)
}

// IsAuthorizedUpdater is a free data retrieval call binding the contract method 0xb865bccc.
//
// Solidity: function isAuthorizedUpdater(address _updater) view returns(bool)
func (_BootnodeRegistry *BootnodeRegistryCallerSession) IsAuthorizedUpdater(_updater common.Address) (bool, error) {
	return _BootnodeRegistry.Contract.IsAuthorizedUpdater(&_BootnodeRegistry.CallOpts, _updater)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_BootnodeRegistry *BootnodeRegistryCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _BootnodeRegistry.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_BootnodeRegistry *BootnodeRegistrySession) Owner() (common.Address, error) {
	return _BootnodeRegistry.Contract.Owner(&_BootnodeRegistry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_BootnodeRegistry *BootnodeRegistryCallerSession) Owner() (common.Address, error) {
	return _BootnodeRegistry.Contract.Owner(&_BootnodeRegistry.CallOpts)
}

// AddAuthorizedUpdater is a paid mutator transaction binding the contract method 0x8c9b9fdc.
//
// Solidity: function addAuthorizedUpdater(address _updater) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactor) AddAuthorizedUpdater(opts *bind.TransactOpts, _updater common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.contract.Transact(opts, "addAuthorizedUpdater", _updater)
}

// AddAuthorizedUpdater is a paid mutator transaction binding the contract method 0x8c9b9fdc.
//
// Solidity: function addAuthorizedUpdater(address _updater) returns()
func (_BootnodeRegistry *BootnodeRegistrySession) AddAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.AddAuthorizedUpdater(&_BootnodeRegistry.TransactOpts, _updater)
}

// AddAuthorizedUpdater is a paid mutator transaction binding the contract method 0x8c9b9fdc.
//
// Solidity: function addAuthorizedUpdater(address _updater) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactorSession) AddAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.AddAuthorizedUpdater(&_BootnodeRegistry.TransactOpts, _updater)
}

// AddIPAddress is a paid mutator transaction binding the contract method 0x90139fa0.
//
// Solidity: function addIPAddress(string _ipAddress) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactor) AddIPAddress(opts *bind.TransactOpts, _ipAddress string) (*types.Transaction, error) {
	return _BootnodeRegistry.contract.Transact(opts, "addIPAddress", _ipAddress)
}

// AddIPAddress is a paid mutator transaction binding the contract method 0x90139fa0.
//
// Solidity: function addIPAddress(string _ipAddress) returns()
func (_BootnodeRegistry *BootnodeRegistrySession) AddIPAddress(_ipAddress string) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.AddIPAddress(&_BootnodeRegistry.TransactOpts, _ipAddress)
}

// AddIPAddress is a paid mutator transaction binding the contract method 0x90139fa0.
//
// Solidity: function addIPAddress(string _ipAddress) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactorSession) AddIPAddress(_ipAddress string) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.AddIPAddress(&_BootnodeRegistry.TransactOpts, _ipAddress)
}

// RemoveAuthorizedUpdater is a paid mutator transaction binding the contract method 0x603cda09.
//
// Solidity: function removeAuthorizedUpdater(address _updater) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactor) RemoveAuthorizedUpdater(opts *bind.TransactOpts, _updater common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.contract.Transact(opts, "removeAuthorizedUpdater", _updater)
}

// RemoveAuthorizedUpdater is a paid mutator transaction binding the contract method 0x603cda09.
//
// Solidity: function removeAuthorizedUpdater(address _updater) returns()
func (_BootnodeRegistry *BootnodeRegistrySession) RemoveAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.RemoveAuthorizedUpdater(&_BootnodeRegistry.TransactOpts, _updater)
}

// RemoveAuthorizedUpdater is a paid mutator transaction binding the contract method 0x603cda09.
//
// Solidity: function removeAuthorizedUpdater(address _updater) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactorSession) RemoveAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.RemoveAuthorizedUpdater(&_BootnodeRegistry.TransactOpts, _updater)
}

// RemoveIPAddress is a paid mutator transaction binding the contract method 0xba14bc71.
//
// Solidity: function removeIPAddress(uint256 index) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactor) RemoveIPAddress(opts *bind.TransactOpts, index *big.Int) (*types.Transaction, error) {
	return _BootnodeRegistry.contract.Transact(opts, "removeIPAddress", index)
}

// RemoveIPAddress is a paid mutator transaction binding the contract method 0xba14bc71.
//
// Solidity: function removeIPAddress(uint256 index) returns()
func (_BootnodeRegistry *BootnodeRegistrySession) RemoveIPAddress(index *big.Int) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.RemoveIPAddress(&_BootnodeRegistry.TransactOpts, index)
}

// RemoveIPAddress is a paid mutator transaction binding the contract method 0xba14bc71.
//
// Solidity: function removeIPAddress(uint256 index) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactorSession) RemoveIPAddress(index *big.Int) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.RemoveIPAddress(&_BootnodeRegistry.TransactOpts, index)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_BootnodeRegistry *BootnodeRegistryTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _BootnodeRegistry.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_BootnodeRegistry *BootnodeRegistrySession) RenounceOwnership() (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.RenounceOwnership(&_BootnodeRegistry.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_BootnodeRegistry *BootnodeRegistryTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.RenounceOwnership(&_BootnodeRegistry.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_BootnodeRegistry *BootnodeRegistrySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.TransferOwnership(&_BootnodeRegistry.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_BootnodeRegistry *BootnodeRegistryTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _BootnodeRegistry.Contract.TransferOwnership(&_BootnodeRegistry.TransactOpts, newOwner)
}

// BootnodeRegistryIPAddedIterator is returned from FilterIPAdded and is used to iterate over the raw logs and unpacked data for IPAdded events raised by the BootnodeRegistry contract.
type BootnodeRegistryIPAddedIterator struct {
	Event *BootnodeRegistryIPAdded // Event containing the contract specifics and raw log

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
func (it *BootnodeRegistryIPAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BootnodeRegistryIPAdded)
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
		it.Event = new(BootnodeRegistryIPAdded)
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
func (it *BootnodeRegistryIPAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BootnodeRegistryIPAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BootnodeRegistryIPAdded represents a IPAdded event raised by the BootnodeRegistry contract.
type BootnodeRegistryIPAdded struct {
	IpAddress string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIPAdded is a free log retrieval operation binding the contract event 0xe7f7a48f1891a089b5b0418c215a3fdc894029f208c5b5930473161f39ae988b.
//
// Solidity: event IPAdded(string ipAddress)
func (_BootnodeRegistry *BootnodeRegistryFilterer) FilterIPAdded(opts *bind.FilterOpts) (*BootnodeRegistryIPAddedIterator, error) {

	logs, sub, err := _BootnodeRegistry.contract.FilterLogs(opts, "IPAdded")
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryIPAddedIterator{contract: _BootnodeRegistry.contract, event: "IPAdded", logs: logs, sub: sub}, nil
}

// WatchIPAdded is a free log subscription operation binding the contract event 0xe7f7a48f1891a089b5b0418c215a3fdc894029f208c5b5930473161f39ae988b.
//
// Solidity: event IPAdded(string ipAddress)
func (_BootnodeRegistry *BootnodeRegistryFilterer) WatchIPAdded(opts *bind.WatchOpts, sink chan<- *BootnodeRegistryIPAdded) (event.Subscription, error) {

	logs, sub, err := _BootnodeRegistry.contract.WatchLogs(opts, "IPAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BootnodeRegistryIPAdded)
				if err := _BootnodeRegistry.contract.UnpackLog(event, "IPAdded", log); err != nil {
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

// ParseIPAdded is a log parse operation binding the contract event 0xe7f7a48f1891a089b5b0418c215a3fdc894029f208c5b5930473161f39ae988b.
//
// Solidity: event IPAdded(string ipAddress)
func (_BootnodeRegistry *BootnodeRegistryFilterer) ParseIPAdded(log types.Log) (*BootnodeRegistryIPAdded, error) {
	event := new(BootnodeRegistryIPAdded)
	if err := _BootnodeRegistry.contract.UnpackLog(event, "IPAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BootnodeRegistryIPRemovedIterator is returned from FilterIPRemoved and is used to iterate over the raw logs and unpacked data for IPRemoved events raised by the BootnodeRegistry contract.
type BootnodeRegistryIPRemovedIterator struct {
	Event *BootnodeRegistryIPRemoved // Event containing the contract specifics and raw log

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
func (it *BootnodeRegistryIPRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BootnodeRegistryIPRemoved)
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
		it.Event = new(BootnodeRegistryIPRemoved)
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
func (it *BootnodeRegistryIPRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BootnodeRegistryIPRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BootnodeRegistryIPRemoved represents a IPRemoved event raised by the BootnodeRegistry contract.
type BootnodeRegistryIPRemoved struct {
	IpAddress string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIPRemoved is a free log retrieval operation binding the contract event 0x45fe66c64cad3093171b605f5ffe092b5333c407560ee34f49a9096c6b312c4f.
//
// Solidity: event IPRemoved(string ipAddress)
func (_BootnodeRegistry *BootnodeRegistryFilterer) FilterIPRemoved(opts *bind.FilterOpts) (*BootnodeRegistryIPRemovedIterator, error) {

	logs, sub, err := _BootnodeRegistry.contract.FilterLogs(opts, "IPRemoved")
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryIPRemovedIterator{contract: _BootnodeRegistry.contract, event: "IPRemoved", logs: logs, sub: sub}, nil
}

// WatchIPRemoved is a free log subscription operation binding the contract event 0x45fe66c64cad3093171b605f5ffe092b5333c407560ee34f49a9096c6b312c4f.
//
// Solidity: event IPRemoved(string ipAddress)
func (_BootnodeRegistry *BootnodeRegistryFilterer) WatchIPRemoved(opts *bind.WatchOpts, sink chan<- *BootnodeRegistryIPRemoved) (event.Subscription, error) {

	logs, sub, err := _BootnodeRegistry.contract.WatchLogs(opts, "IPRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BootnodeRegistryIPRemoved)
				if err := _BootnodeRegistry.contract.UnpackLog(event, "IPRemoved", log); err != nil {
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

// ParseIPRemoved is a log parse operation binding the contract event 0x45fe66c64cad3093171b605f5ffe092b5333c407560ee34f49a9096c6b312c4f.
//
// Solidity: event IPRemoved(string ipAddress)
func (_BootnodeRegistry *BootnodeRegistryFilterer) ParseIPRemoved(log types.Log) (*BootnodeRegistryIPRemoved, error) {
	event := new(BootnodeRegistryIPRemoved)
	if err := _BootnodeRegistry.contract.UnpackLog(event, "IPRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BootnodeRegistryOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the BootnodeRegistry contract.
type BootnodeRegistryOwnershipTransferredIterator struct {
	Event *BootnodeRegistryOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *BootnodeRegistryOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BootnodeRegistryOwnershipTransferred)
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
		it.Event = new(BootnodeRegistryOwnershipTransferred)
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
func (it *BootnodeRegistryOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BootnodeRegistryOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BootnodeRegistryOwnershipTransferred represents a OwnershipTransferred event raised by the BootnodeRegistry contract.
type BootnodeRegistryOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_BootnodeRegistry *BootnodeRegistryFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*BootnodeRegistryOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _BootnodeRegistry.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryOwnershipTransferredIterator{contract: _BootnodeRegistry.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_BootnodeRegistry *BootnodeRegistryFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *BootnodeRegistryOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _BootnodeRegistry.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BootnodeRegistryOwnershipTransferred)
				if err := _BootnodeRegistry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_BootnodeRegistry *BootnodeRegistryFilterer) ParseOwnershipTransferred(log types.Log) (*BootnodeRegistryOwnershipTransferred, error) {
	event := new(BootnodeRegistryOwnershipTransferred)
	if err := _BootnodeRegistry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BootnodeRegistryUpdaterAddedIterator is returned from FilterUpdaterAdded and is used to iterate over the raw logs and unpacked data for UpdaterAdded events raised by the BootnodeRegistry contract.
type BootnodeRegistryUpdaterAddedIterator struct {
	Event *BootnodeRegistryUpdaterAdded // Event containing the contract specifics and raw log

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
func (it *BootnodeRegistryUpdaterAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BootnodeRegistryUpdaterAdded)
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
		it.Event = new(BootnodeRegistryUpdaterAdded)
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
func (it *BootnodeRegistryUpdaterAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BootnodeRegistryUpdaterAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BootnodeRegistryUpdaterAdded represents a UpdaterAdded event raised by the BootnodeRegistry contract.
type BootnodeRegistryUpdaterAdded struct {
	Updater common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUpdaterAdded is a free log retrieval operation binding the contract event 0x23a38f89c31ff6329bf86f3863cfa2ad8fc1462c40dbf907dbbebb8f9cb237ec.
//
// Solidity: event UpdaterAdded(address updater)
func (_BootnodeRegistry *BootnodeRegistryFilterer) FilterUpdaterAdded(opts *bind.FilterOpts) (*BootnodeRegistryUpdaterAddedIterator, error) {

	logs, sub, err := _BootnodeRegistry.contract.FilterLogs(opts, "UpdaterAdded")
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryUpdaterAddedIterator{contract: _BootnodeRegistry.contract, event: "UpdaterAdded", logs: logs, sub: sub}, nil
}

// WatchUpdaterAdded is a free log subscription operation binding the contract event 0x23a38f89c31ff6329bf86f3863cfa2ad8fc1462c40dbf907dbbebb8f9cb237ec.
//
// Solidity: event UpdaterAdded(address updater)
func (_BootnodeRegistry *BootnodeRegistryFilterer) WatchUpdaterAdded(opts *bind.WatchOpts, sink chan<- *BootnodeRegistryUpdaterAdded) (event.Subscription, error) {

	logs, sub, err := _BootnodeRegistry.contract.WatchLogs(opts, "UpdaterAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BootnodeRegistryUpdaterAdded)
				if err := _BootnodeRegistry.contract.UnpackLog(event, "UpdaterAdded", log); err != nil {
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

// ParseUpdaterAdded is a log parse operation binding the contract event 0x23a38f89c31ff6329bf86f3863cfa2ad8fc1462c40dbf907dbbebb8f9cb237ec.
//
// Solidity: event UpdaterAdded(address updater)
func (_BootnodeRegistry *BootnodeRegistryFilterer) ParseUpdaterAdded(log types.Log) (*BootnodeRegistryUpdaterAdded, error) {
	event := new(BootnodeRegistryUpdaterAdded)
	if err := _BootnodeRegistry.contract.UnpackLog(event, "UpdaterAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// BootnodeRegistryUpdaterRemovedIterator is returned from FilterUpdaterRemoved and is used to iterate over the raw logs and unpacked data for UpdaterRemoved events raised by the BootnodeRegistry contract.
type BootnodeRegistryUpdaterRemovedIterator struct {
	Event *BootnodeRegistryUpdaterRemoved // Event containing the contract specifics and raw log

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
func (it *BootnodeRegistryUpdaterRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(BootnodeRegistryUpdaterRemoved)
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
		it.Event = new(BootnodeRegistryUpdaterRemoved)
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
func (it *BootnodeRegistryUpdaterRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *BootnodeRegistryUpdaterRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// BootnodeRegistryUpdaterRemoved represents a UpdaterRemoved event raised by the BootnodeRegistry contract.
type BootnodeRegistryUpdaterRemoved struct {
	Updater common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUpdaterRemoved is a free log retrieval operation binding the contract event 0x209d819a9ec655e89f2b2b9d65c8a78879b45a8f20d1941d69c5fe6dc21bcb62.
//
// Solidity: event UpdaterRemoved(address updater)
func (_BootnodeRegistry *BootnodeRegistryFilterer) FilterUpdaterRemoved(opts *bind.FilterOpts) (*BootnodeRegistryUpdaterRemovedIterator, error) {

	logs, sub, err := _BootnodeRegistry.contract.FilterLogs(opts, "UpdaterRemoved")
	if err != nil {
		return nil, err
	}
	return &BootnodeRegistryUpdaterRemovedIterator{contract: _BootnodeRegistry.contract, event: "UpdaterRemoved", logs: logs, sub: sub}, nil
}

// WatchUpdaterRemoved is a free log subscription operation binding the contract event 0x209d819a9ec655e89f2b2b9d65c8a78879b45a8f20d1941d69c5fe6dc21bcb62.
//
// Solidity: event UpdaterRemoved(address updater)
func (_BootnodeRegistry *BootnodeRegistryFilterer) WatchUpdaterRemoved(opts *bind.WatchOpts, sink chan<- *BootnodeRegistryUpdaterRemoved) (event.Subscription, error) {

	logs, sub, err := _BootnodeRegistry.contract.WatchLogs(opts, "UpdaterRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(BootnodeRegistryUpdaterRemoved)
				if err := _BootnodeRegistry.contract.UnpackLog(event, "UpdaterRemoved", log); err != nil {
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

// ParseUpdaterRemoved is a log parse operation binding the contract event 0x209d819a9ec655e89f2b2b9d65c8a78879b45a8f20d1941d69c5fe6dc21bcb62.
//
// Solidity: event UpdaterRemoved(address updater)
func (_BootnodeRegistry *BootnodeRegistryFilterer) ParseUpdaterRemoved(log types.Log) (*BootnodeRegistryUpdaterRemoved, error) {
	event := new(BootnodeRegistryUpdaterRemoved)
	if err := _BootnodeRegistry.contract.UnpackLog(event, "UpdaterRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
