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

// GpsStatementVerifierMetaData contains all meta data concerning the GpsStatementVerifier contract.
var GpsStatementVerifierMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"bootloaderProgramContract\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"memoryPageFactRegistry_\",\"type\":\"address\"},{\"internalType\":\"address[]\",\"name\":\"cairoVerifierContracts\",\"type\":\"address[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"bytes32\",\"name\":\"factHash\",\"type\":\"bytes32\"},{\"indexed\":false,\"internalType\":\"bytes32[]\",\"name\":\"pagesHashes\",\"type\":\"bytes32[]\"}],\"name\":\"LogMemoryPagesHashes\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"PAGE_INFO_ADDRESS_OFFSET\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PAGE_INFO_HASH_OFFSET\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PAGE_INFO_SIZE\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PAGE_INFO_SIZE_IN_BYTES\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"PAGE_INFO_SIZE_OFFSET\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"hasRegisteredFact\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"identify\",\"outputs\":[{\"internalType\":\"string\",\"name\":\"\",\"type\":\"string\"}],\"stateMutability\":\"pure\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fact\",\"type\":\"bytes32\"}],\"name\":\"isValid\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"proofParams\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"proof\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"taskMetadata\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"cairoAuxInput\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"cairoVerifierId\",\"type\":\"uint256\"}],\"name\":\"verifyProofAndRegister\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// GpsStatementVerifierABI is the input ABI used to generate the binding from.
// Deprecated: Use GpsStatementVerifierMetaData.ABI instead.
var GpsStatementVerifierABI = GpsStatementVerifierMetaData.ABI

// GpsStatementVerifier is an auto generated Go binding around an Ethereum contract.
type GpsStatementVerifier struct {
	GpsStatementVerifierCaller     // Read-only binding to the contract
	GpsStatementVerifierTransactor // Write-only binding to the contract
	GpsStatementVerifierFilterer   // Log filterer for contract events
}

// GpsStatementVerifierCaller is an auto generated read-only Go binding around an Ethereum contract.
type GpsStatementVerifierCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GpsStatementVerifierTransactor is an auto generated write-only Go binding around an Ethereum contract.
type GpsStatementVerifierTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GpsStatementVerifierFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type GpsStatementVerifierFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// GpsStatementVerifierSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type GpsStatementVerifierSession struct {
	Contract     *GpsStatementVerifier // Generic contract binding to set the session for
	CallOpts     bind.CallOpts         // Call options to use throughout this session
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// GpsStatementVerifierCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type GpsStatementVerifierCallerSession struct {
	Contract *GpsStatementVerifierCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts               // Call options to use throughout this session
}

// GpsStatementVerifierTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type GpsStatementVerifierTransactorSession struct {
	Contract     *GpsStatementVerifierTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts               // Transaction auth options to use throughout this session
}

// GpsStatementVerifierRaw is an auto generated low-level Go binding around an Ethereum contract.
type GpsStatementVerifierRaw struct {
	Contract *GpsStatementVerifier // Generic contract binding to access the raw methods on
}

// GpsStatementVerifierCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type GpsStatementVerifierCallerRaw struct {
	Contract *GpsStatementVerifierCaller // Generic read-only contract binding to access the raw methods on
}

// GpsStatementVerifierTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type GpsStatementVerifierTransactorRaw struct {
	Contract *GpsStatementVerifierTransactor // Generic write-only contract binding to access the raw methods on
}

