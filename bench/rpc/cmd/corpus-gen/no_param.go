package main

import (
	"context"
	"math/rand/v2"

	"github.com/spf13/cobra"
)

func newNoParamCmds(cfg *rootConfig, methods ...string) []*cobra.Command {
	cmds := make([]*cobra.Command, 0, len(methods))
	for _, method := range methods {
		cmds = append(cmds, newNoParamCmd(cfg, method))
	}
	return cmds
}

func newNoParamCmd(cfg *rootConfig, method string) *cobra.Command {
	return &cobra.Command{
		Use:     commandName(method),
		Aliases: []string{method},
		GroupID: methodsGroupID,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gen := func(context.Context, *rand.Rand) (any, error) { return nil, nil }
			client := newRPCClient(cfg.sourceURL, cfg.concurrency)
			return runCorpus(cmd, cfg, client, method, struct{}{}, gen)
		},
	}
}
