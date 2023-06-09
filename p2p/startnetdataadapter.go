package p2p

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/starknetdata"
	"github.com/NethermindEth/juno/utils"
	"github.com/nsf/jsondiff"
)

type StartnetDataAdapter struct {
	base    starknetdata.StarknetData
	p2p     P2P
	network utils.Network
}

func NewStarknetDataAdapter(base starknetdata.StarknetData, p2p P2P, network utils.Network) starknetdata.StarknetData {
	return &StartnetDataAdapter{
		base:    base,
		p2p:     p2p,
		network: network,
	}
}

func (s *StartnetDataAdapter) BlockByNumber(ctx context.Context, blockNumber uint64) (block *core.Block, err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
			err = errors.New(fmt.Sprintf("%s", r))
		}
	}()

	gatewayBlock, err := s.base.BlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, err
	}

	protoheader, err := coreBlockToProtobufHeader(gatewayBlock)
	if err != nil {
		return nil, err
	}

	protoBody := coreBlockToProtobufBody(gatewayBlock)

	newCoreBlock, err := protobufHeaderAndBodyToCoreBlock(protoheader, protoBody, s.network)
	if err != nil {
		return nil, err
	}

	gatewayjson, err := json.MarshalIndent(gatewayBlock, "", "    ")
	if err != nil {
		return nil, err
	}

	reencodedblockjson, err := json.MarshalIndent(newCoreBlock, "", "    ")
	if err != nil {
		return nil, err
	}

	if string(gatewayjson) != string(reencodedblockjson) {
		for i, receipt := range gatewayBlock.Receipts {
			tx := gatewayBlock.Transactions[i]

			tx2 := newCoreBlock.Transactions[i]
			receipt2 := newCoreBlock.Receipts[i]

			if !compareAndPrintDiff(tx, tx2) {
				thegrpctx := protoBody.Transactions[i]
				felttx := fieldElementToFelt(thegrpctx.GetL1Handler().GetHash())
				return nil, fmt.Errorf("mismatch. %s %s", thegrpctx, felttx)
			}

			if !compareAndPrintDiff(receipt, receipt2) {
				return nil, errors.New("Mismatch")
			}

		}

		return nil, errors.New("Mismatch")
	}

	return gatewayBlock, err
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

func (s *StartnetDataAdapter) BlockLatest(ctx context.Context) (*core.Block, error) {
	return s.base.BlockLatest(ctx)
}

func (s *StartnetDataAdapter) Transaction(ctx context.Context, transactionHash *felt.Felt) (core.Transaction, error) {
	return s.base.Transaction(ctx, transactionHash)
}

func (s *StartnetDataAdapter) Class(ctx context.Context, classHash *felt.Felt) (core.Class, error) {
	return s.base.Class(ctx, classHash)
}

func (s *StartnetDataAdapter) StateUpdate(ctx context.Context, blockNumber uint64) (*core.StateUpdate, error) {
	return s.base.StateUpdate(ctx, blockNumber)
}

// Typecheck
var _ starknetdata.StarknetData = &StartnetDataAdapter{}
