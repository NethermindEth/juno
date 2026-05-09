package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// flagCategoryAnnotation is the pflag annotation key used to tag a flag with a
// display category. The grouped usage func partitions flags by this annotation
// when rendering --help.
const flagCategoryAnnotation = "juno_category"

const (
	catLogging       = "Logging"
	catHTTPRPC       = "HTTP RPC"
	catWebSocket     = "WebSocket RPC"
	catGRPC          = "gRPC"
	catObservability = "Metrics & Profiling"
	catHTTPUpdate    = "HTTP Update Endpoint"
	catDatabase      = "Database"
	catNetwork       = "Network & L1"
	catCustomNetwork = "Custom Network"
	catGateway       = "Gateway"
	catP2P           = "P2P (experimental)"
	catSequencer     = "Sequencer"
	catSyncPolling   = "Sync & Polling"
	catPruning       = "Pruning"
	catVMCompile     = "VM & Compilation"
	catTxCache       = "Transaction Cache"
	catMisc          = "Plugins & Misc"
	catOther         = "Other"
)

// setCategory tags every named flag on cmd with the given display category.
// It panics if a flag is not registered, surfacing wiring mistakes at startup.
func setCategory(cmd *cobra.Command, category string, flagNames ...string) {
	for _, name := range flagNames {
		err := cmd.Flags().SetAnnotation(name, flagCategoryAnnotation, []string{category})
		if err != nil {
			panic(fmt.Errorf("setCategory %q on %q: %w", category, name, err))
		}
	}
}

func writeUsageLine(b *strings.Builder, cmd *cobra.Command) {
	b.WriteString("Usage:")
	if cmd.Runnable() {
		b.WriteString("\n  " + cmd.UseLine())
	}
	if cmd.HasAvailableSubCommands() {
		b.WriteString("\n  " + cmd.CommandPath() + " [command]")
	}
	b.WriteByte('\n')
}

func writeAliases(b *strings.Builder, cmd *cobra.Command) {
	if len(cmd.Aliases) > 0 {
		b.WriteString("\nAliases:\n  " + cmd.NameAndAliases() + "\n")
	}
}

func writeExamples(b *strings.Builder, cmd *cobra.Command) {
	if cmd.HasExample() {
		b.WriteString("\nExamples:\n" + cmd.Example + "\n")
	}
}

func writeAvailableCommands(b *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasAvailableSubCommands() {
		return
	}
	b.WriteString("\nAvailable Commands:\n")
	pad := cmd.NamePadding()
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() || c.Name() == "help" {
			fmt.Fprintf(b, "  %s %s\n", rpad(c.Name(), pad), c.Short)
		}
	}
}

func writeFlagsSection(b *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasAvailableLocalFlags() {
		return
	}
	if !hasCategorisedFlags(cmd.LocalFlags()) {
		// Subcommands inherit this usage func via Cobra's parent walk but
		// don't tag their flags. Render the standard flat list for them.
		b.WriteString("\nFlags:\n")
		b.WriteString(strings.TrimRight(cmd.LocalFlags().FlagUsages(), " \t\n"))
		b.WriteByte('\n')
		return
	}

	// orderedCategories controls render order. catOther is appended after these if
	// non-empty, so a forgotten annotation is visible rather than silently dropped.
	orderedCategories := []string{
		catLogging,
		catHTTPRPC,
		catWebSocket,
		catGRPC,
		catObservability,
		catHTTPUpdate,
		catDatabase,
		catNetwork,
		catCustomNetwork,
		catGateway,
		catP2P,
		catSequencer,
		catSyncPolling,
		catPruning,
		catVMCompile,
		catTxCache,
		catMisc,
	}

	buckets := bucketFlags(cmd.LocalFlags())
	for _, cat := range orderedCategories {
		writeFlagSection(b, cat, buckets[cat])
	}
	writeFlagSection(b, catOther, buckets[catOther])
}

func writeGlobalFlags(b *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasAvailableInheritedFlags() {
		return
	}
	b.WriteString("\nGlobal Flags:\n")
	b.WriteString(strings.TrimRight(cmd.InheritedFlags().FlagUsages(), " \t\n"))
	b.WriteByte('\n')
}

func writeAdditionalHelpTopics(b *strings.Builder, cmd *cobra.Command) {
	if !cmd.HasHelpSubCommands() {
		return
	}
	b.WriteString("\nAdditional help topics:\n")
	pad := cmd.CommandPathPadding()
	for _, c := range cmd.Commands() {
		if c.IsAdditionalHelpTopicCommand() {
			fmt.Fprintf(b, "  %s %s\n", rpad(c.CommandPath(), pad), c.Short)
		}
	}
}

func writeFooter(b *strings.Builder, cmd *cobra.Command) {
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(
			b,
			"\nUse \"%s [command] --help\" for more information about a command.\n",
			cmd.CommandPath(),
		)
	}
}

// writeGroupedUsage is a drop-in replacement for cobra's default usage function
// that splits Flags into one section per juno_category annotation.
func writeGroupedUsage(cmd *cobra.Command) error {
	var b strings.Builder
	writeUsageLine(&b, cmd)
	writeAliases(&b, cmd)
	writeExamples(&b, cmd)
	writeAvailableCommands(&b, cmd)
	writeFlagsSection(&b, cmd)
	writeGlobalFlags(&b, cmd)
	writeAdditionalHelpTopics(&b, cmd)
	writeFooter(&b, cmd)
	_, err := io.WriteString(cmd.OutOrStderr(), b.String())
	return err
}

// hasCategorisedFlags reports whether any flag in the set carries the
// juno_category annotation. Used to decide whether to render grouped sections
// or fall back to a single flat "Flags:" block.
func hasCategorisedFlags(fs *pflag.FlagSet) bool {
	found := false
	fs.VisitAll(func(f *pflag.Flag) {
		if found {
			return
		}
		if vals, ok := f.Annotations[flagCategoryAnnotation]; ok && len(vals) > 0 {
			found = true
		}
	})
	return found
}

// bucketFlags groups visible flags by their juno_category annotation. Hidden
// flags are skipped, matching pflag.FlagUsages behaviour. Cobra adds --help
// lazily on Execute(), so it cannot be tagged via setCategory; route it into
// catMisc so it lands alongside --config and --plugin-path.
func bucketFlags(fs *pflag.FlagSet) map[string][]*pflag.Flag {
	buckets := make(map[string][]*pflag.Flag)
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		cat := catOther
		if vals, ok := f.Annotations[flagCategoryAnnotation]; ok && len(vals) > 0 {
			cat = vals[0]
		} else if f.Name == "help" {
			cat = catMisc
		}
		buckets[cat] = append(buckets[cat], f)
	})
	return buckets
}

// writeFlagSection renders one category's flags using pflag's standard column
// alignment, by adding the matching flag pointers to a throwaway FlagSet.
func writeFlagSection(b *strings.Builder, category string, flags []*pflag.Flag) {
	if len(flags) == 0 {
		return
	}
	tmp := pflag.NewFlagSet(category, pflag.ContinueOnError)
	for _, f := range flags {
		tmp.AddFlag(f)
	}
	fmt.Fprintf(b, "\n%s Flags:\n", category)
	b.WriteString(strings.TrimRight(tmp.FlagUsages(), " \t\n"))
	b.WriteByte('\n')
}

func rpad(s string, padding int) string {
	if len(s) >= padding {
		return s
	}
	return s + strings.Repeat(" ", padding-len(s))
}
