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
	Bin: "0x608060405234801561001057600080fd5b50600436106100a35760003560e01c80639b3b76cc11610076578063d6354e151161005b578063d6354e1514610273578063e5b62b291461027b578063eeb7286614610283576100a3565b80639b3b76cc14610103578063b7a771f71461026b576100a3565b806329e10520146100a85780634c14a6f9146100c25780635b4c41c2146100ca5780636a938567146100d2575b600080fd5b6100b0610300565b60408051918252519081900360200190f35b6100b0610305565b6100b061030a565b6100ef600480360360208110156100e857600080fd5b503561030f565b604080519115158252519081900360200190f35b610269600480360360a081101561011957600080fd5b81019060208101813564010000000081111561013457600080fd5b82018360208201111561014657600080fd5b8035906020019184602083028401116401000000008311171561016857600080fd5b91939092909160208101903564010000000081111561018657600080fd5b82018360208201111561019857600080fd5b803590602001918460208302840111640100000000831117156101ba57600080fd5b9193909290916020810190356401000000008111156101d857600080fd5b8201836020820111156101ea57600080fd5b8035906020019184602083028401116401000000008311171561020c57600080fd5b91939092909160208101903564010000000081111561022a57600080fd5b82018360208201111561023c57600080fd5b8035906020019184602083028401116401000000008311171561025e57600080fd5b919350915035610320565b005b6100b06108ff565b6100ef610904565b6100b061090d565b61028b610912565b6040805160208082528351818301528351919283929083019185019080838360005b838110156102c55781810151838201526020016102ad565b50505050905090810190601f1680156102f25780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b600281565b600381565b600181565b600061031a82610932565b92915050565b600354811061039057604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820181905260248201527f636169726f56657269666965724964206973206f7574206f662072616e67652e604482015290519081900360640190fd5b60006003828154811061039f57fe5b600091825260208220015473ffffffffffffffffffffffffffffffffffffffff16915036906103f27ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe8601828789611ad9565b9150915060606000808573ffffffffffffffffffffffffffffffffffffffff16638080fdfb6040518163ffffffff1660e01b8152600401604080518083038186803b15801561044057600080fd5b505afa158015610454573d6000803e3d6000fd5b505050506040513d604081101561046a57600080fd5b50805160209091015190925090508188116104e657604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601d60248201527f496e76616c696420636169726f417578496e707574206c656e6774682e000000604482015290519081900360640190fd5b6104f284838188611ad9565b8080602002602001604051908101604052809392919081815260200183836020028082843760009201829052508451949750938793508492501515905061053557fe5b6020026020010151905061271081106105af57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152600f60248201527f496e76616c6964206e50616765732e0000000000000000000000000000000000604482015290519081900360640190fd5b8351600482021461060b576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526021815260200180611b536021913960400191505060405180910390fd5b600080600061061d8f8f8f8f89610947565b925092509250828760018151811061063157fe5b6020026020010151146106a557604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601f60248201527f496e76616c69642073697a6520666f72206d656d6f7279207061676520302e00604482015290519081900360640190fd5b81876002815181106106b357fe5b60200260200101511461072757604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601f60248201527f496e76616c6964206861736820666f72206d656d6f7279207061676520302e00604482015290519081900360640190fd5b8087600386028151811061073757fe5b602002602001015114610795576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602d815260200180611c3b602d913960400191505060405180910390fd5b5050505050508373ffffffffffffffffffffffffffffffffffffffff16631cb7dd798e8e8e8e88886040518763ffffffff1660e01b81526004018080602001806020018060200184810384528a8a82818152602001925060200280828437600083820152601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169091018581038452888152602090810191508990890280828437600083820152601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169091018581038352868152602090810191508790870280828437600081840152601f19601f8201169050808301925050509950505050505050505050600060405180830381600087803b1580156108b957600080fd5b505af11580156108cd573d6000803e3d6000fd5b505050506108f08989838a8a60088181106108e457fe5b90506020020135611363565b50505050505050505050505050565b606081565b60015460ff1690565b600081565b6060604051806060016040528060258152602001611b2e60259139905090565b60009081526020819052604090205460ff1690565b6000806000808888600081811061095a57fe5b905060200201359050634000000081106109d557604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601860248201527f496e76616c6964206e756d626572206f66207461736b732e0000000000000000604482015290519081900360640190fd5b600281810260e5019450606090850267ffffffffffffffff811180156109fa57600080fd5b50604051908082528060200260200182016040528015610a24578160200160208202803683370190505b5090506000610a31611a9c565b60018054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16634b6cee596040518163ffffffff1660e01b8152600401611b006040518083038186803b158015610a9857600080fd5b505afa158015610aac573d6000803e3d6000fd5b505050506040513d601f19601f82011682018060405250611b00811015610ad257600080fd5b50905060005b60d8811015610b375760018101848481518110610af157fe5b602002602001018181525050818160d88110610b0957fe5b6020020151848460010181518110610b1d57fe5b602090810291909101015260029290920191600101610ad8565b5050600089896006818110610b4857fe5b9050602002013590506002811015610bc157604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820181905260248201527f496e76616c696420657865637574696f6e20626567696e20616464726573732e604482015290519081900360640190fd5b60028103838360000181518110610bd457fe5b60200260200101818152505080838360010181518110610bf057fe5b60200260200101818152505060018103838360020181518110610c0f57fe5b6020026020010181815250506000838360030181518110610c2c57fe5b602002602001018181525050600482019150600060058b8b6007818110610c4f57fe5b9050602002013503905060006008905060005b6005811015610d6957808401868681518110610c7a57fe5b60209081029190910101528551600a86019084830190889083908110610c9c57fe5b602090810291909101015260018c1615610d1a578d8d84818110610cbc57fe5b90506020020135878760010181518110610cd257fe5b6020026020010181815250508d8d84600101818110610ced57fe5b90506020020135878260010181518110610d0357fe5b602002602001018181525050600283019250610d55565b6000878760010181518110610d2b57fe5b6020026020010181815250506000878260010181518110610d4857fe5b6020026020010181815250505b5060019a8b1c9a6002959095019401610c62565b508915610dc1576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526024815260200180611bc86024913960400191505060405180910390fd5b505050600a01600583028989600c818110610dd857fe5b90506020020135018989600d818110610ded57fe5b905060200201351015610e4b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252604f815260200180611bec604f913960600191505060405180910390fd5b600089896008818110610e5a57fe5b90506020020135905080838360000181518110610e7357fe5b60200260200101818152505083838360010181518110610e8f57fe5b6020026020010181815250506002820191506001810190503660008d8d6001908092610ebd93929190611ad9565b9150915060005b8681101561109e57600083836000818110610edb57fe5b90506020020135905080600211158015610ef85750634000000081105b610f6357604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601960248201527f496e76616c6964207461736b206f75747075742073697a652e00000000000000604482015290519081900360640190fd5b600084846001818110610f7257fe5b905060200201359050600085856002818110610f8a57fe5b90506020020135905080600111158015610fa657506210000081105b610ffb576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526035815260200180611d0f6035913960400191505060405180910390fd5b8689896000018151811061100b57fe5b6020026020010181815250508289896001018151811061102757fe5b6020026020010181815250508660010189896002018151811061104657fe5b6020026020010181815250508189896003018151811061106257fe5b6020908102919091010152600497909701969582019561108a85600360028402018189611ad9565b955095505050508080600101915050610ec4565b50801561110c57604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601f60248201527f496e76616c6964206c656e677468206f66207461736b4d657461646174612e00604482015290519081900360640190fd5b828c8c600981811061111a57fe5b9050602002013514611177576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526023815260200180611c686023913960400191505060405180910390fd5b505050808251146111d3576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526029815260200180611b056029913960400191505060405180910390fd5b600089897ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe810181811061120357fe5b90506020020135905060008a8a60018d8d90500381811061122057fe5b9050602002013590506000600260009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1663405a63628685857f08000000000000110000000000000000000000000000000000000000000000016040518563ffffffff1660e01b81526004018080602001858152602001848152602001838152602001828103825286818151815260200191508051906020019060200280838360005b838110156112ee5781810151838201526020016112d6565b5050505090500195505050505050606060405180830381600087803b15801561131657600080fd5b505af115801561132a573d6000803e3d6000fd5b505050506040513d606081101561134057600080fd5b506020810151604090910151999f909e50989c50979a5050505050505050505050565b60008260008151811061137257fe5b6020026020010151905060008060008787600081811061138e57fe5b90506020020135905060608460030167ffffffffffffffff811180156113b357600080fd5b506040519080825280602002602001820160405280156113dd578160200160208202803683370190505b5090506040816001815181106113ef57fe5b60209081029190910101526001600387018160606002890267ffffffffffffffff8111801561141d57600080fd5b50604051908082528060200260200182016040528015611447578160200160208202803683370190505b50905060608d8d8080602002602001604051908101604052809392919081815260200183836020028082843760009201919091525092935061148c9250611abb915050565b506000985060808c015b878a10156117a0576000808590508360028901815181106114b357fe5b60200260200101519a506000805b8c8110156115e65760008660008360020260038e010101815181106114e257fe5b60200260200101519050621000008110611547576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602b815260200180611caf602b913960400191505060405180910390fd5b60005b818110156115a257600080611562898e8a8e8a611809565b91509150808f888e036003018151811061157857fe5b6020908102919091010152509a8b019a60019a8b019a60609890980197960195938401930161154a565b5060008760018460020260038f010101815181106115bc57fe5b60200260200101519050806000146115dc576115d9898583611942565b93505b50506001016114c1565b5080600114611640576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526029815260200180611b746029913960400191505060405180910390fd5b60008560018b018151811061165157fe5b6020026020010151905060008660008c018151811061166c57fe5b60200260200101519050808860018151811061168457fe5b6020026020010151600201146116e5576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526035815260200180611cda6035913960400191505060405180910390fd5b506000876000815181106116f557fe5b602002602001015190506000828260405160200180838152602001828152602001925050506040516020818303038152906040528051906020012090508e6002026003018c019b5060007f73b132cb33951232d83dc0f1f81c2d10f9a2598f057404ed02756716092097bb905060208e01868c03848252806040830152826020600383010283a150505061178881611a2c565b50506001909d019c5050506002959095019450611496565b838b146117f8576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526024815260200180611c8b6024913960400191505060405180910390fd5b505050505050505050505050505050565b84516020860151604087015190916340000000831061188957604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601260248201527f496e76616c696420706167652073697a652e0000000000000000000000000000604482015290519081900360640190fd5b8681146118f757604080517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601560248201527f496e76616c6964207061676520616464726573732e0000000000000000000000604482015290519081900360640190fd5b82860185600186600202018151811061190c57fe5b6020026020010181815250508185600086600202018151811061192b57fe5b602002602001018181525050509550959350505050565b60008282111561199d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602b815260200180611b9d602b913960400191505060405180910390fd5b600084600180860360020201815181106119b357fe5b60200260200101519050600083850390506000600282026020026020019050600060408602828901209050838860018560020201815181106119f157fe5b60200260200101818152505080600101886000856002020181518110611a1357fe5b6020908102919091010152505060010195945050505050565b600081815260208190526040902080547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff001660019081179091555460ff16611a9957600180547fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0016811790555b50565b60405180611b00016040528060d8906020820280368337509192915050565b60405180606001604052806003906020820280368337509192915050565b60008085851115611ae8578182fd5b83861115611af4578182fd5b505060208302019391909203915056fe4e6f7420616c6c20436169726f207075626c696320696e707574732077657265207772697474656e2e537461726b576172655f47707353746174656d656e7456657269666965725f323032315f34496e76616c6964207075626c69634d656d6f72795061676573206c656e6774682e4e6f646520737461636b206d75737420636f6e7461696e2065786163746c79206f6e65206974656d2e496e76616c69642076616c7565206f66206e5f6e6f64657320696e2074726565207374727563747572652e53454c45435445445f4255494c54494e535f564543544f525f49535f544f4f5f4c4f4e4752616e67652d636865636b2073746f7020706f696e7465722073686f756c6420626520616674657220616c6c2072616e676520636865636b73207573656420666f722076616c69646174696f6e732e496e76616c69642063756d756c61746976652070726f6475637420666f72206d656d6f7279207061676520302e496e636f6e73697374656e742070726f6772616d206f7574707574206c656e6774682e4e6f7420616c6c206d656d6f727920706167657320776572652070726f6365737365642e496e76616c69642076616c7565206f66206e5f706167657320696e2074726565207374727563747572652e5468652073756d206f662074686520706167652073697a657320646f6573206e6f74206d61746368206f75747075742073697a652e496e76616c6964206e756d626572206f6620706169727320696e20746865204d65726b6c652074726565207374727563747572652ea264697066735822122001962cd4d4b2316b215e8315c6116d1902927c8eeccab2dd2ad454df2cb71fd764736f6c634300060c0033",
}

