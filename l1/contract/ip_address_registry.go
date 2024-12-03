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

// IPAddressRegistryMetaData contains all meta data concerning the IPAddressRegistry contract.
var IPAddressRegistryMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_updater\",\"type\":\"address\"}],\"name\":\"addAuthorizedUpdater\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"_ipAddress\",\"type\":\"string\"}],\"name\":\"addIPAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"owner\",\"type\":\"address\"}],\"name\":\"OwnableInvalidOwner\",\"type\":\"error\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"account\",\"type\":\"address\"}],\"name\":\"OwnableUnauthorizedAccount\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ipAddress\",\"type\":\"string\"}],\"name\":\"IPAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"ipAddress\",\"type\":\"string\"}],\"name\":\"IPRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_updater\",\"type\":\"address\"}],\"name\":\"removeAuthorizedUpdater\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"index\",\"type\":\"uint256\"}],\"name\":\"removeIPAddress\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"updater\",\"type\":\"address\"}],\"name\":\"UpdaterAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"updater\",\"type\":\"address\"}],\"name\":\"UpdaterRemoved\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"getIPAddresses\",\"outputs\":[{\"internalType\":\"string[]\",\"name\":\"\",\"type\":\"string[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_updater\",\"type\":\"address\"}],\"name\":\"isAuthorizedUpdater\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// IPAddressRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use IPAddressRegistryMetaData.ABI instead.
var IPAddressRegistryABI = IPAddressRegistryMetaData.ABI

// IPAddressRegistry is an auto generated Go binding around an Ethereum contract.
type IPAddressRegistry struct {
	IPAddressRegistryCaller     // Read-only binding to the contract
	IPAddressRegistryTransactor // Write-only binding to the contract
	IPAddressRegistryFilterer   // Log filterer for contract events
}

// IPAddressRegistryCaller is an auto generated read-only Go binding around an Ethereum contract.
type IPAddressRegistryCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IPAddressRegistryTransactor is an auto generated write-only Go binding around an Ethereum contract.
type IPAddressRegistryTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IPAddressRegistryFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type IPAddressRegistryFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// IPAddressRegistrySession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type IPAddressRegistrySession struct {
	Contract     *IPAddressRegistry // Generic contract binding to set the session for
	CallOpts     bind.CallOpts      // Call options to use throughout this session
	TransactOpts bind.TransactOpts  // Transaction auth options to use throughout this session
}

// IPAddressRegistryCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type IPAddressRegistryCallerSession struct {
	Contract *IPAddressRegistryCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts            // Call options to use throughout this session
}

// IPAddressRegistryTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type IPAddressRegistryTransactorSession struct {
	Contract     *IPAddressRegistryTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts            // Transaction auth options to use throughout this session
}

// IPAddressRegistryRaw is an auto generated low-level Go binding around an Ethereum contract.
type IPAddressRegistryRaw struct {
	Contract *IPAddressRegistry // Generic contract binding to access the raw methods on
}

// IPAddressRegistryCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type IPAddressRegistryCallerRaw struct {
	Contract *IPAddressRegistryCaller // Generic read-only contract binding to access the raw methods on
}

// IPAddressRegistryTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type IPAddressRegistryTransactorRaw struct {
	Contract *IPAddressRegistryTransactor // Generic write-only contract binding to access the raw methods on
}

