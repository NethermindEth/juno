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

// MerkleStatementMetaData contains all meta data concerning the MerkleStatement contract.
var MerkleStatementMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"hasRegisteredFact\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fact\",\"type\":\"bytes32\"}],\"name\":\"isValid\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"merkleView\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"initialMerkleQueue\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"height\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"expectedRoot\",\"type\":\"uint256\"}],\"name\":\"verifyMerkle\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50600436106100415760003560e01c80633fe317a6146100465780636a93856714610174578063d6354e15146101a5575b600080fd5b6101726004803603608081101561005c57600080fd5b81019060208101813564010000000081111561007757600080fd5b82018360208201111561008957600080fd5b803590602001918460208302840111640100000000831117156100ab57600080fd5b91908080602002602001604051908101604052809392919081815260200183836020028082843760009201919091525092959493602081019350359150506401000000008111156100fb57600080fd5b82018360208201111561010d57600080fd5b8035906020019184602083028401116401000000008311171561012f57600080fd5b91908080602002602001604051908101604052809392919081815260200183836020028082843760009201919091525092955050823593505050602001356101ad565b005b6101916004803603602081101561018a57600080fd5b5035610420565b604080519115158252519081900360200190f35b610191610431565b60c8821061021c57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601560248201527f486569676874206d757374206265203c203230302e0000000000000000000000604482015290519081900360640190fd5b8251610100101561028e57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f544f4f5f4d414e595f4d45524b4c455f51554552494553000000000000000000604482015290519081900360640190fd5b600283518161029957fe5b061561030657604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601560248201527f4f44445f4d45524b4c455f51554555455f53495a450000000000000000000000604482015290519081900360640190fd5b6040805160208681018252855186820193600290910491838101916000919084028901016001881b5b818710156103605786518085526020808901519086015260409788019790940193908110929092179160010161032f565b60208481016040528a0196506002891b1091909117905080156103e457604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601660248201527f494e56414c49445f4d45524b4c455f494e444943455300000000000000000000604482015290519081900360640190fd5b60006103f28587898761043a565b905060008184526020860193506020604086020184209050610413816105de565b5050505050505050505050565b600061042b8261064e565b92915050565b60015460ff1690565b600080610445610663565b905060808311156104b757604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f544f4f5f4d414e595f4d45524b4c455f51554552494553000000000000000000604482015290519081900360640190fd5b60208501604084026040600080898201518b515b60018211156105585760018218604060208209888601518160201852878787086002909404858f01528d84015193955060208301928285141561053e57507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0909201918589018888880896508e87015194505b5190525060406000208816838801528585840892506104cb565b9290950151918b52509450505084831490506105d557604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601460248201527f494e56414c49445f4d45524b4c455f50524f4f46000000000000000000000000604482015290519081900360640190fd5b50949350505050565b600081815260208190526040902080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660019081179091555460ff1661064b57600180547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0016811790555b50565b60009081526020819052604090205460ff1690565b7fffffffffffffffffffffffffffffffffffffffff0000000000000000000000009056fea2646970667358221220ee53e60e7da7678f79f6c75a0d3394890e03d611b2c28372cffadf52f362dcc564736f6c634300060c0033",
}

// MerkleStatementABI is the input ABI used to generate the binding from.
// Deprecated: Use MerkleStatementMetaData.ABI instead.
var MerkleStatementABI = MerkleStatementMetaData.ABI

// MerkleStatementBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use MerkleStatementMetaData.Bin instead.
var MerkleStatementBin = MerkleStatementMetaData.Bin

// DeployMerkleStatement deploys a new Ethereum contract, binding an instance of MerkleStatement to it.
func DeployMerkleStatement(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *MerkleStatement, error) {
	parsed, err := MerkleStatementMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(MerkleStatementBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &MerkleStatement{MerkleStatementCaller: MerkleStatementCaller{contract: contract}, MerkleStatementTransactor: MerkleStatementTransactor{contract: contract}, MerkleStatementFilterer: MerkleStatementFilterer{contract: contract}}, nil
}

// MerkleStatement is an auto generated Go binding around an Ethereum contract.
type MerkleStatement struct {
	MerkleStatementCaller     // Read-only binding to the contract
	MerkleStatementTransactor // Write-only binding to the contract
	MerkleStatementFilterer   // Log filterer for contract events
}

// MerkleStatementCaller is an auto generated read-only Go binding around an Ethereum contract.
type MerkleStatementCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MerkleStatementTransactor is an auto generated write-only Go binding around an Ethereum contract.
type MerkleStatementTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MerkleStatementFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type MerkleStatementFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// MerkleStatementSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type MerkleStatementSession struct {
	Contract     *MerkleStatement  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// MerkleStatementCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type MerkleStatementCallerSession struct {
	Contract *MerkleStatementCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// MerkleStatementTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type MerkleStatementTransactorSession struct {
	Contract     *MerkleStatementTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// MerkleStatementRaw is an auto generated low-level Go binding around an Ethereum contract.
type MerkleStatementRaw struct {
	Contract *MerkleStatement // Generic contract binding to access the raw methods on
}

// MerkleStatementCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type MerkleStatementCallerRaw struct {
	Contract *MerkleStatementCaller // Generic read-only contract binding to access the raw methods on
}

// MerkleStatementTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type MerkleStatementTransactorRaw struct {
	Contract *MerkleStatementTransactor // Generic write-only contract binding to access the raw methods on
}