// GpsStatementVerifierABI is the input ABI used to generate the binding from.
// Deprecated: Use GpsStatementVerifierMetaData.ABI instead.
var GpsStatementVerifierABI = GpsStatementVerifierMetaData.ABI

// GpsStatementVerifierBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use GpsStatementVerifierMetaData.Bin instead.
var GpsStatementVerifierBin = GpsStatementVerifierMetaData.Bin

// DeployGpsStatementVerifier deploys a new Ethereum contract, binding an instance of GpsStatementVerifier to it.
func DeployGpsStatementVerifier(auth *bind.TransactOpts, backend bind.ContractBackend, bootloaderProgramContract common.Address, memoryPageFactRegistry_ common.Address, cairoVerifierContracts []common.Address) (common.Address, *types.Transaction, *GpsStatementVerifier, error) {
	parsed, err := GpsStatementVerifierMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(GpsStatementVerifierBin), backend, bootloaderProgramContract, memoryPageFactRegistry_, cairoVerifierContracts)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &GpsStatementVerifier{GpsStatementVerifierCaller: GpsStatementVerifierCaller{contract: contract}, GpsStatementVerifierTransactor: GpsStatementVerifierTransactor{contract: contract}, GpsStatementVerifierFilterer: GpsStatementVerifierFilterer{contract: contract}}, nil
}

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
