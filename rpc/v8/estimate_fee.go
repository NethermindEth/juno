package rpcv8

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/rpc/rpccore"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
)

type FeeUnit byte

const (
	WEI FeeUnit = iota
	FRI
)

func (u FeeUnit) MarshalText() ([]byte, error) {
	switch u {
	case WEI:
		return []byte("WEI"), nil
	case FRI:
		return []byte("FRI"), nil
	default:
		return nil, fmt.Errorf("unknown FeeUnit %v", u)
	}
}

type FeeEstimate struct {
	L1GasConsumed     *felt.Felt `json:"l1_gas_consumed,omitempty"`
	L1GasPrice        *felt.Felt `json:"l1_gas_price,omitempty"`
	L2GasConsumed     *felt.Felt `json:"l2_gas_consumed,omitempty"`
	L2GasPrice        *felt.Felt `json:"l2_gas_price,omitempty"`
	L1DataGasConsumed *felt.Felt `json:"l1_data_gas_consumed,omitempty"`
	L1DataGasPrice    *felt.Felt `json:"l1_data_gas_price,omitempty"`
	OverallFee        *felt.Felt `json:"overall_fee"`
	Unit              *FeeUnit   `json:"unit,omitempty"`
}

/****************************************************
		Estimate Fee Handlers
*****************************************************/