// NewIPAddressRegistry creates a new instance of IPAddressRegistry, bound to a specific deployed contract.
func NewIPAddressRegistry(address common.Address, backend bind.ContractBackend) (*IPAddressRegistry, error) {
	contract, err := bindIPAddressRegistry(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistry{IPAddressRegistryCaller: IPAddressRegistryCaller{contract: contract}, IPAddressRegistryTransactor: IPAddressRegistryTransactor{contract: contract}, IPAddressRegistryFilterer: IPAddressRegistryFilterer{contract: contract}}, nil
}

// NewIPAddressRegistryCaller creates a new read-only instance of IPAddressRegistry, bound to a specific deployed contract.
func NewIPAddressRegistryCaller(address common.Address, caller bind.ContractCaller) (*IPAddressRegistryCaller, error) {
	contract, err := bindIPAddressRegistry(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryCaller{contract: contract}, nil
}

// NewIPAddressRegistryTransactor creates a new write-only instance of IPAddressRegistry, bound to a specific deployed contract.
func NewIPAddressRegistryTransactor(address common.Address, transactor bind.ContractTransactor) (*IPAddressRegistryTransactor, error) {
	contract, err := bindIPAddressRegistry(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryTransactor{contract: contract}, nil
}

// NewIPAddressRegistryFilterer creates a new log filterer instance of IPAddressRegistry, bound to a specific deployed contract.
func NewIPAddressRegistryFilterer(address common.Address, filterer bind.ContractFilterer) (*IPAddressRegistryFilterer, error) {
	contract, err := bindIPAddressRegistry(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryFilterer{contract: contract}, nil
}

// bindIPAddressRegistry binds a generic wrapper to an already deployed contract.
func bindIPAddressRegistry(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := IPAddressRegistryMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IPAddressRegistry *IPAddressRegistryRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IPAddressRegistry.Contract.IPAddressRegistryCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IPAddressRegistry *IPAddressRegistryRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.IPAddressRegistryTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IPAddressRegistry *IPAddressRegistryRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.IPAddressRegistryTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_IPAddressRegistry *IPAddressRegistryCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _IPAddressRegistry.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_IPAddressRegistry *IPAddressRegistryTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_IPAddressRegistry *IPAddressRegistryTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.contract.Transact(opts, method, params...)
}

// GetIPAddresses is a free data retrieval call binding the contract method 0xd42c014a.
//
// Solidity: function getIPAddresses() view returns(string[])
func (_IPAddressRegistry *IPAddressRegistryCaller) GetIPAddresses(opts *bind.CallOpts) ([]string, error) {
	var out []interface{}
	err := _IPAddressRegistry.contract.Call(opts, &out, "getIPAddresses")

	if err != nil {
		return *new([]string), err
	}

	out0 := *abi.ConvertType(out[0], new([]string)).(*[]string)

	return out0, err

}

// GetIPAddresses is a free data retrieval call binding the contract method 0xd42c014a.
//
// Solidity: function getIPAddresses() view returns(string[])
func (_IPAddressRegistry *IPAddressRegistrySession) GetIPAddresses() ([]string, error) {
	return _IPAddressRegistry.Contract.GetIPAddresses(&_IPAddressRegistry.CallOpts)
}

// GetIPAddresses is a free data retrieval call binding the contract method 0xd42c014a.
//
// Solidity: function getIPAddresses() view returns(string[])
func (_IPAddressRegistry *IPAddressRegistryCallerSession) GetIPAddresses() ([]string, error) {
	return _IPAddressRegistry.Contract.GetIPAddresses(&_IPAddressRegistry.CallOpts)
}

// IsAuthorizedUpdater is a free data retrieval call binding the contract method 0xb865bccc.
//
// Solidity: function isAuthorizedUpdater(address _updater) view returns(bool)
func (_IPAddressRegistry *IPAddressRegistryCaller) IsAuthorizedUpdater(opts *bind.CallOpts, _updater common.Address) (bool, error) {
	var out []interface{}
	err := _IPAddressRegistry.contract.Call(opts, &out, "isAuthorizedUpdater", _updater)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsAuthorizedUpdater is a free data retrieval call binding the contract method 0xb865bccc.
//
// Solidity: function isAuthorizedUpdater(address _updater) view returns(bool)
func (_IPAddressRegistry *IPAddressRegistrySession) IsAuthorizedUpdater(_updater common.Address) (bool, error) {
	return _IPAddressRegistry.Contract.IsAuthorizedUpdater(&_IPAddressRegistry.CallOpts, _updater)
}

// IsAuthorizedUpdater is a free data retrieval call binding the contract method 0xb865bccc.
//
// Solidity: function isAuthorizedUpdater(address _updater) view returns(bool)
func (_IPAddressRegistry *IPAddressRegistryCallerSession) IsAuthorizedUpdater(_updater common.Address) (bool, error) {
	return _IPAddressRegistry.Contract.IsAuthorizedUpdater(&_IPAddressRegistry.CallOpts, _updater)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_IPAddressRegistry *IPAddressRegistryCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _IPAddressRegistry.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_IPAddressRegistry *IPAddressRegistrySession) Owner() (common.Address, error) {
	return _IPAddressRegistry.Contract.Owner(&_IPAddressRegistry.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_IPAddressRegistry *IPAddressRegistryCallerSession) Owner() (common.Address, error) {
	return _IPAddressRegistry.Contract.Owner(&_IPAddressRegistry.CallOpts)
}

// AddAuthorizedUpdater is a paid mutator transaction binding the contract method 0x8c9b9fdc.
//
// Solidity: function addAuthorizedUpdater(address _updater) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactor) AddAuthorizedUpdater(opts *bind.TransactOpts, _updater common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.contract.Transact(opts, "addAuthorizedUpdater", _updater)
}

// AddAuthorizedUpdater is a paid mutator transaction binding the contract method 0x8c9b9fdc.
//
// Solidity: function addAuthorizedUpdater(address _updater) returns()
func (_IPAddressRegistry *IPAddressRegistrySession) AddAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.AddAuthorizedUpdater(&_IPAddressRegistry.TransactOpts, _updater)
}

// AddAuthorizedUpdater is a paid mutator transaction binding the contract method 0x8c9b9fdc.
//
// Solidity: function addAuthorizedUpdater(address _updater) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactorSession) AddAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.AddAuthorizedUpdater(&_IPAddressRegistry.TransactOpts, _updater)
}

// AddIPAddress is a paid mutator transaction binding the contract method 0x90139fa0.
//
// Solidity: function addIPAddress(string _ipAddress) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactor) AddIPAddress(opts *bind.TransactOpts, _ipAddress string) (*types.Transaction, error) {
	return _IPAddressRegistry.contract.Transact(opts, "addIPAddress", _ipAddress)
}

// AddIPAddress is a paid mutator transaction binding the contract method 0x90139fa0.
//
// Solidity: function addIPAddress(string _ipAddress) returns()
func (_IPAddressRegistry *IPAddressRegistrySession) AddIPAddress(_ipAddress string) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.AddIPAddress(&_IPAddressRegistry.TransactOpts, _ipAddress)
}

// AddIPAddress is a paid mutator transaction binding the contract method 0x90139fa0.
//
// Solidity: function addIPAddress(string _ipAddress) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactorSession) AddIPAddress(_ipAddress string) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.AddIPAddress(&_IPAddressRegistry.TransactOpts, _ipAddress)
}

