package main

import (
	"fmt"

	sync "github.com/NethermindEth/juno/customsync"
)

func main() {
	cs := sync.NewCustomSynchronizer()
	if err := cs.Run(); err != nil {
		fmt.Println(err)
	}
}
