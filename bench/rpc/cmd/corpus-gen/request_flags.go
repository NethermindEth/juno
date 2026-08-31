package main

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	includeProofFactsFlag      = "INCLUDE_PROOF_FACTS"
	includeLastUpdateBlockFlag = "INCLUDE_LAST_UPDATE_BLOCK"
	returnInitialReadsFlag     = "RETURN_INITIAL_READS"
)

type txnFlags []string

func (f *txnFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlag(cmd, (*[]string)(f), includeProofFactsFlag)
}

type storageAtFlags []string

func (f *storageAtFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlag(cmd, (*[]string)(f), includeLastUpdateBlockFlag)
}

type traceFlags []string

func (f *traceFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlag(cmd, (*[]string)(f), returnInitialReadsFlag)
}

func addRequestFlag(cmd *cobra.Command, flags *[]string, flag string) {
	*flags = []string{}
	include := cmd.Flags().Bool(
		strings.ToLower(strings.ReplaceAll(flag, "_", "-")),
		false,
		"Add the "+flag+" flag to each request.",
	)
	chainPreRunE(cmd, func() error {
		if *include {
			*flags = []string{flag}
		}
		return nil
	})
}
