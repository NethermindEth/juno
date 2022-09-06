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

// CairoBootloaderProgramMetaData contains all meta data concerning the CairoBootloaderProgram contract.
var CairoBootloaderProgramMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"getCompiledProgram\",\"outputs\":[{\"internalType\":\"uint256[216]\",\"name\":\"\",\"type\":\"uint256[216]\"}],\"stateMutability\":\"pure\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b506004361061002b5760003560e01c80634b6cee5914610030575b600080fd5b610038610071565b6040518082611b0080838360005b8381101561005e578181015183820152602001610046565b5050505090500191505060405180910390f35b610079610d3c565b60405180611b00016040528067040780017fff7fff815260200160058152602001671104800180018000815260200160a7815260200167010780017fff7fff81526020016000815260200167020780017fff7ffd81526020016004815260200167480a7ffb7fff8000815260200167208b7fff7fff7ffe815260200167040780017fff7fff81526020016003815260200167404b800080008000815260200167400380007ff98001815260200167400380007ffa8002815260200167020780017fff800081526020016004815260200167010780017fff7fff81526020016004815260200167400380007ffb8001815260200167400380007ffc8002815260200167482680017ff9800081526020016001815260200167482680017ffa800081526020016001815260200167482a80007ffb8000815260200167482a80007ffc8000815260200167482680017ffd800081526020017f0800000000000011000000000000000000000000000000000000000000000000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffea815260200167208b7fff7fff7ffe815260200167208b7fff7fff7ffe815260200167110480018001800081526020017f0800000000000011000000000000000000000000000000000000000000000000815260200167482480017ffe800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffff815260200167208b7fff7fff7ffe815260200167110480018001800081526020017f0800000000000010fffffffffffffffffffffffffffffffffffffffffffffffb815260200167480a7ffa7fff8000815260200167480a7ffb7fff8000815260200167480a7ffc7fff8000815260200167482480017ffb80008152602001603c815260200167480680017fff800081526020016005815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffd8815260200167402a7ffc7ffd7fff815260200167040b7ffd7fff7fff815260200167208b7fff7fff7ffe81526020016748297ffb80007ffc81526020016748487ffd80007fff815260200167400280007ffa7fff815260200167482680017ffa800081526020016001815260200167208b7fff7fff7ffe815260200167020780017fff7ffd81526020016004815260200167480a7ff97fff8000815260200167208b7fff7fff7ffe815260200167480a7ff97fff8000815260200167480280007ffa8000815260200167480280007ffb8000815260200167480280007ffc8000815260200167110480018001800081526020017f0800000000000010fffffffffffffffffffffffffffffffffffffffffffffff3815260200167482680017ffa800081526020016001815260200167482680017ffb800081526020016001815260200167482680017ffc800081526020016001815260200167482680017ffd800081526020017f0800000000000011000000000000000000000000000000000000000000000000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffef815260200167208b7fff7fff7ffe815260200167480280007ffd800081526020016748327fff7ffd8000815260200167480a7ffc7fff8000815260200167480080007ffe800081526020016748007fff7ffd8000815260200167480080007ffd7fff815260200167400080017ffc7ffd815260200167482480017ffb800081526020017f0800000000000011000000000000000000000000000000000000000000000000815260200167482480017ffb800081526020016003815260200167480080027ffa800081526020016740287ffd7ffc7ffd815260200167020680017fff7ffc81526020017f0800000000000010fffffffffffffffffffffffffffffffffffffffffffffff8815260200167208b7fff7fff7ffe815260200167020780017fff7ffd81526020016005815260200167480a7ff97fff8000815260200167480a7ffc7fff8000815260200167208b7fff7fff7ffe815260200167040780017fff7fff81526020016012815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffbb81526020016740137ffe7fff8000815260200167400380007ff98002815260200167480680017fff8000815260200160008152602001674002800180017fff815260200167480280017ff98000815260200167480a80017fff8000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffe18152602001674002800180027fff8152602001674027800180018003815260200160048152602001674003800380018004815260200167482a800480038000815260200167480280028001800081526020016740317fff7ffe800581526020016740278001800280068152602001600281526020016740137ffc7fff8007815260200167400380027ff98008815260200167400380037ff98009815260200167400380047ff9800a815260200167480a7ffa7fff8000815260200167482680018000800081526020016006815260200167480a80037fff8000815260200167480a80047fff8000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffa48152602001671088800580018000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffff9c815260200167402a8004800b7fff815260200167480a7ffa7fff800081526020016748268001800080008152602001600c815260200167480a80037fff8000815260200167480a800b7fff8000815260200167480680017fff800081526020016005815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffff77815260200167402a800380047fff815260200167480a7ffc7fff800081526020016748268001800080008152602001600681526020016748268001800080008152602001600c815260200167480a7ffb7fff8000815260200167480680017fff800081526020016005815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffa0815260200167402b80028011800c815260200167400380008002801181526020016748268001800080008152602001600c815260200167480a7ffa7fff8000815260200167480a7ffb7fff800081526020016748127ffc7fff8000815260200167482680017ffd800081526020017f0800000000000011000000000000000000000000000000000000000000000000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffffba815260200167208b7fff7fff7ffe815260200167040780017fff7fff81526020016010815260200167402780017ff9800181526020016001815260200167400b7ffa7fff8002815260200167400b80007fff8003815260200167400b7ffc7fff8004815260200167400b7ffd7fff8005815260200167400780017fff80068152602001656f7574707574815260200167400780017fff8007815260200167706564657273656e815260200167400780017fff800881526020016a72616e67655f636865636b815260200167400780017fff80098152602001646563647361815260200167400780017fff800a81526020016662697477697365815260200167400780017fff800b81526020016001815260200167400780017fff800c81526020016003815260200167400780017fff800d81526020016001815260200167400780017fff800e81526020016002815260200167400780017fff800f81526020016005815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffff5c815260200167482480017ffe800081526020016001815260200167482480017ffd800081526020016006815260200167482480017ffc80008152602001600b815260200167480a7ffb7fff8000815260200167480280007ff98000815260200167110480018001800081526020017f0800000000000010ffffffffffffffffffffffffffffffffffffffffffffff91815260200167400a80007fff7fff815260200167480080007ffe8000815260200167480080017ffd8000815260200167480080027ffc8000815260200167480080037ffb8000815260200167480080047ffa8000815260200167208b7fff7fff7ffe815250905090565b60405180611b00016040528060d890602082028036833750919291505056fea2646970667358221220673302b4bc52024f12ab58f10e477051aeab31faab81e934870faf3b1d81f48c64736f6c634300060c0033",
}

