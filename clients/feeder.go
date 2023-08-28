package client

import (
	"context"
	"encoding/json"
	"io"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/utils"
)

type FeederInterface interface {
	StateUpdate(ctx context.Context, blockID string) (*utils.StateUpdateSeq, error)
	Transaction(ctx context.Context, transactionHash *felt.Felt) (*utils.TransactionStatus, error)
	Block(ctx context.Context, blockID string) (*utils.Block, error)
	ClassDefinition(ctx context.Context, classHash *felt.Felt) (*utils.ClassDefinition, error)
	CompiledClassDefinition(ctx context.Context, classHash *felt.Felt) (json.RawMessage, error)
	PublickKey(ctx context.Context) (*felt.Felt, error)
	Signature(ctx context.Context, blockID string) (*utils.Signature, error)
	StateUpdateWithBlock(ctx context.Context, blockID string) (*utils.StateUpdateWithBlock, error)
}

type Feeder struct {
	client *Client
}

var _ FeederInterface = &Feeder{}

func NewFeeder(client *Client) *Feeder {
	return &Feeder{client: client}
}

func (f *Feeder) StateUpdate(ctx context.Context, blockID string) (*utils.StateUpdateSeq, error) {
	queryURL := f.client.buildQueryString("get_state_update", map[string]string{
		"blockNumber": blockID,
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	update := new(utils.StateUpdateSeq)
	if err = json.NewDecoder(body).Decode(update); err != nil {
		return nil, err
	}
	return update, nil
}

func (f *Feeder) Transaction(ctx context.Context, transactionHash *felt.Felt) (*utils.TransactionStatus, error) {
	queryURL := f.client.buildQueryString("get_transaction", map[string]string{
		"transactionHash": transactionHash.String(),
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	txStatus := new(utils.TransactionStatus)
	if err = json.NewDecoder(body).Decode(txStatus); err != nil {
		return nil, err
	}
	return txStatus, nil
}

func (f *Feeder) Block(ctx context.Context, blockID string) (*utils.Block, error) {
	queryURL := f.client.buildQueryString("get_block", map[string]string{
		"blockNumber": blockID,
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	block := new(utils.Block)
	if err = json.NewDecoder(body).Decode(block); err != nil {
		return nil, err
	}
	return block, nil
}

func (f *Feeder) ClassDefinition(ctx context.Context, classHash *felt.Felt) (*utils.ClassDefinition, error) {
	queryURL := f.client.buildQueryString("get_class_by_hash", map[string]string{
		"classHash": classHash.String(),
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	class := new(utils.ClassDefinition)
	if err = json.NewDecoder(body).Decode(class); err != nil {
		return nil, err
	}
	return class, nil
}

func (f *Feeder) CompiledClassDefinition(ctx context.Context, classHash *felt.Felt) (json.RawMessage, error) {
	queryURL := f.client.buildQueryString("get_compiled_class_by_class_hash", map[string]string{
		"classHash": classHash.String(),
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var class json.RawMessage
	if err = json.NewDecoder(body).Decode(&class); err != nil {
		return nil, err
	}
	return class, nil
}

func (f *Feeder) PublickKey(ctx context.Context) (*felt.Felt, error) {
	queryURL := f.client.buildQueryString("get_public_key", nil)

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	b, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	publicKey := new(felt.Felt).SetBytes(b)

	return publicKey, nil
}

func (f *Feeder) Signature(ctx context.Context, blockID string) (*utils.Signature, error) {
	queryURL := f.client.buildQueryString("get_signature", map[string]string{
		"blockNumber": blockID,
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	signature := new(utils.Signature)
	if err := json.NewDecoder(body).Decode(signature); err != nil {
		return nil, err
	}

	return signature, nil
}

func (f *Feeder) StateUpdateWithBlock(ctx context.Context, blockID string) (*utils.StateUpdateWithBlock, error) {
	queryURL := f.client.buildQueryString("get_state_update", map[string]string{
		"blockNumber":  blockID,
		"includeBlock": "true",
	})

	body, err := f.client.get(ctx, queryURL)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	stateUpdate := new(utils.StateUpdateWithBlock)
	if err := json.NewDecoder(body).Decode(stateUpdate); err != nil {
		return nil, err
	}

	return stateUpdate, nil
}
