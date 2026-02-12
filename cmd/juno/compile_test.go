package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/starknet"
	"github.com/stretchr/testify/require"
)

func TestCompileSierraCmd(t *testing.T) {
	t.Run("invalid input returns error", func(t *testing.T) {
		cmd := juno.CompileSierraCmd()
		cmd.SetIn(bytes.NewReader([]byte("not json")))
		cmd.SetOut(&bytes.Buffer{})

		err := cmd.Execute()
		require.Error(t, err)
	})

	t.Run("empty sierra class returns error", func(t *testing.T) {
		input, err := json.Marshal(starknet.SierraClass{})
		require.NoError(t, err)

		cmd := juno.CompileSierraCmd()
		cmd.SetIn(bytes.NewReader(input))

		var stdout bytes.Buffer
		cmd.SetOut(&stdout)

		err = cmd.Execute()
		require.Error(t, err)
	})
}
