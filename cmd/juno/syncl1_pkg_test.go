package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestNewSyncL1Cmd(t *testing.T) {
	tests := map[string]struct {
		args []string
		want int // if negative, expect error
	}{
		"no args": {
			args: []string{},
			want: -1,
		},
		"single empty arg": {
			args: []string{""},
			want: -1,
		},
		"single negative arg": {
			args: []string{"-1"},
			want: -1,
		},
		"two args": {
			args: []string{"1", "1"},
			want: -1,
		},
		"zero": {
			args: []string{"0"},
			want: 0,
		},
		"large number": {
			args: []string{"1232882"},
			want: 1232882,
		},
	}

	for description, test := range tests {
		t.Run(description, func(t *testing.T) {
			var endBlock uint64
			cmd := newSyncL1Cmd(&endBlock, func(_ *cobra.Command, _ []string) error { return nil })
			cmd.SetArgs(test.args)
			err := cmd.Execute()
			if test.want < 0 {
				require.Error(t, err)
				return
			}
			require.Equal(t, uint64(test.want), endBlock)
		})
	}
}