// NewGpsStatementVerifier creates a new instance of GpsStatementVerifier, bound to a specific deployed contract.
func NewGpsStatementVerifier(address common.Address, backend bind.ContractBackend) (*GpsStatementVerifier, error) {
	contract, err := bindGpsStatementVerifier(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &GpsStatementVerifier{GpsStatementVerifierCaller: GpsStatementVerifierCaller{contract: contract}, GpsStatementVerifierTransactor: GpsStatementVerifierTransactor{contract: contract}, GpsStatementVerifierFilterer: GpsStatementVerifierFilterer{contract: contract}}, nil
}

// NewGpsStatementVerifierCaller creates a new read-only instance of GpsStatementVerifier, bound to a specific deployed contract.
func NewGpsStatementVerifierCaller(address common.Address, caller bind.ContractCaller) (*GpsStatementVerifierCaller, error) {
	contract, err := bindGpsStatementVerifier(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &GpsStatementVerifierCaller{contract: contract}, nil
}

// NewGpsStatementVerifierTransactor creates a new write-only instance of GpsStatementVerifier, bound to a specific deployed contract.
func NewGpsStatementVerifierTransactor(address common.Address, transactor bind.ContractTransactor) (*GpsStatementVerifierTransactor, error) {
	contract, err := bindGpsStatementVerifier(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &GpsStatementVerifierTransactor{contract: contract}, nil
}

// NewGpsStatementVerifierFilterer creates a new log filterer instance of GpsStatementVerifier, bound to a specific deployed contract.
func NewGpsStatementVerifierFilterer(address common.Address, filterer bind.ContractFilterer) (*GpsStatementVerifierFilterer, error) {
	contract, err := bindGpsStatementVerifier(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &GpsStatementVerifierFilterer{contract: contract}, nil
}

// bindGpsStatementVerifier binds a generic wrapper to an already deployed contract.
func bindGpsStatementVerifier(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(GpsStatementVerifierABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GpsStatementVerifier *GpsStatementVerifierRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GpsStatementVerifier.Contract.GpsStatementVerifierCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GpsStatementVerifier *GpsStatementVerifierRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GpsStatementVerifier.Contract.GpsStatementVerifierTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GpsStatementVerifier *GpsStatementVerifierRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GpsStatementVerifier.Contract.GpsStatementVerifierTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_GpsStatementVerifier *GpsStatementVerifierCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _GpsStatementVerifier.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_GpsStatementVerifier *GpsStatementVerifierTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _GpsStatementVerifier.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_GpsStatementVerifier *GpsStatementVerifierTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _GpsStatementVerifier.Contract.contract.Transact(opts, method, params...)
}

// PAGEINFOADDRESSOFFSET is a free data retrieval call binding the contract method 0xe5b62b29.
//
// Solidity: function PAGE_INFO_ADDRESS_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) PAGEINFOADDRESSOFFSET(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "PAGE_INFO_ADDRESS_OFFSET")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PAGEINFOADDRESSOFFSET is a free data retrieval call binding the contract method 0xe5b62b29.
//
// Solidity: function PAGE_INFO_ADDRESS_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierSession) PAGEINFOADDRESSOFFSET() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOADDRESSOFFSET(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOADDRESSOFFSET is a free data retrieval call binding the contract method 0xe5b62b29.
//
// Solidity: function PAGE_INFO_ADDRESS_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) PAGEINFOADDRESSOFFSET() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOADDRESSOFFSET(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOHASHOFFSET is a free data retrieval call binding the contract method 0x29e10520.
//
// Solidity: function PAGE_INFO_HASH_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) PAGEINFOHASHOFFSET(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "PAGE_INFO_HASH_OFFSET")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PAGEINFOHASHOFFSET is a free data retrieval call binding the contract method 0x29e10520.
//
// Solidity: function PAGE_INFO_HASH_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierSession) PAGEINFOHASHOFFSET() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOHASHOFFSET(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOHASHOFFSET is a free data retrieval call binding the contract method 0x29e10520.
//
// Solidity: function PAGE_INFO_HASH_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) PAGEINFOHASHOFFSET() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOHASHOFFSET(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOSIZE is a free data retrieval call binding the contract method 0x4c14a6f9.
//
// Solidity: function PAGE_INFO_SIZE() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) PAGEINFOSIZE(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "PAGE_INFO_SIZE")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PAGEINFOSIZE is a free data retrieval call binding the contract method 0x4c14a6f9.
//
// Solidity: function PAGE_INFO_SIZE() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierSession) PAGEINFOSIZE() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOSIZE(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOSIZE is a free data retrieval call binding the contract method 0x4c14a6f9.
//
// Solidity: function PAGE_INFO_SIZE() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) PAGEINFOSIZE() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOSIZE(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOSIZEINBYTES is a free data retrieval call binding the contract method 0xb7a771f7.
//
// Solidity: function PAGE_INFO_SIZE_IN_BYTES() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) PAGEINFOSIZEINBYTES(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "PAGE_INFO_SIZE_IN_BYTES")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PAGEINFOSIZEINBYTES is a free data retrieval call binding the contract method 0xb7a771f7.
//
// Solidity: function PAGE_INFO_SIZE_IN_BYTES() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierSession) PAGEINFOSIZEINBYTES() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOSIZEINBYTES(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOSIZEINBYTES is a free data retrieval call binding the contract method 0xb7a771f7.
//
// Solidity: function PAGE_INFO_SIZE_IN_BYTES() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) PAGEINFOSIZEINBYTES() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOSIZEINBYTES(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOSIZEOFFSET is a free data retrieval call binding the contract method 0x5b4c41c2.
//
// Solidity: function PAGE_INFO_SIZE_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) PAGEINFOSIZEOFFSET(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "PAGE_INFO_SIZE_OFFSET")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// PAGEINFOSIZEOFFSET is a free data retrieval call binding the contract method 0x5b4c41c2.
//
// Solidity: function PAGE_INFO_SIZE_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierSession) PAGEINFOSIZEOFFSET() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOSIZEOFFSET(&_GpsStatementVerifier.CallOpts)
}

// PAGEINFOSIZEOFFSET is a free data retrieval call binding the contract method 0x5b4c41c2.
//
// Solidity: function PAGE_INFO_SIZE_OFFSET() view returns(uint256)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) PAGEINFOSIZEOFFSET() (*big.Int, error) {
	return _GpsStatementVerifier.Contract.PAGEINFOSIZEOFFSET(&_GpsStatementVerifier.CallOpts)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) HasRegisteredFact(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "hasRegisteredFact")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_GpsStatementVerifier *GpsStatementVerifierSession) HasRegisteredFact() (bool, error) {
	return _GpsStatementVerifier.Contract.HasRegisteredFact(&_GpsStatementVerifier.CallOpts)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) HasRegisteredFact() (bool, error) {
	return _GpsStatementVerifier.Contract.HasRegisteredFact(&_GpsStatementVerifier.CallOpts)
}

// Identify is a free data retrieval call binding the contract method 0xeeb72866.
//
// Solidity: function identify() pure returns(string)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) Identify(opts *bind.CallOpts) (string, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "identify")

	if err != nil {
		return *new(string), err
	}

	out0 := *abi.ConvertType(out[0], new(string)).(*string)

	return out0, err

}

// Identify is a free data retrieval call binding the contract method 0xeeb72866.
//
// Solidity: function identify() pure returns(string)
func (_GpsStatementVerifier *GpsStatementVerifierSession) Identify() (string, error) {
	return _GpsStatementVerifier.Contract.Identify(&_GpsStatementVerifier.CallOpts)
}

// Identify is a free data retrieval call binding the contract method 0xeeb72866.
//
// Solidity: function identify() pure returns(string)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) Identify() (string, error) {
	return _GpsStatementVerifier.Contract.Identify(&_GpsStatementVerifier.CallOpts)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_GpsStatementVerifier *GpsStatementVerifierCaller) IsValid(opts *bind.CallOpts, fact [32]byte) (bool, error) {
	var out []interface{}
	err := _GpsStatementVerifier.contract.Call(opts, &out, "isValid", fact)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_GpsStatementVerifier *GpsStatementVerifierSession) IsValid(fact [32]byte) (bool, error) {
	return _GpsStatementVerifier.Contract.IsValid(&_GpsStatementVerifier.CallOpts, fact)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_GpsStatementVerifier *GpsStatementVerifierCallerSession) IsValid(fact [32]byte) (bool, error) {
	return _GpsStatementVerifier.Contract.IsValid(&_GpsStatementVerifier.CallOpts, fact)
}

