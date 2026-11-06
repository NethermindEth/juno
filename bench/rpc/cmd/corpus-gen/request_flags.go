package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const (
	includeProofFactsFlag      = "INCLUDE_PROOF_FACTS"
	includeLastUpdateBlockFlag = "INCLUDE_LAST_UPDATE_BLOCK"
	returnInitialReadsFlag     = "RETURN_INITIAL_READS"
)

// requestFlagsMinVersion is the first spec version with response_flags and
// trace_flags.
var requestFlagsMinVersion = [2]int{0, 10}

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
		majorMinor, err := specMajorMinor(version)
		if err != nil {
			return err
		}
		if slices.Compare(majorMinor[:], requestFlagsMinVersion[:]) < 0 {
			return fmt.Errorf("--%s needs spec v%d.%d or later (source is v%s)",
				name, requestFlagsMinVersion[0], requestFlagsMinVersion[1], version)
		}
		*flags = []string{flag}
		return nil
	})
}

// specMajorMinor parses the major and minor parts of a spec version such as
// "0.10.0".
func specMajorMinor(version string) ([2]int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return [2]int{}, fmt.Errorf("unexpected spec version %q", version)
	}
	major, errMajor := strconv.Atoi(parts[0])
	minor, errMinor := strconv.Atoi(parts[1])
	if errMajor != nil || errMinor != nil {
		return [2]int{}, fmt.Errorf("unexpected spec version %q", version)
	}
	return [2]int{major, minor}, nil
}
