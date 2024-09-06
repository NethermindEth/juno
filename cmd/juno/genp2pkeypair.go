package main

import (
	"encoding/hex"
	"fmt"

	"github.com/NethermindEth/juno/p2p"
	"github.com/spf13/cobra"
)

func GenP2PKeyPair() *cobra.Command {
	return &cobra.Command{
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
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "P2P Private Key:", string(privHex))
			if err != nil {
				return err
			}

			rawPub, err := pub.Raw()
			if err != nil {
				return err
			}

			pubHex := make([]byte, hex.EncodedLen(len(rawPub)))
			hex.Encode(pubHex, rawPub)
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "P2P Public Key:", string(pubHex))
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "P2P PeerID:", id)
			return err
		},
	}
}
