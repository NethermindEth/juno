package rpc

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMe(t *testing.T) {
	trace := struct {
		V *string `json:"v,omitempty"`
	}{
		V: nil,
	}

	buff, err := json.Marshal(trace)
	require.NoError(t, err)

	fmt.Println(string(buff))
}