// VerifyProofAndRegister is a paid mutator transaction binding the contract method 0x9b3b76cc.
//
// Solidity: function verifyProofAndRegister(uint256[] proofParams, uint256[] proof, uint256[] taskMetadata, uint256[] cairoAuxInput, uint256 cairoVerifierId) returns()
func (_GpsStatementVerifier *GpsStatementVerifierTransactor) VerifyProofAndRegister(opts *bind.TransactOpts, proofParams []*big.Int, proof []*big.Int, taskMetadata []*big.Int, cairoAuxInput []*big.Int, cairoVerifierId *big.Int) (*types.Transaction, error) {
	return _GpsStatementVerifier.contract.Transact(opts, "verifyProofAndRegister", proofParams, proof, taskMetadata, cairoAuxInput, cairoVerifierId)
}

// VerifyProofAndRegister is a paid mutator transaction binding the contract method 0x9b3b76cc.
//
// Solidity: function verifyProofAndRegister(uint256[] proofParams, uint256[] proof, uint256[] taskMetadata, uint256[] cairoAuxInput, uint256 cairoVerifierId) returns()
func (_GpsStatementVerifier *GpsStatementVerifierSession) VerifyProofAndRegister(proofParams []*big.Int, proof []*big.Int, taskMetadata []*big.Int, cairoAuxInput []*big.Int, cairoVerifierId *big.Int) (*types.Transaction, error) {
	return _GpsStatementVerifier.Contract.VerifyProofAndRegister(&_GpsStatementVerifier.TransactOpts, proofParams, proof, taskMetadata, cairoAuxInput, cairoVerifierId)
}

