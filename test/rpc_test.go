package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/NethermindEth/juno/pkg/rpc"
)

func getServerHandler() *rpc.HandlerJsonRpc {
	return rpc.NewHandlerJsonRpc(rpc.HandlerRPC{})
}

type rpcTest struct {
	Request  string `json:"request"`
	Response string `json:"response"`
}

func testServer(t *testing.T, tests []rpcTest) {
	server := getServerHandler()

	for i, v := range tests {
		req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBuffer([]byte(v.Request)))
		w := httptest.NewRecorder()
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(w, req)
		res := w.Result()
		data, err := io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("expected error to be nil got %v", err)
			_ = res.Body.Close()
		}
		s := string(data)
		if s != v.Response {
			t.Errorf("expected %v, got %v", v.Response, string(data))
			_ = res.Body.Close()
		}
		t.Log("Executed test ", i)
	}
}

func TestRPCServer(t *testing.T) {
	jsonFile, err := os.Open("rpc_tests.json")
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	// defer the closing of our jsonFile so that we can parse it later on
	defer jsonFile.Close()
	// read our opened jsonFile as a byte array.
	byteValue, _ := io.ReadAll(jsonFile)

	// we initialize our Users array
	var tests []rpcTest

	// we unmarshal our byteArray which contains our
	// jsonFile's content into 'users' which we defined above
	err = json.Unmarshal(byteValue, &tests)
	if err != nil {
		return
	}
	testServer(t, tests)
}

func TestServer(t *testing.T) {
	server := rpc.NewServer(":8080")
	go func() {
		_ = server.ListenAndServe()
	}()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	server.Close(ctx)
	cancel()
}
