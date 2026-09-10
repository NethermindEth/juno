package main

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	includeProofFactsFlag      = "INCLUDE_PROOF_FACTS"
	includeLastUpdateBlockFlag = "INCLUDE_LAST_UPDATE_BLOCK"
	returnInitialReadsFlag     = "RETURN_INITIAL_READS"
	skipValidateFlag           = "SKIP_VALIDATE"
	skipFeeChargeFlag          = "SKIP_FEE_CHARGE"
)

type txnFlags []string

func (f *txnFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlags(cmd, (*[]string)(f), includeProofFactsFlag)
}

type storageAtFlags []string

func (f *storageAtFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlags(cmd, (*[]string)(f), includeLastUpdateBlockFlag)
}

type traceFlags []string

func (f *traceFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlags(cmd, (*[]string)(f), returnInitialReadsFlag)
}

type estimateFlags []string

func (f *estimateFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlags(cmd, (*[]string)(f), skipValidateFlag)
}

type simulateFlags []string

func (f *simulateFlags) bind(cmd *cobra.Command, _ *rpcClient) {
	addRequestFlags(cmd, (*[]string)(f), skipValidateFlag, skipFeeChargeFlag, returnInitialReadsFlag)
}

func addRequestFlags(cmd *cobra.Command, flags *[]string, names ...string) {
	*flags = []string{}
	includes := make([]*bool, len(names))
	for i, name := range names {
		includes[i] = cmd.Flags().Bool(
			strings.ToLower(strings.ReplaceAll(name, "_", "-")),
			false,
			"Add the "+name+" flag to each request.",
		)
	}
	chainPreRunE(cmd, func() error {
		for i, include := range includes {
			if *include {
				*flags = append(*flags, names[i])
			}
		}
		return nil
	})
}
