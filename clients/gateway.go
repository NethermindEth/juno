package clients

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

	"github.com/NethermindEth/juno/core/felt"
)

type GatewayClient struct {
	baseUrl string
}

func NewGatewayClient(baseUrl string) *GatewayClient {
	return &GatewayClient{
		baseUrl: baseUrl,
	}
}

// `buildQueryString` builds the query url with encoded parameters
func (c *GatewayClient) buildQueryString(endpoint string, args map[string]string) string {
	base, err := url.Parse(c.baseUrl)
	if err != nil {
		panic("Malformed feeder gateway base URL")
	}

	base.Path += endpoint

	params := url.Values{}
	for k, v := range args {
		params.Add(k, v)
	}
	base.RawQuery = params.Encode()

	return base.String()
}

// StateUpdate object returned by the gateway in JSON format for "get_state_update" endpoint
type StateUpdate struct {
	BlockHash *felt.Felt `json:"block_hash"`
	NewRoot   *felt.Felt `json:"new_root"`
	OldRoot   *felt.Felt `json:"old_root"`

	StateDiff struct {
		StorageDiffs map[string][]struct {
			Key   *felt.Felt `json:"key"`
			Value *felt.Felt `json:"value"`
		} `json:"storage_diffs"`

		Nonces            interface{} `json:"nonces"` // todo: define
		DeployedContracts []struct {
			Address   *felt.Felt `json:"address"`
			ClassHash *felt.Felt `json:"class_hash"`
		} `json:"deployed_contracts"`
		DeclaredContracts interface{} `json:"declared_contracts"` // todo: define
	} `json:"state_diff"`
}

func (c *GatewayClient) GetStateUpdate(blockNumber uint64) (*StateUpdate, error) {
	queryUrl := c.buildQueryString("get_state_update", map[string]string{
		"blockNumber": strconv.FormatUint(blockNumber, 10),
	})

	res, err := http.Get(queryUrl)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	update := new(StateUpdate)
	if err = json.Unmarshal(body, update); err != nil {
		return nil, err
	}

	return update, nil
}
