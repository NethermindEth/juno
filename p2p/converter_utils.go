package p2p

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/encoder"
	"github.com/nsf/jsondiff"
	"github.com/pkg/errors"
	"os"
	"sync"
)

func runBlockEncodingTests(blockchain *blockchain.Blockchain) error {
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

	head, err := blockchain.Head()
	if err != nil {
		return errors.Wrap(err, "error fetching head")
	}

	startblock := 4800
	for i := startblock; i < int(head.Number); i++ {
		blocknumchan <- i
	}
	close(blocknumchan)
	wg.Wait()
	return nil
}

func testBlockEncoding(originalBlock *core.Block, blockchain *blockchain.Blockchain) error {
	c := NewConverter(&blockchainClassProvider{
		blockchain: blockchain,
	})
	v := &verifier{network: blockchain.Network()}
	originalBlock.ProtocolVersion = ""

	protoheader, err := c.coreBlockToProtobufHeader(originalBlock)
	if err != nil {
		return err
	}

	protoBody, err := c.coreBlockToProtobufBody(originalBlock)
	if err != nil {
		return err
	}

	newCoreBlock, classes, err := c.protobufHeaderAndBodyToCoreBlock(protoheader, protoBody)
	if err != nil {
		return err
	}

	err = v.VerifyBlock(newCoreBlock)
	if err != nil {
		return errors.Wrap(err, "error verifying blocks")
	}

	for key, class := range classes {
		err = v.VerifyClass(class, &key)
		if err != nil {
			return errors.Wrap(err, "error verifying class")
		}
	}

	newCoreBlock.ProtocolVersion = ""

	gatewayjson, err := json.MarshalIndent(originalBlock, "", "    ")
	if err != nil {
		return err
	}

	reencodedblockjson, err := json.MarshalIndent(newCoreBlock, "", "    ")
	if err != nil {
		return err
	}

	if string(gatewayjson) != string(reencodedblockjson) {

		updateBytes, err := encoder.Marshal(originalBlock)
		if err != nil {
			return err
		}
		err = os.WriteFile(fmt.Sprintf("p2p/converter_tests/blocks/%d.dat", originalBlock.Number), updateBytes, 0666)
		if err != nil {
			return err
		}

		for i, receipt := range originalBlock.Receipts {
			tx := originalBlock.Transactions[i]

			tx2 := newCoreBlock.Transactions[i]
			receipt2 := newCoreBlock.Receipts[i]

			if !compareAndPrintDiff(tx, tx2) {
				return errors.New("tx mismatch.")
			}

			if !compareAndPrintDiff(receipt, receipt2) {
				return errors.New("receipt mismatch")
			}
		}

		txCommit, err := originalBlock.CalculateTransactionCommitment()
		if err != nil {
			return err
		}

		eCommit, err := originalBlock.CalculateEventCommitment()
		if err != nil {
			return err
		}

		headeragain, _ := c.coreBlockToProtobufHeader(originalBlock)
		txCommit2 := fieldElementToFelt(headeragain.TransactionCommitment)
		eCommit2 := fieldElementToFelt(headeragain.EventCommitment)
		if !txCommit.Equal(txCommit2) {
			return errors.New("Tx commit not match")
		}
		if !eCommit.Equal(eCommit2) {
			return errors.New("Event commit not match")
		}

		err = core.VerifyBlockHash(originalBlock, blockchain.Network())
		if err != nil {
			return err
		}

		compareAndPrintDiff(originalBlock, newCoreBlock)
		return errors.New("Mismatch")
	}

	return nil
}

func testStateDiff(stateDiff *core.StateUpdate) error {
	oriBlockHash := stateDiff.BlockHash
	stateDiff.BlockHash = nil
	stateDiff.NewRoot = nil
	stateDiff.OldRoot = nil

	protobuff := coreStateUpdateToProtobufStateUpdate(stateDiff)

	reencodedStateDiff := protobufStateUpdateToCoreStateUpdate(protobuff)

	before, err := encoder.Marshal(stateDiff)
	if err != nil {
		panic(err)
	}
	after, err := encoder.Marshal(reencodedStateDiff)
	if err != nil {
		panic(err)
	}

	if bytes.Equal(before, after) {
		return nil
	}

	updateBytes, err := encoder.Marshal(stateDiff)
	if err != nil {
		return err
	}
	err = os.WriteFile(fmt.Sprintf("p2p/converter_tests/state_updates/%s.dat", oriBlockHash.String()), updateBytes, 0666)
	if err != nil {
		return err
	}

	oriSD := stateDiff.StateDiff
	rSD := reencodedStateDiff.StateDiff

	for key, diffs := range oriSD.StorageDiffs {
		odiff, ok := rSD.StorageDiffs[key]
		if !ok {
			return fmt.Errorf("missing entry %s", key.String())
		}

		if !compareAndPrintDiff(diffs, odiff) {
			return errors.New("Wrong diff")
		}
	}

	if !compareAndPrintDiff(stateDiff.StateDiff.DeclaredV0Classes, reencodedStateDiff.StateDiff.DeclaredV0Classes) {
		return errors.New("Unable to compare")
	}

	if !compareAndPrintDiff(stateDiff.StateDiff.DeclaredV1Classes, reencodedStateDiff.StateDiff.DeclaredV1Classes) {
		return errors.New("Unable to compare")
	}

	if !compareAndPrintDiff(stateDiff.StateDiff.ReplacedClasses, reencodedStateDiff.StateDiff.ReplacedClasses) {
		return errors.New("Unable to compare")
	}

	return errors.New("mismatch")
}

func compareAndPrintDiff(item1 interface{}, item2 interface{}) bool {
	item1json, _ := json.MarshalIndent(item1, "", "    ")
	item2json, _ := json.MarshalIndent(item2, "", "    ")

	opt := jsondiff.DefaultConsoleOptions()
	diff, strdiff := jsondiff.Compare(item1json, item2json, &opt)

	if diff == jsondiff.FullMatch {
		return true
	}

	fmt.Printf("Mismatch\n")
	fmt.Println(strdiff)

	return false
}
