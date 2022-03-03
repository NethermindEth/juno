package rpc

import (
	"bytes"
	"context"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"github.com/NethermindEth/juno/internal/log"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
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

	for _, v := range tests {
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

	}
}

func TestHandlerRPC_StarknetCall(t *testing.T) {
	testServer(t, []rpcTest{
		{
			request:  "{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"starknet_call\",\"params\": [{\"request\":{\"contract_address\":\"address\",\"entry_point_selector\":\"selector\",\"CallData\":[\"string1\",\"string2\"]}},{\"block_hash\":\"latest\"}],  \"id\": \"243a718a-2ebb-4e32-8cc8-210c39e8a14b\"}",
			response: "{\"jsonrpc\":\"2.0\",\"result\":[\"Response\",\"of\",\"starknet_call\"],\"id\":\"243a718a-2ebb-4e32-8cc8-210c39e8a14b\"}\n",
		},
	})
}

var handlerRpc *HandlerRPC

func init() {
	handlerRpc = getHandler()
	logger = log.GetLogger()
}

func getHandler() *HandlerRPC {
	return &HandlerRPC{}
}

type starknetCallParams struct {
	ctx       context.Context
	request   cmd.FunctionCall
	blockHash cmd.BlockHashOrTag
}

type starknetCallResponse struct {
	result cmd.ResultCall
	error  error
}

func checkResultCallEqual(r1, r2 cmd.ResultCall) bool {
	if len(r1) != len(r2) {
		return false
	}
	for i, v := range r1 {
		if v != r2[i] {
			return false
		}
	}
	return true
}

func TestStarknetCall(t *testing.T) {
	test := []struct {
		params   starknetCallParams
		response starknetCallResponse
	}{
		{
			starknetCallParams{
				ctx:       context.Background(),
				request:   cmd.FunctionCall{},
				blockHash: cmd.BlockHashOrTag{},
			},
			starknetCallResponse{
				result: []string{"Response", "of", "starknet_call"},
				error:  nil,
			},
		},
	}
	for _, v := range test {
		r, err := handlerRpc.StarknetCall(v.params.ctx, v.params.request, v.params.blockHash)
		if !checkResultCallEqual(r, v.response.result) || err != v.response.error {
			t.Fail()
		}
	}
}