// TODO: Implement infrastructure for testing with actual vm execution.
// Now error stack can be tested with this request:
/*
curl --location 'http://localhost:6060/rpc/v0_8' \
--header 'Content-Type: application/json' \
--data '{
    "method": "starknet_estimateFee",
    "params": [
        [
            {
                "type": "INVOKE",
                "version": "0x3",
                "nonce": "0x00000000000000000000000000000000000000000000000000000000000018f9",
                "sender_address": "0x4a3140b5a24d4010e8e7bd02142ebd6a3e6daf8d8aa5568f0db45961938d1ad",
                "signature": [
                    "0x18c9fb76277f4087ab95cf2eaabe185ef2d335ee9982e5916f5103a857fbd36",
                    "0x3b911086dd21d28d4f503adafd617d77f92e83da238a0b5167e93473633b7ef"
                ],
                "calldata": [
                    "0x1",
                    "0x450a08ede3e6e2f182ef29b282ff822d11c87636f556f4be3c046394cf98970",
                    "0x3dbc508ba4afd040c8dc4ff8a61113a7bcaf5eae88a6ba27b3c50578b3587e3",
                    "0x7f",
                    "0x414e595f43414c4c4552",
                    "0x7b8fe8db8031a864737008f1a461d900ae792bb16f3d895dbd91be73ce26917",
                    "0x1",
                    "0x0",
                    "0x67bd8167",
                    "0x1",
                    "0x58ccf59375950f5f918131e82e50e2980e367f71a952754610a335ab0bb526d",
                    "0x28f8f885d026133a2e43ae2f112a5a5d12f37022f8647d8acdb9f85df00907e",
                    "0x1",
                    "0x3",
                    "0x74",
                    "0x73657373696f6e2d746f6b656e",
                    "0x67bd8211",
                    "0x77c2583d72331d348640fb32f85f86ffc5df7ecab796e88ea0571c158a47ce4",
                    "0x0",
                    "0x104cbe66509c32968c34be77ce07791cd8705752fa9e00bcbd628b10456d5eb",
                    "0x0",
                    "0x1",
                    "0x5e",
                    "0x1",
                    "0x4",
                    "0x16",
                    "0x68",
                    "0x74",
                    "0x74",
                    "0x70",
                    "0x73",
                    "0x3a",
                    "0x2f",
                    "0x2f",
                    "0x78",
                    "0x2e",
                    "0x63",
                    "0x61",
                    "0x72",
                    "0x74",
                    "0x72",
                    "0x69",
                    "0x64",
                    "0x67",
                    "0x65",
                    "0x2e",
                    "0x67",
                    "0x67",
                    "0x9d0aec9905466c9adf79584fa75fed3",
                    "0x20a97ec3f8efbc2aca0cf7cabb420b4a",
                    "0x8b405e42872d26327e86d4953338d819",
                    "0x926e1c4c5c47ac5ff63779fed5e34738",
                    "0x39",
                    "0x2c",
                    "0x22",
                    "0x63",
                    "0x72",
                    "0x6f",
                    "0x73",
                    "0x73",
                    "0x4f",
                    "0x72",
                    "0x69",
                    "0x67",
                    "0x69",
                    "0x6e",
                    "0x22",
                    "0x3a",
                    "0x74",
                    "0x72",
                    "0x75",
                    "0x65",
                    "0x2c",
                    "0x22",
                    "0x74",
                    "0x6f",
                    "0x70",
                    "0x4f",
                    "0x72",
                    "0x69",
                    "0x67",
                    "0x69",
                    "0x6e",
                    "0x22",
                    "0x3a",
                    "0x22",
                    "0x68",
                    "0x74",
                    "0x74",
                    "0x70",
                    "0x73",
                    "0x3a",
                    "0x2f",
                    "0x2f",
                    "0x6c",
                    "0x6f",
                    "0x63",
                    "0x61",
                    "0x6c",
                    "0x68",
                    "0x6f",
                    "0x73",
                    "0x74",
                    "0x3a",
                    "0x33",
                    "0x30",
                    "0x30",
                    "0x30",
                    "0x22",
                    "0x7d",
                    "0x1d",
                    "0x0",
                    "0xe11db4f020fd6b81d9589d8faa643e1c",
                    "0xf5b3980135a115d798f6883eddc5a5c1",
                    "0xdbed742a6b7b384633c748e4213279ed",
                    "0x6b4509da990cdaea592e8f6a836ed7e5",
                    "0x0",
                    "0x0",
                    "0x5b061cafcbc698ddb2f653206796b69f05a09163f4250ca09f54590954647fb",
                    "0x250645c8960f9ed37ca89c10df910330fb2b9b7d481d220e5dd140ba31a0682",
                    "0x4b506c76d2b94048c6abec9ef999d2df23e01d0f5538c4be2f15bf7954526e3",
                    "0x0",
                    "0x1e6a6f52e47fe42e024287b729bc47e58019fcc7e1cc8b141bb8d669b779b49",
                    "0x148cb9af1e7c0bb6f0dc6a1f6ae003b8206ab9d995f52d0943a159bef8df071",
                    "0xe2868f35910d1d50d3c02ee07aa02a2fe98feae0434566444c5dd527b60e1",
                    "0x1",
                    "0x4",
                    "0x485424d0e0336dee6ec30a907da1a6c463372243e2a6e70adf2657e8e4ca36b",
                    "0x56bb97a7a8c8da6e733e26ac91d21683b1e1e18c3e78fcc415e0da982d79437",
                    "0x5f1cbea946ff6641e9ada5c18ae5bd6a3624d662406d81ae60b27a65302fd3f",
                    "0x2949354fb20e55ec74b7d6afe1482f8f21f1f63cedc9bcb3b0fd165cb537236"
                ],
                "resource_bounds": {
                    "l1_gas": {
                        "max_amount": "0x31d7",
                        "max_price_per_unit": "0x32e5d0bef9aae"
                    },
                    "l2_gas": {
                        "max_amount": "0x0",
                        "max_price_per_unit": "0x0"
                    },
                    "l1_data_gas": {
                        "max_amount": "0x0",
                        "max_price_per_unit": "0x0"
                    }
                },
                "tip": "0x0",
                "paymaster_data": [],
                "account_deployment_data": [],
                "nonce_data_availability_mode": "L1",
                "fee_data_availability_mode": "L1"
            }
        ],
        [
            "SKIP_VALIDATE"
        ],
        {
            "block_number": 553000
        }
    ],
    "id": 0,
    "jsonrpc": "2.0"
}' | jq
*/
// Expected output:
/*
{
  "jsonrpc": "2.0",
  "error": {
    "code": 41,
    "message": "Transaction execution error",
    "data": {
      "transaction_index": 0,
      "execution_error": {
        "class_hash": "0xe2eb8f5672af4e6a4e8a8f1b44989685e668489b0a25437733756c5a34a1d6",
        "contract_address": "0x4a3140b5a24d4010e8e7bd02142ebd6a3e6daf8d8aa5568f0db45961938d1ad",
        "error": {
          "class_hash": "0xe2eb8f5672af4e6a4e8a8f1b44989685e668489b0a25437733756c5a34a1d6",
          "contract_address": "0x4a3140b5a24d4010e8e7bd02142ebd6a3e6daf8d8aa5568f0db45961938d1ad",
          "error": {
            "class_hash": "0x511dd75da368f5311134dee2356356ac4da1538d2ad18aa66d57c47e3757d59",
            "contract_address": "0x450a08ede3e6e2f182ef29b282ff822d11c87636f556f4be3c046394cf98970",
            "error": "0x617267656e742f696e76616c69642d74696d657374616d70 ('argent/invalid-timestamp')",
            "selector": "0x3dbc508ba4afd040c8dc4ff8a61113a7bcaf5eae88a6ba27b3c50578b3587e3"
          },
          "selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad"
        },
        "selector": "0x15d40a3d6ca2ac30f4031e42be28da9b056fef9bb7357ac5e85627ee876e5ad"
      }
    }
  },
  "id": 0
}
*/