// RemoveAuthorizedUpdater is a paid mutator transaction binding the contract method 0x603cda09.
//
// Solidity: function removeAuthorizedUpdater(address _updater) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactor) RemoveAuthorizedUpdater(opts *bind.TransactOpts, _updater common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.contract.Transact(opts, "removeAuthorizedUpdater", _updater)
}

// RemoveAuthorizedUpdater is a paid mutator transaction binding the contract method 0x603cda09.
//
// Solidity: function removeAuthorizedUpdater(address _updater) returns()
func (_IPAddressRegistry *IPAddressRegistrySession) RemoveAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.RemoveAuthorizedUpdater(&_IPAddressRegistry.TransactOpts, _updater)
}

// RemoveAuthorizedUpdater is a paid mutator transaction binding the contract method 0x603cda09.
//
// Solidity: function removeAuthorizedUpdater(address _updater) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactorSession) RemoveAuthorizedUpdater(_updater common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.RemoveAuthorizedUpdater(&_IPAddressRegistry.TransactOpts, _updater)
}

// RemoveIPAddress is a paid mutator transaction binding the contract method 0xba14bc71.
//
// Solidity: function removeIPAddress(uint256 index) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactor) RemoveIPAddress(opts *bind.TransactOpts, index *big.Int) (*types.Transaction, error) {
	return _IPAddressRegistry.contract.Transact(opts, "removeIPAddress", index)
}

// RemoveIPAddress is a paid mutator transaction binding the contract method 0xba14bc71.
//
// Solidity: function removeIPAddress(uint256 index) returns()
func (_IPAddressRegistry *IPAddressRegistrySession) RemoveIPAddress(index *big.Int) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.RemoveIPAddress(&_IPAddressRegistry.TransactOpts, index)
}

