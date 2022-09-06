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

// FriStatementMetaData contains all meta data concerning the FriStatement contract.
var FriStatementMetaData = &bind.MetaData{
	ABI: "[{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"string\",\"name\":\"name\",\"type\":\"string\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"val\",\"type\":\"uint256\"}],\"name\":\"LogGas\",\"type\":\"event\"},{\"inputs\":[],\"name\":\"hasRegisteredFact\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"bytes32\",\"name\":\"fact\",\"type\":\"bytes32\"}],\"name\":\"isValid\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"proof\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256[]\",\"name\":\"friQueue\",\"type\":\"uint256[]\"},{\"internalType\":\"uint256\",\"name\":\"evaluationPoint\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"friStepSize\",\"type\":\"uint256\"},{\"internalType\":\"uint256\",\"name\":\"expectedRoot\",\"type\":\"uint256\"}],\"name\":\"verifyFRI\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50600436106100415760003560e01c80636a93856714610046578063d6354e1514610077578063e85a6a281461007f575b600080fd5b6100636004803603602081101561005c57600080fd5b50356101b3565b604080519115158252519081900360200190f35b6100636101c4565b6101b1600480360360a081101561009557600080fd5b8101906020810181356401000000008111156100b057600080fd5b8201836020820111156100c257600080fd5b803590602001918460208302840111640100000000831117156100e457600080fd5b919080806020026020016040519081016040528093929190818152602001838360200280828437600092019190915250929594936020810193503591505064010000000081111561013457600080fd5b82018360208201111561014657600080fd5b8035906020019184602083028401116401000000008311171561016857600080fd5b91908080602002602001604051908101604052809392919081815260200183836020028082843760009201919091525092955050823593505050602081013590604001356101cd565b005b60006101be8261071c565b92915050565b60015460ff1690565b600482111561023d57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f46524920737465702073697a6520746f6f206c61726765000000000000000000604482015290519081900360640190fd5b600384518161024857fe5b066001146102a1576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252603e8152602001806110f2603e913960400191505060405180910390fd5b60048451101561031257604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601360248201527f4e6f20717565727920746f2070726f6365737300000000000000000000000000604482015290519081900360640190fd5b835161050090600380820491600091889190840290811061032f57fe5b60200260200101818152505060008060008060007f08000000000000110000000000000000000000000000000000000000000000018a106103d157604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601260248201527f494e56414c49445f4556414c5f504f494e540000000000000000000000000000604482015290519081900360640190fd5b6000805b878110156105d357818d82600302815181106103ed57fe5b60200260200101511161046157604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601360248201527f494e56414c49445f51554552595f56414c554500000000000000000000000000604482015290519081900360640190fd5b7f08000000000000110000000000000000000000000000000000000000000000018d826003026001018151811061049457fe5b60200260200101511061050857604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601160248201527f494e56414c49445f4652495f56414c5545000000000000000000000000000000604482015290519081900360640190fd5b7f08000000000000110000000000000000000000000000000000000000000000018d826003026002018151811061053b57fe5b6020026020010151106105af57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601960248201527f494e56414c49445f4652495f494e56455253455f504f494e5400000000000000604482015290519081900360640190fd5b8c81600302815181106105be57fe5b602090810291909101015191506001016103d5565b508b6000815181106105e157fe5b60200260200101518c60038960030203815181106105fb57fe5b60200260200101518d60008151811061061057fe5b6020026020010151181061068557604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601560248201527f494e56414c49445f515545524945535f52414e47450000000000000000000000604482015290519081900360640190fd5b60208c019450604051935060208d0184526020840195508660400286019250878301915060a082016040528a825289602083015288608083015286606002852060408301526106d383610731565b6106e58486888a8f8f60020a8961081c565b96506106f384878b8a610879565b50606080880286209083015260a0822061070c81610a1d565b5050505050505050505050505050565b60009081526020819052604090205460ff1690565b610200810161040082017f05ec467b88826aba4537602d514425f3b0bdf467bbf302458337c45f6021e539600061076982600f610a8d565b60018085528086527f08000000000000110000000000000000000000000000000000000000000000006020870152909150807f08000000000000110000000000000000000000000000000000000000000000016008825b81811015610810576107d28588610ac1565b94506107de8487610ac1565b935060006107ed826003610aee565b60208082028b0187905260409091028b01878152878603910152506001016107c0565b50505050505050505050565b60008587806060880281015b6000806108378e89878c610b2c565b919650909250905061084e8885848d8d868c610c15565b60408601955060608401935050508083106108285760608b8303049c9b505050505050505050505050565b600080610884610cee565b905060808311156108f657604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f544f4f5f4d414e595f4d45524b4c455f51554552494553000000000000000000604482015290519081900360640190fd5b60208501604084026040600080898201518b515b60018211156109975760018218604060208209888601518160201852878787086002909404858f01528d84015193955060208301928285141561097d57507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0909201918589018888880896508e87015194505b51905250604060002088168388015285858408925061090a565b9290950151918b5250945050508483149050610a1457604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601460248201527f494e56414c49445f4d45524b4c455f50524f4f46000000000000000000000000604482015290519081900360640190fd5b50949350505050565b600081815260208190526040902080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660019081179091555460ff16610a8a57600180547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0016811790555b50565b6000610aba83837f0800000000000011000000000000000000000000000000000000000000000001610d12565b9392505050565b60007f08000000000000110000000000000000000000000000000000000000000000018284099392505050565b6000816101001480610b0257508160020a83105b610b0857fe5b826000805b84811015610a1457600291820260018416179183049250600101610b0d565b81517fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff820119811680820360200285016102009081015160408601518694600093899390840192868901917f0800000000000011000000000000000000000000000000000000000000000001918291900995508b51875b83811015610c0157602082019181861415610bed575060608a018051909a9095507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0909201916020015b518390068752602090960195600101610ba3565b50808d525050505050509450945094915050565b60008761040081016008861415610c3b57610c3281838a8a610d56565b98509250610cb6565b8560041415610c5057610c3281838a8a610e63565b8560101415610c6557610c3281838a8a610eec565b6040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602b8152602001806110c7602b913960400191505060405180910390fd5b6000610cc0610cee565b9587900480865260209788029093209095169386019390935287529286019290925250505060409091015250565b7fffffffffffffffffffffffffffffffffffffffff00000000000000000000000090565b600060405160208152602080820152602060408201528460608201528360808201528260a082015260208160c08360055afa610d4d57600080fd5b51949350505050565b6000807f08000000000000110000000000000000000000000000000000000000000000017f80000000000001100000000000000000000000000000000000000000000000108651828787098381820960208b015160208b0151868188038601850960408d015160608e01519690920101948780848709828a0384010991010186818703860184098186010194505060808b01518660408e0151850960a08d015188818a03840183098184010192505060c08d015160e08e0151898a868509828c038401099101018881890384018184800909818401019250505086878288038701898687090982870108985050505050505080858609818182099050818182099250505094509492505050565b6000807f08000000000000110000000000000000000000000000000000000000000000018085850986516020880151838185038301840981830101915050604088015160608901518485868660208f0151098388038501098284010891505083888909848182099550508384828603840186868709098284010895505050505094509492505050565b60008060007f08000000000000110000000000000000000000000000000000000000000000017f800000000000011000000000000000000000000000000000000000000000001087518288880960208b015160208b0151858187038501840960408d015160608e01519590920101938680848609828903840109910101858380098681820997508682870386018209828601019450505060808b01518560408e0151840960a08d015187818903840183098184010192505060c08d015160e08e01518889868509828b03840109910101878188038401818480090981840101925050508581860385018809818501019350506101008b01518560808e015184096101208d01518781890384018309818401019250506101408d01516101608e01518889868509828b03840109910101878188038401818480090981840101925050506101808c01518660c08f015185096101a08e015188818a0384018309818401019250506101c08e01516101e08f0151898a878509828c038401099101018881890384018184800909818401019250505086818703830188858b090991010185808287038601818a800909828601089850505050505080868709915080828309915080828309915080828309925050509450949250505056fe4f6e6c7920737465702073697a6573206f6620322c2033206f7220342061726520737570706f727465642e465249205175657565206d75737420626520636f6d706f736564206f6620747269706c65747320706c7573206f6e652064656c696d697465722063656c6ca26469706673582212209daff408ee60e5cf622ea81dc11677d3c6ad51998bd9d164ac3fe9ef6b13285564736f6c634300060c0033",
}

