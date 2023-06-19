package p2p

import (
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"sync"
)

func runBlockEncodingTests(blockchain *blockchain.Blockchain, err error) error {
	head, err := blockchain.Head()
	if err != nil {
		return err
	}

	blocknumchan := make(chan int)

	threadcount := 32
	wg := sync.WaitGroup{}
	wg.Add(threadcount)
	for i := 0; i < threadcount; i++ {
		go func() {
			defer wg.Done()
			for i := range blocknumchan {
				fmt.Printf("Running on block %d\n", i)

				block, err := blockchain.BlockByNumber(uint64(i))
				if err != nil {
					panic(err)
				}

				err = testBlockEncoding(block, blockchain)
				if err != nil {
					panic(err)
				}

				update, err := blockchain.StateUpdateByNumber(uint64(i))
				if err != nil {
					panic(err)
				}

				err = testStateDiff(update)
				if err != nil {
					panic(err)
				}
			}
		}()
	}

	startblock := 4800
	for i := startblock; i < int(head.Number); i++ {
		blocknumchan <- i
	}
	close(blocknumchan)
	wg.Wait()
	return nil
}
