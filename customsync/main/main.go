package main

import (
	"fmt"

	sync "github.com/NethermindEth/juno/customsync"
)

func main() {
	cs := sync.NewCustomSynchronizer()
	cs.Run()
	fmt.Println(cs)
}
