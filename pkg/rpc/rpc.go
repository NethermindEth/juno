package rpc

import (
	"fmt"
	"github.com/osamingo/jsonrpc/v2"
	"log"
	"net/http"
)

func Handlers(end chan error) {
	mr := jsonrpc.NewMethodRepository()
	dispatcher := NewRPCDispatcher(
		[]HandleParamsResulter{
			EchoHandler{},
		},
	)

	for _, h := range dispatcher.Handlers() {
		err := mr.RegisterMethod(dispatcher.MethodName(h), h, h.Params(), h.Result())
		if err != nil {
			fmt.Println("Error registering Method")
			end <- err
		}
	}

	http.Handle("/rpc", mr)
	http.HandleFunc("/rpc/debug", mr.ServeDebug)

	fmt.Println("Listening for connections .... ")
	if err := http.ListenAndServe(":8080", http.DefaultServeMux); err != nil {
		end <- err
		log.Fatalln(err)
	}
	end <- nil
}
