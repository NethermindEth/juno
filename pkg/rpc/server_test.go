package rpc

import (
	"bytes"
	"context"
	"github.com/NethermindEth/juno/internal/log"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	time "time"
)

func getServerHandler() *MethodRepository {
	server := NewMethodRepositoryWithMethods(HandlerRPC{})
	return server
}

type rpcTest struct {
	request  string
	response string
}

func testServer(t *testing.T, tests []rpcTest) {
	server := getServerHandler()

	for i, v := range tests {
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer([]byte(v.request)))
		w := httptest.NewRecorder()
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(w, req)
		res := w.Result()
		data, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Errorf("expected error to be nil got %v", err)
			_ = res.Body.Close()
		}
		s := string(data)
		if s != v.response {
			t.Errorf("expected %v, got %v", v.response, string(data))
			_ = res.Body.Close()
		}
		t.Log("Executed test ", i)
	}
}

func TestRPCServer(t *testing.T) {
	testServer(t, []rpcTest{
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"345\",\"method\":\"echo\",\"params\":[\"Hello Echo\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":\"Hello Echo\",\"id\":\"345\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"345\",\"method\":\"starknet_getEvents\",\"params\":[{\"fromBlock\":0,\"toBlock\":0,\"address\":\"\",\"keys\":null,\"page_size\":0,\"page_number\":0}]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"EmittedEventArray\":null,\"page_number\":0},\"id\":\"345\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":[{\"callata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"34\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"34\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":9{[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]}}",
			response: "{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32700,\"message\":\"Parse error\"}}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"0\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"pending\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"pending\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"0\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"2\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"pending\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"pending\",\"parent_hash\":\"TXN_HASH\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"2\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"1\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"pending\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"1\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"3\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"pending\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"3\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"4\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"4\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"5\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"5\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"6\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"TXN_HASH\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"6\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"7\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\",\"TXN_HASH\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"7\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"8\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\",\"FULL_TXNS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"FULL_TXNS\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"8\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"9\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\",\"FULL_TXNS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"9\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"10\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"latest\",\"FULL_TXN_AND_RECEIPTS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"latest\",\"parent_hash\":\"FULL_TXN_AND_RECEIPTS\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"10\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"11\",\"method\":\"starknet_getBlockByNumber\",\"params\":[\"latest\",\"FULL_TXN_AND_RECEIPTS\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"11\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"39\",\"method\":\"starknet_protocolVersion\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":\"Here the Protocol Version\",\"id\":\"39\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"40\",\"method\":\"starknet_syncing\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"starting_block\":\"\",\"current_block\":\"\",\"highest_block\":\"\"},\"id\":\"40\"}\n",
		},
		{
			request:  "[{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"35\",\"method\":\"starknet_call\",\"params\":[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"pending\"]}]",
			response: "[{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"34\"},{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"35\"}]\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"12\",\"method\":\"starknet_getBlockByHash\",\"params\":[\"0x7d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"0x7d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"12\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"13\",\"method\":\"starknet_getBlockByNumber\",\"params\":[41000]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"parent_hash\":\"\",\"block_number\":0,\"status\":\"\",\"sequencer\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"transactions\":null},\"id\":\"13\"}\n",
		},
		{
			request:  "[{\"jsonrpc\":\"2.0\",\"id\":\"14\",\"method\":\"starknet_getStateUpdateByHash\",\"params\":[\"latest\"]},{\"jsonrpc\":\"2.0\",\"id\":\"15\",\"method\":\"starknet_getStateUpdateByHash\",\"params\":[\"0x7d328a71faf48c5c3857e99f20a77b18522480956d1cd5bff1ff2df3c8b427b\"]}]",
			response: "[{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"state_diff\":{\"storage_diffs\":null,\"contracts\":null}},\"id\":\"14\"},{\"jsonrpc\":\"2.0\",\"result\":{\"block_hash\":\"\",\"new_root\":\"\",\"old_root\":\"\",\"accepted_time\":0,\"state_diff\":{\"storage_diffs\":null,\"contracts\":null}},\"id\":\"15\"}]\n",
		},
		{
			request:  "[{\"jsonrpc\":\"2.0\",\"id\":\"16\",\"method\":\"starknet_getStorageAt\",\"params\":[\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\", \"0x0206F38F7E4F15E87567361213C28F235CCCDAA1D7FD34C9DB1DFE9489C6A091\", \"latest\"]},{\"jsonrpc\":\"2.0\",\"id\":\"17\",\"method\":\"starknet_getStorageAt\",\"params\":[\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\", \"0x0206F38F7E4F15E87567361213C28F235CCCDAA1D7FD34C9DB1DFE9489C6A091\", \"pending\"]},{\"jsonrpc\":\"2.0\",\"id\":\"18\",\"method\":\"starknet_getStorageAt\",\"params\":[\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\", \"0x0206F38F7E4F15E87567361213C28F235CCCDAA1D7FD34C9DB1DFE9489C6A091\", \"0x3871c8a0c3555687515a07f365f6f5b1d8c2ae953f7844575b8bde2b2efed27\"]}]'",
			response: "[{\"jsonrpc\":\"2.0\",\"result\":\"Storage\",\"id\":\"16\"},{\"jsonrpc\":\"2.0\",\"result\":\"Storage\",\"id\":\"17\"},{\"jsonrpc\":\"2.0\",\"result\":\"Storage\",\"id\":\"18\"}]\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"19\",\"method\":\"starknet_getTransactionByHash\",\"params\":[\"0x74ec6667e6057becd3faff77d9ab14aecf5dde46edb7c599ee771f70f9e80ba\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"19\"}\n",
		},
		{
			request:  "[{\"jsonrpc\":\"2.0\",\"id\":\"20\",\"method\":\"starknet_getTransactionByBlockHashAndIndex\",\"params\":[\"latest\", 0]},\n{\"jsonrpc\":\"2.0\",\"id\":\"21\",\"method\":\"starknet_getTransactionByBlockNumberAndIndex\",\"params\":[\"latest\", 0]},\n{\"jsonrpc\":\"2.0\",\"id\":\"22\",\"method\":\"starknet_getTransactionByBlockHashAndIndex\",\"params\":[\"pending\", 0]},\n{\"jsonrpc\":\"2.0\",\"id\":\"23\",\"method\":\"starknet_getTransactionByBlockNumberAndIndex\",\"params\":[\"pending\", 0]},\n{\"jsonrpc\":\"2.0\",\"id\":\"24\",\"method\":\"starknet_getTransactionByBlockHashAndIndex\",\"params\":[\"0x3871c8a0c3555687515a07f365f6f5b1d8c2ae953f7844575b8bde2b2efed27\", 4]},\n{\"jsonrpc\":\"2.0\",\"id\":\"25\",\"method\":\"starknet_getTransactionByBlockNumberAndIndex\",\"params\":[21348, 4]}]",
			response: "[{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"20\"},{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"21\"},{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"22\"},{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"23\"},{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"24\"},{\"jsonrpc\":\"2.0\",\"error\":{\"code\":-32602,\"message\":\"Invalid params\"},\"id\":\"25\"}]\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"22\",\"method\":\"starknet_getTransactionByBlockNumberAndIndex\",\"params\":[\"pending\", 0]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"contract_address\":\"\",\"entry_point_selector\":\"\",\"calldata\":null,\"txn_hash\":\"\"},\"id\":\"22\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"26\",\"method\":\"starknet_getTransactionReceipt\",\"params\":[\"0x74ec6667e6057becd3faff77d9ab14aecf5dde46edb7c599ee771f70f9e80ba\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"txn_hash\":\"\",\"status\":\"\",\"status_data\":\"\",\"messages_sent\":null,\"l1_origin_message\":{\"from_address\":\"\",\"payload\":null},\"events\":{\"keys\":null,\"data\":null,\"from_address\":\"\"}},\"id\":\"26\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"27\",\"method\":\"starknet_getCode\",\"params\":[\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\"]}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"bytecode\":null,\"abi\":\"\"},\"id\":\"27\"}\n",
		},
		{
			request:  "[{\"jsonrpc\":\"2.0\",\"id\":\"28\",\"method\":\"starknet_getBlockTransactionCountByHash\",\"params\":[\"latest\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"29\",\"method\":\"starknet_getBlockTransactionCountByNumber\",\"params\":[\"latest\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"30\",\"method\":\"starknet_getBlockTransactionCountByHash\",\"params\":[\"pending\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"31\",\"method\":\"starknet_getBlockTransactionCountByNumber\",\"params\":[\"pending\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"32\",\"method\":\"starknet_getBlockTransactionCountByHash\",\"params\":[\"0x3871c8a0c3555687515a07f365f6f5b1d8c2ae953f7844575b8bde2b2efed27\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"33\",\"method\":\"starknet_getBlockTransactionCountByNumber\",\"params\":[21348]}]",
			response: "[{\"jsonrpc\":\"2.0\",\"result\":{\"TransactionCount\":0},\"id\":\"28\"},{\"jsonrpc\":\"2.0\",\"result\":{\"TransactionCount\":0},\"id\":\"29\"},{\"jsonrpc\":\"2.0\",\"result\":{\"TransactionCount\":0},\"id\":\"30\"},{\"jsonrpc\":\"2.0\",\"result\":{\"TransactionCount\":0},\"id\":\"31\"},{\"jsonrpc\":\"2.0\",\"result\":{\"TransactionCount\":0},\"id\":\"32\"},{\"jsonrpc\":\"2.0\",\"result\":{\"TransactionCount\":0},\"id\":\"33\"}]\n",
		},
		{
			request:  "[{\"jsonrpc\":\"2.0\",\"id\":\"34\",\"method\":\"starknet_call\",\"params\":[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"latest\"]},\n{\"jsonrpc\":\"2.0\",\"id\":\"35\",\"method\":\"starknet_call\",\"params\":[{\"calldata\":[\"0x1234\"],\"contract_address\":\"0x6fbd460228d843b7fbef670ff15607bf72e19fa94de21e29811ada167b4ca39\",\n\"entry_point_selector\":\"0x362398bec32bc0ebb411203221a35a0301193a96f317ebe5e40be9f60d15320\"}, \"pending\"]}]",
			response: "[{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"34\"},{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"35\"}]\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"36\",\"method\":\"starknet_blockNumber\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":0,\"id\":\"36\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"37\",\"method\":\"starknet_chainId\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":\"Here the ChainID\",\"id\":\"37\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"38\",\"method\":\"starknet_pendingTransactions\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":null,\"id\":\"38\"}\n",
		},
		{
			request:  "{\"jsonrpc\":\"2.0\",\"id\":\"40\",\"method\":\"starknet_syncing\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":{\"starting_block\":\"\",\"current_block\":\"\",\"highest_block\":\"\"},\"id\":\"40\"}\n",
		},
	})
}

func TestServer(t *testing.T) {
	server := NewServer(":8080")
	go func() {
		_ = server.ListenAndServe(logger)
	}()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	server.Close(ctx)
	cancel()
}

func init() {

	logger = log.GetLogger()
}
