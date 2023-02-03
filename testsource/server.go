package testsource

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"

	"github.com/NethermindEth/juno/utils"
)

func newTestGatewayServer(network utils.Network) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queryMap, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}

		base := wd[:strings.LastIndex(wd, "juno")+4]
		queryArg := ""
		dir := ""

		switch {
		case strings.HasSuffix(r.URL.Path, "get_block"):
			dir = "block"
			queryArg = "blockNumber"
		case strings.HasSuffix(r.URL.Path, "get_state_update"):
			dir = "state_update"
			queryArg = "blockNumber"
		case strings.HasSuffix(r.URL.Path, "get_transaction"):
			dir = "transaction"
			queryArg = "transactionHash"
		case strings.HasSuffix(r.URL.Path, "get_class_by_hash"):
			dir = "class"
			queryArg = "classHash"
		}

		fileName, found := queryMap[queryArg]
		if !found {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		path := fmt.Sprintf("%s/testsource/testdata/%s/%s/%s.json", base, network.String(), dir, fileName[0])
		read, err := os.ReadFile(path)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(read)
	}))

	return srv
}