// FriStatementABI is the input ABI used to generate the binding from.
// Deprecated: Use FriStatementMetaData.ABI instead.
var FriStatementABI = FriStatementMetaData.ABI

// FriStatementBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use FriStatementMetaData.Bin instead.
var FriStatementBin = FriStatementMetaData.Bin

// DeployFriStatement deploys a new Ethereum contract, binding an instance of FriStatement to it.
func DeployFriStatement(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *FriStatement, error) {
	parsed, err := FriStatementMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(FriStatementBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &FriStatement{FriStatementCaller: FriStatementCaller{contract: contract}, FriStatementTransactor: FriStatementTransactor{contract: contract}, FriStatementFilterer: FriStatementFilterer{contract: contract}}, nil
}

// FriStatement is an auto generated Go binding around an Ethereum contract.
type FriStatement struct {
	FriStatementCaller     // Read-only binding to the contract
	FriStatementTransactor // Write-only binding to the contract
	FriStatementFilterer   // Log filterer for contract events
}

// FriStatementCaller is an auto generated read-only Go binding around an Ethereum contract.
type FriStatementCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FriStatementTransactor is an auto generated write-only Go binding around an Ethereum contract.
type FriStatementTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FriStatementFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type FriStatementFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// FriStatementSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type FriStatementSession struct {
	Contract     *FriStatement     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// FriStatementCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type FriStatementCallerSession struct {
	Contract *FriStatementCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// FriStatementTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type FriStatementTransactorSession struct {
	Contract     *FriStatementTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// FriStatementRaw is an auto generated low-level Go binding around an Ethereum contract.
type FriStatementRaw struct {
	Contract *FriStatement // Generic contract binding to access the raw methods on
}

// FriStatementCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type FriStatementCallerRaw struct {
	Contract *FriStatementCaller // Generic read-only contract binding to access the raw methods on
}

// FriStatementTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type FriStatementTransactorRaw struct {
	Contract *FriStatementTransactor // Generic write-only contract binding to access the raw methods on
}

// NewFriStatement creates a new instance of FriStatement, bound to a specific deployed contract.
func NewFriStatement(address common.Address, backend bind.ContractBackend) (*FriStatement, error) {
	contract, err := bindFriStatement(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &FriStatement{FriStatementCaller: FriStatementCaller{contract: contract}, FriStatementTransactor: FriStatementTransactor{contract: contract}, FriStatementFilterer: FriStatementFilterer{contract: contract}}, nil
}

// NewFriStatementCaller creates a new read-only instance of FriStatement, bound to a specific deployed contract.
func NewFriStatementCaller(address common.Address, caller bind.ContractCaller) (*FriStatementCaller, error) {
	contract, err := bindFriStatement(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &FriStatementCaller{contract: contract}, nil
}

// NewFriStatementTransactor creates a new write-only instance of FriStatement, bound to a specific deployed contract.
func NewFriStatementTransactor(address common.Address, transactor bind.ContractTransactor) (*FriStatementTransactor, error) {
	contract, err := bindFriStatement(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &FriStatementTransactor{contract: contract}, nil
}

// NewFriStatementFilterer creates a new log filterer instance of FriStatement, bound to a specific deployed contract.
func NewFriStatementFilterer(address common.Address, filterer bind.ContractFilterer) (*FriStatementFilterer, error) {
	contract, err := bindFriStatement(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &FriStatementFilterer{contract: contract}, nil
}

// bindFriStatement binds a generic wrapper to an already deployed contract.
func bindFriStatement(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(FriStatementABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FriStatement *FriStatementRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FriStatement.Contract.FriStatementCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FriStatement *FriStatementRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FriStatement.Contract.FriStatementTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FriStatement *FriStatementRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FriStatement.Contract.FriStatementTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_FriStatement *FriStatementCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _FriStatement.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_FriStatement *FriStatementTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _FriStatement.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_FriStatement *FriStatementTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _FriStatement.Contract.contract.Transact(opts, method, params...)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_FriStatement *FriStatementCaller) HasRegisteredFact(opts *bind.CallOpts) (bool, error) {
	var out []interface{}
	err := _FriStatement.contract.Call(opts, &out, "hasRegisteredFact")

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_FriStatement *FriStatementSession) HasRegisteredFact() (bool, error) {
	return _FriStatement.Contract.HasRegisteredFact(&_FriStatement.CallOpts)
}

// HasRegisteredFact is a free data retrieval call binding the contract method 0xd6354e15.
//
// Solidity: function hasRegisteredFact() view returns(bool)
func (_FriStatement *FriStatementCallerSession) HasRegisteredFact() (bool, error) {
	return _FriStatement.Contract.HasRegisteredFact(&_FriStatement.CallOpts)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_FriStatement *FriStatementCaller) IsValid(opts *bind.CallOpts, fact [32]byte) (bool, error) {
	var out []interface{}
	err := _FriStatement.contract.Call(opts, &out, "isValid", fact)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_FriStatement *FriStatementSession) IsValid(fact [32]byte) (bool, error) {
	return _FriStatement.Contract.IsValid(&_FriStatement.CallOpts, fact)
}

// IsValid is a free data retrieval call binding the contract method 0x6a938567.
//
// Solidity: function isValid(bytes32 fact) view returns(bool)
func (_FriStatement *FriStatementCallerSession) IsValid(fact [32]byte) (bool, error) {
	return _FriStatement.Contract.IsValid(&_FriStatement.CallOpts, fact)
}

// VerifyFRI is a paid mutator transaction binding the contract method 0xe85a6a28.
//
// Solidity: function verifyFRI(uint256[] proof, uint256[] friQueue, uint256 evaluationPoint, uint256 friStepSize, uint256 expectedRoot) returns()
func (_FriStatement *FriStatementTransactor) VerifyFRI(opts *bind.TransactOpts, proof []*big.Int, friQueue []*big.Int, evaluationPoint *big.Int, friStepSize *big.Int, expectedRoot *big.Int) (*types.Transaction, error) {
	return _FriStatement.contract.Transact(opts, "verifyFRI", proof, friQueue, evaluationPoint, friStepSize, expectedRoot)
}

// VerifyFRI is a paid mutator transaction binding the contract method 0xe85a6a28.
//
// Solidity: function verifyFRI(uint256[] proof, uint256[] friQueue, uint256 evaluationPoint, uint256 friStepSize, uint256 expectedRoot) returns()
func (_FriStatement *FriStatementSession) VerifyFRI(proof []*big.Int, friQueue []*big.Int, evaluationPoint *big.Int, friStepSize *big.Int, expectedRoot *big.Int) (*types.Transaction, error) {
	return _FriStatement.Contract.VerifyFRI(&_FriStatement.TransactOpts, proof, friQueue, evaluationPoint, friStepSize, expectedRoot)
}

// VerifyFRI is a paid mutator transaction binding the contract method 0xe85a6a28.
//
// Solidity: function verifyFRI(uint256[] proof, uint256[] friQueue, uint256 evaluationPoint, uint256 friStepSize, uint256 expectedRoot) returns()
func (_FriStatement *FriStatementTransactorSession) VerifyFRI(proof []*big.Int, friQueue []*big.Int, evaluationPoint *big.Int, friStepSize *big.Int, expectedRoot *big.Int) (*types.Transaction, error) {
	return _FriStatement.Contract.VerifyFRI(&_FriStatement.TransactOpts, proof, friQueue, evaluationPoint, friStepSize, expectedRoot)
}

// FriStatementLogGasIterator is returned from FilterLogGas and is used to iterate over the raw logs and unpacked data for LogGas events raised by the FriStatement contract.
type FriStatementLogGasIterator struct {
	Event *FriStatementLogGas // Event containing the contract specifics and raw log

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
func (it *FriStatementLogGasIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(FriStatementLogGas)
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
		it.Event = new(FriStatementLogGas)
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
func (it *FriStatementLogGasIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *FriStatementLogGasIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// FriStatementLogGas represents a LogGas event raised by the FriStatement contract.
type FriStatementLogGas struct {
	Name string
	Val  *big.Int
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterLogGas is a free log retrieval operation binding the contract event 0x16298c4eb3718e3472be44fc630569ed86dc073363b2f8291c987b88d571b212.
//
// Solidity: event LogGas(string name, uint256 val)
func (_FriStatement *FriStatementFilterer) FilterLogGas(opts *bind.FilterOpts) (*FriStatementLogGasIterator, error) {

	logs, sub, err := _FriStatement.contract.FilterLogs(opts, "LogGas")
	if err != nil {
		return nil, err
	}
	return &FriStatementLogGasIterator{contract: _FriStatement.contract, event: "LogGas", logs: logs, sub: sub}, nil
}

// WatchLogGas is a free log subscription operation binding the contract event 0x16298c4eb3718e3472be44fc630569ed86dc073363b2f8291c987b88d571b212.
//
// Solidity: event LogGas(string name, uint256 val)
func (_FriStatement *FriStatementFilterer) WatchLogGas(opts *bind.WatchOpts, sink chan<- *FriStatementLogGas) (event.Subscription, error) {

	logs, sub, err := _FriStatement.contract.WatchLogs(opts, "LogGas")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(FriStatementLogGas)
				if err := _FriStatement.contract.UnpackLog(event, "LogGas", log); err != nil {
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

// ParseLogGas is a log parse operation binding the contract event 0x16298c4eb3718e3472be44fc630569ed86dc073363b2f8291c987b88d571b212.
//
// Solidity: event LogGas(string name, uint256 val)
func (_FriStatement *FriStatementFilterer) ParseLogGas(log types.Log) (*FriStatementLogGas, error) {
	event := new(FriStatementLogGas)
	if err := _FriStatement.contract.UnpackLog(event, "LogGas", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
