package main_test

import (
	"strings"
	"testing"

	juno "github.com/NethermindEth/juno/cmd/juno"
	"github.com/NethermindEth/juno/node"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEveryRootFlagIsCategorised(t *testing.T) {
	cmd := juno.NewCmd(new(node.Config), func(*cobra.Command, []string) error { return nil })

	var missing []string
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if vals, ok := f.Annotations[juno.FlagCategoryAnnotation]; !ok || len(vals) == 0 {
			missing = append(missing, f.Name)
		}
	})
	require.Empty(
		t,
		missing,
		"flags missing %q annotation; tag them with setCategory in NewCmd: %v",
		juno.FlagCategoryAnnotation,
		missing,
	)
}

func TestHelpOutputIsGrouped(t *testing.T) {
	cmd := juno.NewCmd(new(node.Config), func(*cobra.Command, []string) error { return nil })

	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	require.NoError(t, cmd.ExecuteContext(t.Context()))

	out := buf.String()
	for _, header := range juno.FlagCategories() {
		assert.Contains(
			t,
			out,
			header+" Flags:",
			"help output is missing category header %q",
			header,
		)
	}
}