// RemoveIPAddress is a paid mutator transaction binding the contract method 0xba14bc71.
//
// Solidity: function removeIPAddress(uint256 index) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactorSession) RemoveIPAddress(index *big.Int) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.RemoveIPAddress(&_IPAddressRegistry.TransactOpts, index)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_IPAddressRegistry *IPAddressRegistryTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _IPAddressRegistry.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_IPAddressRegistry *IPAddressRegistrySession) RenounceOwnership() (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.RenounceOwnership(&_IPAddressRegistry.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_IPAddressRegistry *IPAddressRegistryTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.RenounceOwnership(&_IPAddressRegistry.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_IPAddressRegistry *IPAddressRegistrySession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.TransferOwnership(&_IPAddressRegistry.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_IPAddressRegistry *IPAddressRegistryTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _IPAddressRegistry.Contract.TransferOwnership(&_IPAddressRegistry.TransactOpts, newOwner)
}

// IPAddressRegistryIPAddedIterator is returned from FilterIPAdded and is used to iterate over the raw logs and unpacked data for IPAdded events raised by the IPAddressRegistry contract.
type IPAddressRegistryIPAddedIterator struct {
	Event *IPAddressRegistryIPAdded // Event containing the contract specifics and raw log

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
func (it *IPAddressRegistryIPAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IPAddressRegistryIPAdded)
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
		it.Event = new(IPAddressRegistryIPAdded)
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
func (it *IPAddressRegistryIPAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IPAddressRegistryIPAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IPAddressRegistryIPAdded represents a IPAdded event raised by the IPAddressRegistry contract.
type IPAddressRegistryIPAdded struct {
	IpAddress string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIPAdded is a free log retrieval operation binding the contract event 0xe7f7a48f1891a089b5b0418c215a3fdc894029f208c5b5930473161f39ae988b.
//
// Solidity: event IPAdded(string ipAddress)
func (_IPAddressRegistry *IPAddressRegistryFilterer) FilterIPAdded(opts *bind.FilterOpts) (*IPAddressRegistryIPAddedIterator, error) {

	logs, sub, err := _IPAddressRegistry.contract.FilterLogs(opts, "IPAdded")
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryIPAddedIterator{contract: _IPAddressRegistry.contract, event: "IPAdded", logs: logs, sub: sub}, nil
}

// WatchIPAdded is a free log subscription operation binding the contract event 0xe7f7a48f1891a089b5b0418c215a3fdc894029f208c5b5930473161f39ae988b.
//
// Solidity: event IPAdded(string ipAddress)
func (_IPAddressRegistry *IPAddressRegistryFilterer) WatchIPAdded(opts *bind.WatchOpts, sink chan<- *IPAddressRegistryIPAdded) (event.Subscription, error) {

	logs, sub, err := _IPAddressRegistry.contract.WatchLogs(opts, "IPAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IPAddressRegistryIPAdded)
				if err := _IPAddressRegistry.contract.UnpackLog(event, "IPAdded", log); err != nil {
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
func (_IPAddressRegistry *IPAddressRegistryFilterer) ParseIPAdded(log types.Log) (*IPAddressRegistryIPAdded, error) {
	event := new(IPAddressRegistryIPAdded)
	if err := _IPAddressRegistry.contract.UnpackLog(event, "IPAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IPAddressRegistryIPRemovedIterator is returned from FilterIPRemoved and is used to iterate over the raw logs and unpacked data for IPRemoved events raised by the IPAddressRegistry contract.
type IPAddressRegistryIPRemovedIterator struct {
	Event *IPAddressRegistryIPRemoved // Event containing the contract specifics and raw log

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
func (it *IPAddressRegistryIPRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IPAddressRegistryIPRemoved)
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
		it.Event = new(IPAddressRegistryIPRemoved)
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
func (it *IPAddressRegistryIPRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IPAddressRegistryIPRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IPAddressRegistryIPRemoved represents a IPRemoved event raised by the IPAddressRegistry contract.
type IPAddressRegistryIPRemoved struct {
	IpAddress string
	Raw       types.Log // Blockchain specific contextual infos
}

// FilterIPRemoved is a free log retrieval operation binding the contract event 0x45fe66c64cad3093171b605f5ffe092b5333c407560ee34f49a9096c6b312c4f.
//
// Solidity: event IPRemoved(string ipAddress)
func (_IPAddressRegistry *IPAddressRegistryFilterer) FilterIPRemoved(opts *bind.FilterOpts) (*IPAddressRegistryIPRemovedIterator, error) {

	logs, sub, err := _IPAddressRegistry.contract.FilterLogs(opts, "IPRemoved")
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryIPRemovedIterator{contract: _IPAddressRegistry.contract, event: "IPRemoved", logs: logs, sub: sub}, nil
}

// WatchIPRemoved is a free log subscription operation binding the contract event 0x45fe66c64cad3093171b605f5ffe092b5333c407560ee34f49a9096c6b312c4f.
//
// Solidity: event IPRemoved(string ipAddress)
func (_IPAddressRegistry *IPAddressRegistryFilterer) WatchIPRemoved(opts *bind.WatchOpts, sink chan<- *IPAddressRegistryIPRemoved) (event.Subscription, error) {

	logs, sub, err := _IPAddressRegistry.contract.WatchLogs(opts, "IPRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IPAddressRegistryIPRemoved)
				if err := _IPAddressRegistry.contract.UnpackLog(event, "IPRemoved", log); err != nil {
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
func (_IPAddressRegistry *IPAddressRegistryFilterer) ParseIPRemoved(log types.Log) (*IPAddressRegistryIPRemoved, error) {
	event := new(IPAddressRegistryIPRemoved)
	if err := _IPAddressRegistry.contract.UnpackLog(event, "IPRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IPAddressRegistryOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the IPAddressRegistry contract.
type IPAddressRegistryOwnershipTransferredIterator struct {
	Event *IPAddressRegistryOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *IPAddressRegistryOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IPAddressRegistryOwnershipTransferred)
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
		it.Event = new(IPAddressRegistryOwnershipTransferred)
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
func (it *IPAddressRegistryOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IPAddressRegistryOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IPAddressRegistryOwnershipTransferred represents a OwnershipTransferred event raised by the IPAddressRegistry contract.
type IPAddressRegistryOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_IPAddressRegistry *IPAddressRegistryFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*IPAddressRegistryOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _IPAddressRegistry.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryOwnershipTransferredIterator{contract: _IPAddressRegistry.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_IPAddressRegistry *IPAddressRegistryFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *IPAddressRegistryOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _IPAddressRegistry.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IPAddressRegistryOwnershipTransferred)
				if err := _IPAddressRegistry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_IPAddressRegistry *IPAddressRegistryFilterer) ParseOwnershipTransferred(log types.Log) (*IPAddressRegistryOwnershipTransferred, error) {
	event := new(IPAddressRegistryOwnershipTransferred)
	if err := _IPAddressRegistry.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IPAddressRegistryUpdaterAddedIterator is returned from FilterUpdaterAdded and is used to iterate over the raw logs and unpacked data for UpdaterAdded events raised by the IPAddressRegistry contract.
type IPAddressRegistryUpdaterAddedIterator struct {
	Event *IPAddressRegistryUpdaterAdded // Event containing the contract specifics and raw log

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
func (it *IPAddressRegistryUpdaterAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IPAddressRegistryUpdaterAdded)
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
		it.Event = new(IPAddressRegistryUpdaterAdded)
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
func (it *IPAddressRegistryUpdaterAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IPAddressRegistryUpdaterAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IPAddressRegistryUpdaterAdded represents a UpdaterAdded event raised by the IPAddressRegistry contract.
type IPAddressRegistryUpdaterAdded struct {
	Updater common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUpdaterAdded is a free log retrieval operation binding the contract event 0x23a38f89c31ff6329bf86f3863cfa2ad8fc1462c40dbf907dbbebb8f9cb237ec.
//
// Solidity: event UpdaterAdded(address updater)
func (_IPAddressRegistry *IPAddressRegistryFilterer) FilterUpdaterAdded(opts *bind.FilterOpts) (*IPAddressRegistryUpdaterAddedIterator, error) {

	logs, sub, err := _IPAddressRegistry.contract.FilterLogs(opts, "UpdaterAdded")
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryUpdaterAddedIterator{contract: _IPAddressRegistry.contract, event: "UpdaterAdded", logs: logs, sub: sub}, nil
}

// WatchUpdaterAdded is a free log subscription operation binding the contract event 0x23a38f89c31ff6329bf86f3863cfa2ad8fc1462c40dbf907dbbebb8f9cb237ec.
//
// Solidity: event UpdaterAdded(address updater)
func (_IPAddressRegistry *IPAddressRegistryFilterer) WatchUpdaterAdded(opts *bind.WatchOpts, sink chan<- *IPAddressRegistryUpdaterAdded) (event.Subscription, error) {

	logs, sub, err := _IPAddressRegistry.contract.WatchLogs(opts, "UpdaterAdded")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IPAddressRegistryUpdaterAdded)
				if err := _IPAddressRegistry.contract.UnpackLog(event, "UpdaterAdded", log); err != nil {
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
func (_IPAddressRegistry *IPAddressRegistryFilterer) ParseUpdaterAdded(log types.Log) (*IPAddressRegistryUpdaterAdded, error) {
	event := new(IPAddressRegistryUpdaterAdded)
	if err := _IPAddressRegistry.contract.UnpackLog(event, "UpdaterAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// IPAddressRegistryUpdaterRemovedIterator is returned from FilterUpdaterRemoved and is used to iterate over the raw logs and unpacked data for UpdaterRemoved events raised by the IPAddressRegistry contract.
type IPAddressRegistryUpdaterRemovedIterator struct {
	Event *IPAddressRegistryUpdaterRemoved // Event containing the contract specifics and raw log

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
func (it *IPAddressRegistryUpdaterRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(IPAddressRegistryUpdaterRemoved)
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
		it.Event = new(IPAddressRegistryUpdaterRemoved)
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
func (it *IPAddressRegistryUpdaterRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *IPAddressRegistryUpdaterRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// IPAddressRegistryUpdaterRemoved represents a UpdaterRemoved event raised by the IPAddressRegistry contract.
type IPAddressRegistryUpdaterRemoved struct {
	Updater common.Address
	Raw     types.Log // Blockchain specific contextual infos
}

// FilterUpdaterRemoved is a free log retrieval operation binding the contract event 0x209d819a9ec655e89f2b2b9d65c8a78879b45a8f20d1941d69c5fe6dc21bcb62.
//
// Solidity: event UpdaterRemoved(address updater)
func (_IPAddressRegistry *IPAddressRegistryFilterer) FilterUpdaterRemoved(opts *bind.FilterOpts) (*IPAddressRegistryUpdaterRemovedIterator, error) {

	logs, sub, err := _IPAddressRegistry.contract.FilterLogs(opts, "UpdaterRemoved")
	if err != nil {
		return nil, err
	}
	return &IPAddressRegistryUpdaterRemovedIterator{contract: _IPAddressRegistry.contract, event: "UpdaterRemoved", logs: logs, sub: sub}, nil
}

// WatchUpdaterRemoved is a free log subscription operation binding the contract event 0x209d819a9ec655e89f2b2b9d65c8a78879b45a8f20d1941d69c5fe6dc21bcb62.
//
// Solidity: event UpdaterRemoved(address updater)
func (_IPAddressRegistry *IPAddressRegistryFilterer) WatchUpdaterRemoved(opts *bind.WatchOpts, sink chan<- *IPAddressRegistryUpdaterRemoved) (event.Subscription, error) {

	logs, sub, err := _IPAddressRegistry.contract.WatchLogs(opts, "UpdaterRemoved")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(IPAddressRegistryUpdaterRemoved)
				if err := _IPAddressRegistry.contract.UnpackLog(event, "UpdaterRemoved", log); err != nil {
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
func (_IPAddressRegistry *IPAddressRegistryFilterer) ParseUpdaterRemoved(log types.Log) (*IPAddressRegistryUpdaterRemoved, error) {
	event := new(IPAddressRegistryUpdaterRemoved)
	if err := _IPAddressRegistry.contract.UnpackLog(event, "UpdaterRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
