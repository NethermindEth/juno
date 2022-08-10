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
}

// MemoryPageFactRegistryABI is the input ABI used to generate the binding from.
// Deprecated: Use MemoryPageFactRegistryMetaData.ABI instead.
var MemoryPageFactRegistryABI = MemoryPageFactRegistryMetaData.ABI

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