// NewMerkleStatement creates a new instance of MerkleStatement, bound to a specific deployed contract.
func NewMerkleStatement(address common.Address, backend bind.ContractBackend) (*MerkleStatement, error) {
	contract, err := bindMerkleStatement(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &MerkleStatement{MerkleStatementCaller: MerkleStatementCaller{contract: contract}, MerkleStatementTransactor: MerkleStatementTransactor{contract: contract}, MerkleStatementFilterer: MerkleStatementFilterer{contract: contract}}, nil
}

// NewMerkleStatementCaller creates a new read-only instance of MerkleStatement, bound to a specific deployed contract.
func NewMerkleStatementCaller(address common.Address, caller bind.ContractCaller) (*MerkleStatementCaller, error) {
	contract, err := bindMerkleStatement(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &MerkleStatementCaller{contract: contract}, nil
}

// NewMerkleStatementTransactor creates a new write-only instance of MerkleStatement, bound to a specific deployed contract.
func NewMerkleStatementTransactor(address common.Address, transactor bind.ContractTransactor) (*MerkleStatementTransactor, error) {
	contract, err := bindMerkleStatement(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &MerkleStatementTransactor{contract: contract}, nil
}

// NewMerkleStatementFilterer creates a new log filterer instance of MerkleStatement, bound to a specific deployed contract.
func NewMerkleStatementFilterer(address common.Address, filterer bind.ContractFilterer) (*MerkleStatementFilterer, error) {
	contract, err := bindMerkleStatement(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &MerkleStatementFilterer{contract: contract}, nil
}

// bindMerkleStatement binds a generic wrapper to an already deployed contract.
func bindMerkleStatement(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(MerkleStatementABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MerkleStatement *MerkleStatementRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MerkleStatement.Contract.MerkleStatementCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MerkleStatement *MerkleStatementRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MerkleStatement.Contract.MerkleStatementTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MerkleStatement *MerkleStatementRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MerkleStatement.Contract.MerkleStatementTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_MerkleStatement *MerkleStatementCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _MerkleStatement.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_MerkleStatement *MerkleStatementTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _MerkleStatement.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_MerkleStatement *MerkleStatementTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _MerkleStatement.Contract.contract.Transact(opts, method, params...)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_MerkleStatement *MerkleStatementCaller) HasRegisteredFact(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _MerkleStatement.contract.Call(opts, &out, "hasRegisteredFact")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_MerkleStatement *MerkleStatementSession) HasRegisteredFact() (bool, error) {
	return _MerkleStatement.Contract.HasRegisteredFact(&_MerkleStatement.CallOpts)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_MerkleStatement *MerkleStatementCallerSession) HasRegisteredFact() (bool, error) {
	return _MerkleStatement.Contract.HasRegisteredFact(&_MerkleStatement.CallOpts)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_MerkleStatement *MerkleStatementCaller) IsValid(opts *bind.CallOpts, fact [32]byte) (bool, error) {
	var out []interface{}
	err := _MerkleStatement.contract.Call(opts, &out, "isValid", fact)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_MerkleStatement *MerkleStatementSession) IsValid(fact [32]byte) (bool, error) {
	return _MerkleStatement.Contract.IsValid(&_MerkleStatement.CallOpts, fact)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_MerkleStatement *MerkleStatementCallerSession) IsValid(fact [32]byte) (bool, error) {
	return _MerkleStatement.Contract.IsValid(&_MerkleStatement.CallOpts, fact)
}

// VerifyMerkle is a paid mutator transaction binding the contract method 0x3fe317a6.
//
// Solidity: function verifyMerkle(uint256[] merkleView, uint256[] initialMerkleQueue, uint256 height, uint256 expectedRoot) returns()
func (_MerkleStatement *MerkleStatementTransactor) VerifyMerkle(opts *bind.TransactOpts, merkleView []*big.Int, initialMerkleQueue []*big.Int, height *big.Int, expectedRoot *big.Int) (*types.Transaction, error) {
	return _MerkleStatement.contract.Transact(opts, "verifyMerkle", merkleView, initialMerkleQueue, height, expectedRoot)
}

// VerifyMerkle is a paid mutator transaction binding the contract method 0x3fe317a6.
//
// Solidity: function verifyMerkle(uint256[] merkleView, uint256[] initialMerkleQueue, uint256 height, uint256 expectedRoot) returns()
func (_MerkleStatement *MerkleStatementSession) VerifyMerkle(merkleView []*big.Int, initialMerkleQueue []*big.Int, height *big.Int, expectedRoot *big.Int) (*types.Transaction, error) {
	return _MerkleStatement.Contract.VerifyMerkle(&_MerkleStatement.TransactOpts, merkleView, initialMerkleQueue, height, expectedRoot)
}

// VerifyMerkle is a paid mutator transaction binding the contract method 0x3fe317a6.
//
// Solidity: function verifyMerkle(uint256[] merkleView, uint256[] initialMerkleQueue, uint256 height, uint256 expectedRoot) returns()
func (_MerkleStatement *MerkleStatementTransactorSession) VerifyMerkle(merkleView []*big.Int, initialMerkleQueue []*big.Int, height *big.Int, expectedRoot *big.Int) (*types.Transaction, error) {
	return _MerkleStatement.Contract.VerifyMerkle(&_MerkleStatement.TransactOpts, merkleView, initialMerkleQueue, height, expectedRoot)
}
