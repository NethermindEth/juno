package starknet

import (
	"crypto/rand"
	"slices"

	"github.com/NethermindEth/juno/adapters/core2p2p"
	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/p2p/starknet/spec"
	"github.com/NethermindEth/juno/utils"
	"google.golang.org/protobuf/proto"
)

type blockBodyStep int

const (
	_ blockBodyStep = iota

	sendDiff // initial
	sendClasses
	sendProof
	sendBlockFin
	terminal // final state
)

type blockBodyIterator struct {
	log         utils.Logger
	stateReader core.StateReader
	stateCloser func() error

	state       blockBodyStep
	header      *core.Header
	stateUpdate *core.StateUpdate
}

func newBlockBodyIterator(bcReader blockchain.Reader, header *core.Header, log utils.Logger) (*blockBodyIterator, error) {
	stateUpdate, err := bcReader.StateUpdateByNumber(header.Number)
	if err != nil {
		return nil, err
	}

	stateReader, closer, err := bcReader.StateAtBlockNumber(header.Number)
	if err != nil {
		return nil, err
	}

	return &blockBodyIterator{
		state:       sendDiff,
		header:      header,
		log:         log,
		stateReader: stateReader,
		stateCloser: closer,
		stateUpdate: stateUpdate, // to be filled during one of next states
	}, nil
}

func (b *blockBodyIterator) hasNext() bool {
	return slices.Contains([]blockBodyStep{
		sendDiff,
		sendClasses,
		sendProof,
		sendBlockFin,
	}, b.state)
}

// Either BlockBodiesResponse_Diff, *_Classes, *_Proof, *_Fin
func (b *blockBodyIterator) next() (msg proto.Message, valid bool) {
	switch b.state {
	case sendDiff:
		msg, valid = b.diff()
		b.state = sendClasses
	case sendClasses:
		msg, valid = b.classes()
		b.state = sendProof
	case sendProof:
		msg, valid = b.proof()
		b.state = sendBlockFin
	case sendBlockFin:
		// fin changes state to terminal internally
		msg, valid = b.fin()
	case terminal:
		panic("next called on terminal state")
	default:
		b.log.Errorw("Unknown state in blockBodyIterator", "state", b.state)
	}

	return
}

func (b *blockBodyIterator) classes() (proto.Message, bool) {
	var classes []*spec.Class

	stateDiff := b.stateUpdate.StateDiff

	for _, hash := range stateDiff.DeclaredV0Classes {
		cls, err := b.stateReader.Class(hash)
		if err != nil {
			return b.fin()
		}

		classes = append(classes, core2p2p.AdaptClass(cls.Class, hash))
	}
	for _, class := range stateDiff.DeclaredV1Classes {
		cls, err := b.stateReader.Class(class.ClassHash)
		if err != nil {
			return b.fin()
		}

		cairo1Cls := cls.Class.(*core.Cairo1Class)
		classes = append(classes, core2p2p.AdaptClass(cls.Class, cairo1Cls.Hash()))
	}
	for _, class := range stateDiff.DeployedContracts {
		cls, err := b.stateReader.Class(class.ClassHash)
		if err != nil {
			return b.fin()
		}

		var compiledHash *felt.Felt
		switch cairoClass := cls.Class.(type) {
		case *core.Cairo0Class:
			compiledHash = class.ClassHash
		case *core.Cairo1Class:
			compiledHash = cairoClass.Hash()
		default:
			b.log.Errorw("Unknown cairo class", "cairoClass", cairoClass)
			return b.fin()
		}

		classes = append(classes, core2p2p.AdaptClass(cls.Class, compiledHash))
	}

	return &spec.BlockBodiesResponse{
		Id: core2p2p.AdaptBlockID(b.header),
		BodyMessage: &spec.BlockBodiesResponse_Classes{
			Classes: &spec.Classes{
				Domain:  0,
				Classes: classes,
			},
		},
	}, true
}

type contractDiff struct {
	address      *felt.Felt
	classHash    *felt.Felt
	storageDiffs []core.StorageDiff
	nonce        *felt.Felt
}

func (b *blockBodyIterator) diff() (proto.Message, bool) {
	var err error
	diff := b.stateUpdate.StateDiff

	modifiedContracts := make(map[felt.Felt]*contractDiff)
	initContractDiff := func(addr *felt.Felt) (*contractDiff, error) {
		var cHash *felt.Felt
		cHash, err = b.stateReader.ContractClassHash(addr)
		if err != nil {
			return nil, err
		}
		return &contractDiff{address: addr, classHash: cHash}, nil
	}

	for addr, n := range diff.Nonces {
		cDiff, ok := modifiedContracts[addr]
		if !ok {
			cDiff, err = initContractDiff(&addr)
			if err != nil {
				b.log.Errorw("Failed to get class hash", "err", err)
				return b.fin()
			}
			modifiedContracts[addr] = cDiff
		}
		cDiff.nonce = n
	}

	for addr, sDiff := range diff.StorageDiffs {
		cDiff, ok := modifiedContracts[addr]
		if !ok {
			cDiff, err = initContractDiff(&addr)
			if err != nil {
				b.log.Errorw("Failed to get class hash", "err", err)
				return b.fin()
			}
			modifiedContracts[addr] = cDiff
		}
		cDiff.storageDiffs = sDiff
	}

	var contractDiffs []*spec.StateDiff_ContractDiff
	for _, c := range modifiedContracts {
		contractDiffs = append(contractDiffs, core2p2p.AdaptStateDiff(c.address, c.classHash, c.nonce, c.storageDiffs))
	}

	return &spec.BlockBodiesResponse{
		Id: core2p2p.AdaptBlockID(b.header),
		BodyMessage: &spec.BlockBodiesResponse_Diff{
			Diff: &spec.StateDiff{
				Domain:        0,
				ContractDiffs: contractDiffs,
			},
		},
	}, true
}

func (b *blockBodyIterator) fin() (proto.Message, bool) {
	b.state = terminal
	if err := b.stateCloser(); err != nil {
		b.log.Errorw("Call to state closer failed", "err", err)
	}
	return &spec.BlockBodiesResponse{
		Id:          core2p2p.AdaptBlockID(b.header),
		BodyMessage: &spec.BlockBodiesResponse_Fin{},
	}, true
}

func (b *blockBodyIterator) proof() (proto.Message, bool) {
	// proof size is currently 142K
	proof := make([]byte, 142*1024) //nolint:gomnd
	_, err := rand.Read(proof)
	if err != nil {
		b.log.Errorw("Failed to generate rand proof", "err", err)
		return b.fin()
	}

	return &spec.BlockBodiesResponse{
		Id: core2p2p.AdaptBlockID(b.header),
		BodyMessage: &spec.BlockBodiesResponse_Proof{
			Proof: &spec.BlockProof{
				Proof: proof,
			},
		},
	}, true
}