// CairoBootloaderProgramABI is the input ABI used to generate the binding from.
// Deprecated: Use CairoBootloaderProgramMetaData.ABI instead.
var CairoBootloaderProgramABI = CairoBootloaderProgramMetaData.ABI

// CairoBootloaderProgramBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use CairoBootloaderProgramMetaData.Bin instead.
var CairoBootloaderProgramBin = CairoBootloaderProgramMetaData.Bin

// DeployCairoBootloaderProgram deploys a new Ethereum contract, binding an instance of CairoBootloaderProgram to it.
func DeployCairoBootloaderProgram(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *CairoBootloaderProgram, error) {
	parsed, err := CairoBootloaderProgramMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(CairoBootloaderProgramBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &CairoBootloaderProgram{CairoBootloaderProgramCaller: CairoBootloaderProgramCaller{contract: contract}, CairoBootloaderProgramTransactor: CairoBootloaderProgramTransactor{contract: contract}, CairoBootloaderProgramFilterer: CairoBootloaderProgramFilterer{contract: contract}}, nil
}

// CairoBootloaderProgram is an auto generated Go binding around an Ethereum contract.
type CairoBootloaderProgram struct {
	CairoBootloaderProgramCaller     // Read-only binding to the contract
	CairoBootloaderProgramTransactor // Write-only binding to the contract
	CairoBootloaderProgramFilterer   // Log filterer for contract events
}

// CairoBootloaderProgramCaller is an auto generated read-only Go binding around an Ethereum contract.
type CairoBootloaderProgramCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CairoBootloaderProgramTransactor is an auto generated write-only Go binding around an Ethereum contract.
type CairoBootloaderProgramTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CairoBootloaderProgramFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type CairoBootloaderProgramFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// CairoBootloaderProgramSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type CairoBootloaderProgramSession struct {
	Contract     *CairoBootloaderProgram // Generic contract binding to set the session for
	CallOpts     bind.CallOpts           // Call options to use throughout this session
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// CairoBootloaderProgramCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type CairoBootloaderProgramCallerSession struct {
	Contract *CairoBootloaderProgramCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts                 // Call options to use throughout this session
}

// CairoBootloaderProgramTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type CairoBootloaderProgramTransactorSession struct {
	Contract     *CairoBootloaderProgramTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts                 // Transaction auth options to use throughout this session
}

// CairoBootloaderProgramRaw is an auto generated low-level Go binding around an Ethereum contract.
type CairoBootloaderProgramRaw struct {
	Contract *CairoBootloaderProgram // Generic contract binding to access the raw methods on
}

// CairoBootloaderProgramCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type CairoBootloaderProgramCallerRaw struct {
	Contract *CairoBootloaderProgramCaller // Generic read-only contract binding to access the raw methods on
}

// CairoBootloaderProgramTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type CairoBootloaderProgramTransactorRaw struct {
	Contract *CairoBootloaderProgramTransactor // Generic write-only contract binding to access the raw methods on
}

// NewCairoBootloaderProgram creates a new instance of CairoBootloaderProgram, bound to a specific deployed contract.
func NewCairoBootloaderProgram(address common.Address, backend bind.ContractBackend) (*CairoBootloaderProgram, error) {
	contract, err := bindCairoBootloaderProgram(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &CairoBootloaderProgram{CairoBootloaderProgramCaller: CairoBootloaderProgramCaller{contract: contract}, CairoBootloaderProgramTransactor: CairoBootloaderProgramTransactor{contract: contract}, CairoBootloaderProgramFilterer: CairoBootloaderProgramFilterer{contract: contract}}, nil
}

// NewCairoBootloaderProgramCaller creates a new read-only instance of CairoBootloaderProgram, bound to a specific deployed contract.
func NewCairoBootloaderProgramCaller(address common.Address, caller bind.ContractCaller) (*CairoBootloaderProgramCaller, error) {
	contract, err := bindCairoBootloaderProgram(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &CairoBootloaderProgramCaller{contract: contract}, nil
}

// NewCairoBootloaderProgramTransactor creates a new write-only instance of CairoBootloaderProgram, bound to a specific deployed contract.
func NewCairoBootloaderProgramTransactor(address common.Address, transactor bind.ContractTransactor) (*CairoBootloaderProgramTransactor, error) {
	contract, err := bindCairoBootloaderProgram(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &CairoBootloaderProgramTransactor{contract: contract}, nil
}

// NewCairoBootloaderProgramFilterer creates a new log filterer instance of CairoBootloaderProgram, bound to a specific deployed contract.
func NewCairoBootloaderProgramFilterer(address common.Address, filterer bind.ContractFilterer) (*CairoBootloaderProgramFilterer, error) {
	contract, err := bindCairoBootloaderProgram(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &CairoBootloaderProgramFilterer{contract: contract}, nil
}

// bindCairoBootloaderProgram binds a generic wrapper to an already deployed contract.
func bindCairoBootloaderProgram(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(CairoBootloaderProgramABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CairoBootloaderProgram *CairoBootloaderProgramRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CairoBootloaderProgram.Contract.CairoBootloaderProgramCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CairoBootloaderProgram *CairoBootloaderProgramRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CairoBootloaderProgram.Contract.CairoBootloaderProgramTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CairoBootloaderProgram *CairoBootloaderProgramRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CairoBootloaderProgram.Contract.CairoBootloaderProgramTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_CairoBootloaderProgram *CairoBootloaderProgramCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _CairoBootloaderProgram.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_CairoBootloaderProgram *CairoBootloaderProgramTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _CairoBootloaderProgram.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_CairoBootloaderProgram *CairoBootloaderProgramTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _CairoBootloaderProgram.Contract.contract.Transact(opts, method, params...)
}

// GetCompiledProgram is a free data retrieval call binding the contract method 0x4b6cee59.
//
// Solidity: function getCompiledProgram() pure returns(uint256[216])
func (_CairoBootloaderProgram *CairoBootloaderProgramCaller) GetCompiledProgram(opts *bind.CallOpts) ([216]*big.Int, error) {
	var out []interface{}
	err := _CairoBootloaderProgram.contract.Call(opts, &out, "getCompiledProgram")

	if err != nil {
		return *new([216]*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new([216]*big.Int)).(*[216]*big.Int)

	return out0, err

}

// GetCompiledProgram is a free data retrieval call binding the contract method 0x4b6cee59.
//
// Solidity: function getCompiledProgram() pure returns(uint256[216])
func (_CairoBootloaderProgram *CairoBootloaderProgramSession) GetCompiledProgram() ([216]*big.Int, error) {
	return _CairoBootloaderProgram.Contract.GetCompiledProgram(&_CairoBootloaderProgram.CallOpts)
}

// GetCompiledProgram is a free data retrieval call binding the contract method 0x4b6cee59.
//
// Solidity: function getCompiledProgram() pure returns(uint256[216])
func (_CairoBootloaderProgram *CairoBootloaderProgramCallerSession) GetCompiledProgram() ([216]*big.Int, error) {
	return _CairoBootloaderProgram.Contract.GetCompiledProgram(&_CairoBootloaderProgram.CallOpts)
}
