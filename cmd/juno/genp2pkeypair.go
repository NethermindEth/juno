package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/NethermindEth/juno/p2p"
	"github.com/spf13/cobra"
)

func GenP2PKeyPair() *cobra.Command {
	var filename string

	cmd := &cobra.Command{
		Use:   "genp2pkeypair",
		Short: "Generate private key pair for p2p.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			priv, pub, id, err := p2p.GenKeyPair()
			if err != nil {
				return err
			}

			rawPriv, err := priv.Raw()
			if err != nil {
				return err
			}

			privHex := make([]byte, hex.EncodedLen(len(rawPriv)))
			hex.Encode(privHex, rawPriv)

			rawPub, err := pub.Raw()
			if err != nil {
				return err
			}

			pubHex := make([]byte, hex.EncodedLen(len(rawPub)))
			hex.Encode(pubHex, rawPub)

			output := "P2P Private Key: " + string(privHex) + "\n" +
				"P2P Public Key: " + string(pubHex) + "\n" +
				"P2P PeerID: " + id.String()

			writeTo := cmd.OutOrStdout()
			if filename != "" {
				file, err := os.Create(filename)
				if err != nil {
					return err
				}
				defer file.Close()

				writeTo = file
			}
			fmt.Fprintln(writeTo, output)

			return err
		},
	}

	cmd.Flags().StringVarP(&filename, "file", "f", "", "Optional file to write the keys to")

	return cmd
}
