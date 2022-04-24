package abi

import "github.com/NethermindEth/juno/pkg/db"

func InitAbiService(db db.Databaser) {
	database = db
}
