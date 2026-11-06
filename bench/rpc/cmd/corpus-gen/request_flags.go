package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	includeProofFactsFlag      = "INCLUDE_PROOF_FACTS"
	includeLastUpdateBlockFlag = "INCLUDE_LAST_UPDATE_BLOCK"
	returnInitialReadsFlag     = "RETURN_INITIAL_READS"
)

// requestFlagsMinMinor is the first spec minor version (v0.x) with
// response_flags and trace_flags.
const requestFlagsMinMinor = 10

type txnFlags []string

func (f *txnFlags) bind(cmd *cobra.Command, client *rpcClient) {
	addRequestFlag(cmd, client, (*[]string)(f), includeProofFactsFlag)
}

type storageAtFlags []string

func (f *storageAtFlags) bind(cmd *cobra.Command, client *rpcClient) {
	addRequestFlag(cmd, client, (*[]string)(f), includeLastUpdateBlockFlag)
}

type traceFlags []string

func (f *traceFlags) bind(cmd *cobra.Command, client *rpcClient) {
	addRequestFlag(cmd, client, (*[]string)(f), returnInitialReadsFlag)
}

func addRequestFlag(cmd *cobra.Command, client *rpcClient, flags *[]string, flag string) {
	*flags = []string{}
	name := strings.ToLower(strings.ReplaceAll(flag, "_", "-"))
	include := cmd.Flags().Bool(name, false, "Add the "+flag+" flag to each request.")
	chainPreRunE(cmd, func() error {
		if !*include {
			return nil
		}
		version, err := client.specVersion(cmd.Context())
		if err != nil {
			return fmt.Errorf("fetch spec version: %w", err)
		}
		minor, err := specMinor(version)
		if err != nil {
			return err
		}
		if minor < requestFlagsMinMinor {
			return fmt.Errorf("--%s needs spec v0.%d or later (source is v%s)",
				name, requestFlagsMinMinor, version)
		}
		*flags = []string{flag}
		return nil
	})
}

// specMinor returns the minor part of a v0.x spec version such as "0.10.0".
func specMinor(version string) (int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 || parts[0] != "0" {
		return 0, fmt.Errorf("unexpected spec version %q", version)
	}
	return strconv.Atoi(parts[1])
}