func (h *Handler) EstimateFee(
	broadcastedTxns BroadcastedTransactionInputs,
	simulationFlags []rpcv6.SimulationFlag,
	id *BlockID,
) ([]FeeEstimate, http.Header, *jsonrpc.Error) {
	txnResults, httpHeader, err := h.simulateTransactions(
		id,
		broadcastedTxns.Data,
		append(simulationFlags, rpcv6.SkipFeeChargeFlag),
		true,
		true,
	)
	if err != nil {
		return nil, httpHeader, err
	}

	feeEstimates := make([]FeeEstimate, len(txnResults))
	for i := range feeEstimates {
		feeEstimates[i] = txnResults[i].FeeEstimation
	}

	return feeEstimates, httpHeader, nil
}

func (h *Handler) EstimateMessageFee(
	msg *rpcv6.MsgFromL1, id *BlockID,
) (FeeEstimate, http.Header, *jsonrpc.Error) {
	calldata := make([]*felt.Felt, len(msg.Payload)+1)
	// msg.From needs to be the first element
	calldata[0] = felt.NewFromBytes[felt.Felt](msg.From.Marshal())
	for i := range msg.Payload {
		calldata[i+1] = &msg.Payload[i]
	}
	tx := BroadcastedTransaction{
		Transaction: Transaction{
			Type: TxnL1Handler,
			// todo: this shouldn't have to be casted
			ContractAddress:    (*felt.Felt)(&msg.To),
			EntryPointSelector: &msg.Selector,
			CallData:           &calldata,
			Version:            &felt.Zero, // Needed for transaction hash calculation.
			Nonce:              &felt.Zero, // Needed for transaction hash calculation.
		},
		// Needed to marshal to blockifier type.
		// Must be greater than zero to successfully execute transaction.
		PaidFeeOnL1: new(felt.Felt).SetUint64(1),
	}

	bcTxn := [1]BroadcastedTransaction{tx}
	estimates, httpHeader, err := h.EstimateFee(
		BroadcastedTransactionInputs{Data: bcTxn[:]},
		nil,
		id,
	)
	if err != nil {
		if err.Code == rpccore.ErrTransactionExecutionError.Code {
			data := err.Data.(TransactionExecutionErrorData)
			return FeeEstimate{}, httpHeader, MakeContractError(data.ExecutionError)
		}
		return FeeEstimate{}, httpHeader, err
	}
	return estimates[0], httpHeader, nil
}

type ContractErrorData struct {
	RevertError json.RawMessage `json:"revert_error"`
}

func MakeContractError(err json.RawMessage) *jsonrpc.Error {
	return rpccore.ErrContractError.CloneWithData(ContractErrorData{
		RevertError: err,
	})
}
