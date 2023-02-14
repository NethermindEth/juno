package sync

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/clients"
	"github.com/NethermindEth/juno/utils"
)

type ErrL1VerificationFailed struct {
	Height uint64
	Err    error
}

func (e ErrL1VerificationFailed) Error() string {
	return fmt.Sprintf("L1 verification failed on block #%d with %s", e.Height, e.Err.Error())
}

// L1Verifier is a background worker that continously tries to detect the highest height that was verified
// on L1.
type L1Verifier struct {
	Blockchain *blockchain.Blockchain
	Gateway    *clients.GatewayClient
	Wait       time.Time
	log        utils.SimpleLogger
}

// NewL1Verifier creates a new L1Verifier
func NewL1Verifier(bc *blockchain.Blockchain, gateway *clients.GatewayClient, log utils.SimpleLogger) *L1Verifier {
	return &L1Verifier{
		Blockchain: bc,
		Gateway:    gateway,
		log:        log,
	}
}

// Run starts the L1Verifier.
func (v *L1Verifier) Run(ctx context.Context) error {
	errChan := make(chan ErrL1VerificationFailed)

	nextVerifiedHeight := uint64(0)
	if h, err := v.Blockchain.L1ChainHeight(); err == nil {
		nextVerifiedHeight = h + 1
	}
	isSequential := false

	for {
		select {
		case err := <-errChan:
			nextVerifiedHeight = err.Height
			v.log.Warnw("Retrying L1 verification", "height", err.Height, "err", err.Err)
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			} else {
				return ctx.Err()
			}
		default:
			curHeight := nextVerifiedHeight
			// When the block is not yet verified on L1, wait for 1 hour before trying again.
			if time.Now().After((v.Wait).Add(time.Hour)) {
				var l2Height uint64
				if h, err := v.Blockchain.Height(); err == nil {
					l2Height = h
				}

				if l2Height >= nextVerifiedHeight {
					nextVerifiedHeight = v.verifier(ctx, curHeight, l2Height, &isSequential, errChan)
				}
			}
		}
	}
}

func (v *L1Verifier) verifier(ctx context.Context, nextVerifiedHeight, l2Height uint64, isSequential *bool, errChan chan ErrL1VerificationFailed) uint64 {
	for nextVerifiedHeight <= l2Height {
		select {
		case <-ctx.Done():
			return nextVerifiedHeight
		default:
			if *isSequential {
				if v.isVerified(ctx, nextVerifiedHeight, errChan) {
					return nextVerifiedHeight + 1
				} else {
					v.Wait = time.Now()
					return nextVerifiedHeight
				}
			}

			blockNum := nextVerifiedHeight + (l2Height-(nextVerifiedHeight-1))/2
			if v.isVerified(ctx, l2Height, errChan) {
				return l2Height + 1
			} else if v.isVerified(ctx, blockNum, errChan) {
				nextVerifiedHeight = blockNum + 1
			} else {
				if v.isVerified(ctx, blockNum-1, errChan) {
					v.Wait = time.Now()
					*isSequential = true
					return blockNum
				} else {
					l2Height = blockNum
				}
			}
		}
	}
	return nextVerifiedHeight
}

func (v *L1Verifier) isVerified(ctx context.Context, nextVerifiedHeight uint64, errChan chan ErrL1VerificationFailed) bool {
	// Call the gateway to get block.
	block, err := v.Gateway.GetBlock(ctx, nextVerifiedHeight)
	if err != nil {
		select {
		case <-ctx.Done():
		case errChan <- ErrL1VerificationFailed{nextVerifiedHeight, err}:
		}
		return false
	}
	if block.Status == "ACCEPTED_ON_L1" {
		if err := v.Blockchain.StoreL1ChainHeight(nextVerifiedHeight); err != nil {
			select {
			case <-ctx.Done():
			case errChan <- ErrL1VerificationFailed{nextVerifiedHeight, err}:
			}
			return false
		} else {
			v.log.Infow("Block verified on L1", "height", nextVerifiedHeight)
			return true
		}
	} else {
		return false
	}
}
