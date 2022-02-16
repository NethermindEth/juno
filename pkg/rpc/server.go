package rpc

import (
	"context"
	"fmt"
	cmd "github.com/NethermindEth/juno/cmd/starknet"
	"log"
	"net/http"
)

type Server struct{}

func Handlers(end chan error) {
	mr := NewMethodRepositoryWithMethods(Server{})

	http.Handle("/rpc", mr)

	fmt.Println("Listening for connections .... ")
	if err := http.ListenAndServe(":8080", http.DefaultServeMux); err != nil {
		log.Fatalln(err)
	}
	end <- nil
}

// StarknetCall represents the handler of "starknet_call" rpc call
func (Server) StarknetCall(c context.Context, request cmd.RequestRPC) (cmd.ResultCall, error) {

	return []string{"Response", "of", "starknet_call"}, nil
}