// VerifyProofAndRegister is a paid mutator transaction binding the contract method 0x9b3b76cc.
//
// Solidity: function verifyProofAndRegister(uint256[] proofParams, uint256[] proof, uint256[] taskMetadata, uint256[] cairoAuxInput, uint256 cairoVerifierId) returns()
func (_GpsStatementVerifier *GpsStatementVerifierTransactorSession) VerifyProofAndRegister(proofParams []*big.Int, proof []*big.Int, taskMetadata []*big.Int, cairoAuxInput []*big.Int, cairoVerifierId *big.Int) (*types.Transaction, error) {
	return _GpsStatementVerifier.Contract.VerifyProofAndRegister(&_GpsStatementVerifier.TransactOpts, proofParams, proof, taskMetadata, cairoAuxInput, cairoVerifierId)
}

// GpsStatementVerifierLogMemoryPagesHashesIterator is returned from FilterLogMemoryPagesHashes and is used to iterate over the raw logs and unpacked data for LogMemoryPagesHashes events raised by the GpsStatementVerifier contract.
type GpsStatementVerifierLogMemoryPagesHashesIterator struct {
	Event *GpsStatementVerifierLogMemoryPagesHashes // Event containing the contract specifics and raw log

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
func (it *GpsStatementVerifierLogMemoryPagesHashesIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(GpsStatementVerifierLogMemoryPagesHashes)
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
		it.Event = new(GpsStatementVerifierLogMemoryPagesHashes)
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
func (it *GpsStatementVerifierLogMemoryPagesHashesIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *GpsStatementVerifierLogMemoryPagesHashesIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// GpsStatementVerifierLogMemoryPagesHashes represents a LogMemoryPagesHashes event raised by the GpsStatementVerifier contract.
type GpsStatementVerifierLogMemoryPagesHashes struct {
	FactHash    [32]byte
	PagesHashes [][32]byte
	Raw         types.Log // Blockchain specific contextual infos
}

// FilterLogMemoryPagesHashes is a free log retrieval operation binding the contract event 0x73b132cb33951232d83dc0f1f81c2d10f9a2598f057404ed02756716092097bb.
//
// Solidity: event LogMemoryPagesHashes(bytes32 factHash, bytes32[] pagesHashes)
func (_GpsStatementVerifier *GpsStatementVerifierFilterer) FilterLogMemoryPagesHashes(opts *bind.FilterOpts) (*GpsStatementVerifierLogMemoryPagesHashesIterator, error) {

	logs, sub, err := _GpsStatementVerifier.contract.FilterLogs(opts, "LogMemoryPagesHashes")
	if err != nil {
		return nil, err
	}
	return &GpsStatementVerifierLogMemoryPagesHashesIterator{contract: _GpsStatementVerifier.contract, event: "LogMemoryPagesHashes", logs: logs, sub: sub}, nil
}

// WatchLogMemoryPagesHashes is a free log subscription operation binding the contract event 0x73b132cb33951232d83dc0f1f81c2d10f9a2598f057404ed02756716092097bb.
//
// Solidity: event LogMemoryPagesHashes(bytes32 factHash, bytes32[] pagesHashes)
func (_GpsStatementVerifier *GpsStatementVerifierFilterer) WatchLogMemoryPagesHashes(opts *bind.WatchOpts, sink chan<- *GpsStatementVerifierLogMemoryPagesHashes) (event.Subscription, error) {

	logs, sub, err := _GpsStatementVerifier.contract.WatchLogs(opts, "LogMemoryPagesHashes")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(GpsStatementVerifierLogMemoryPagesHashes)
				if err := _GpsStatementVerifier.contract.UnpackLog(event, "LogMemoryPagesHashes", log); err != nil {
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

// ParseLogMemoryPagesHashes is a log parse operation binding the contract event 0x73b132cb33951232d83dc0f1f81c2d10f9a2598f057404ed02756716092097bb.
//
// Solidity: event LogMemoryPagesHashes(bytes32 factHash, bytes32[] pagesHashes)
func (_GpsStatementVerifier *GpsStatementVerifierFilterer) ParseLogMemoryPagesHashes(log types.Log) (*GpsStatementVerifierLogMemoryPagesHashes, error) {
	event := new(GpsStatementVerifierLogMemoryPagesHashes)
	if err := _GpsStatementVerifier.contract.UnpackLog(event, "LogMemoryPagesHashes", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
