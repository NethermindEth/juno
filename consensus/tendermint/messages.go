package tendermint

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// Todo: Signature over the messages needs to be handled somewhere. There are 2 options:
//	1. Add the signature to each message and extend the Validator Set interface to include VerifyMessageSignature
//	method.
//	2. The P2P layer signs the message before gossiping to other validators and verifies the signature before passing
//	the message to the consensus engine.
//  The benefit of P2P layer handling the verification of the signature is the that the consensus layer can assume
//  the message is from a validator in the validator set. However, this means that the P2P layer would need to be aware
//  of the validator set and would need access to the blockchain which may not be a good idea.

type Message[V Hashable[H], H Hash, A Addr] interface {
	Proposal[V, H, A] | Prevote[H, A] | Precommit[H, A]
}

type Proposal[V Hashable[H], H Hash, A Addr] struct {
	MessageHeader[A]
	ValidRound round `cbor:"valid_round"`
	Value      *V    `cbor:"value"`
}

// Todo: fix
// func (p *Proposal[V, H, A]) MarshalCBOR() ([]byte, error) {
// 	var senderBytes []byte

// 	switch s := any(p.Sender).(type) {
// 	case [20]byte:
// 		senderBytes = s[:]
// 	case [32]byte:
// 		senderBytes = s[:]
// 	case felt.Felt:
// 		tmp := s.Bytes()
// 		senderBytes = tmp[:]
// 	default:
// 		return nil, fmt.Errorf("MarshalCBOR: unsupported Sender type %T", s)
// 	}

// 	return cbor.Marshal([]interface{}{
// 		p.Height,
// 		p.Round,
// 		senderBytes,
// 		p.ValidRound,
// 		*p.Value,
// 	})
// }

// // Todo: fix
// func (p *Proposal[V, H, A]) UnmarshalCBOR(data []byte) error {
// 	var arr []interface{}
// 	if err := cbor.Unmarshal(data, &arr); err != nil {
// 		return err
// 	}
// 	if len(arr) != 5 {
// 		return fmt.Errorf("expected 5 fields, got %d", len(arr))
// 	}

// 	// Height
// 	if h, ok := arr[0].(uint64); ok {
// 		p.Height = height(h)
// 	} else {
// 		return fmt.Errorf("invalid height")
// 	}

// 	// Round
// 	if r, ok := arr[1].(uint64); ok {
// 		p.Round = round(r)
// 	} else {
// 		return fmt.Errorf("invalid round")
// 	}

// 	// Sender
// 	switch s := arr[2].(type) {
// 	case A:
// 		p.Sender = s
// 	case []byte:
// 		var sender A
// 		switch any(&sender).(type) {
// 		case felt.Felt:
// 			p.Sender = any(new(felt.Felt).SetBytes(s)).(A)
// 		case *[20]byte:
// 			if len(s) != 20 {
// 				return fmt.Errorf("invalid sender length")
// 			}
// 			var a [20]byte
// 			copy(a[:], s)
// 			p.Sender = any(a).(A)
// 		default:
// 			return fmt.Errorf("unsupported Addr backing type")
// 		}
// 	default:
// 		return fmt.Errorf("invalid sender type: %T", arr[2])
// 	}

// 	// ValidRound
// 	if vr, ok := arr[3].(uint64); ok {
// 		p.ValidRound = round(vr)
// 	} else {
// 		return fmt.Errorf("invalid validRound")
// 	}

// 	// Decode V from interface{}
// 	tmpV := new(V)
// 	b, err := cbor.Marshal(arr[4])
// 	if err != nil {
// 		return fmt.Errorf("re-encode for value failed: %w", err)
// 	}
// 	if err := cbor.Unmarshal(b, tmpV); err != nil {
// 		return fmt.Errorf("value decode failed: %w", err)
// 	}
// 	p.Value = tmpV

// 	return nil
// }

type (
	Prevote[H Hash, A Addr]   Vote[H, A]
	Precommit[H Hash, A Addr] Vote[H, A]
)

type Vote[H Hash, A Addr] struct {
	MessageHeader[A] `cbor:"message_header"`
	ID               *H `cbor:"id"`
}

type MessageHeader[A Addr] struct {
	Height height `cbor:"height"`
	Round  round  `cbor:"round"`
	Sender A      `cbor:"sender"`
}

// Todo: fix
func (v *Vote[H, A]) MarshalCBOR() ([]byte, error) { // Todo (rian): implement tests
	return cbor.Marshal([]interface{}{
		v.Height,
		v.Round,
		v.Sender,
		*v.ID,
	})
}

// Todo: fix
func (v *Vote[H, A]) UnmarshalCBOR(data []byte) error {
	var arr []interface{}
	if err := cbor.Unmarshal(data, &arr); err != nil {
		return err
	}
	if len(arr) != 4 {
		return fmt.Errorf("expected 4 fields, got %d", len(arr))
	}

	var ok bool

	if v.Height, ok = arr[0].(height); !ok {
		return fmt.Errorf("invalid height")
	}
	if v.Round, ok = arr[1].(round); !ok {
		return fmt.Errorf("invalid round")
	}
	if v.Sender, ok = arr[2].(A); !ok {
		return fmt.Errorf("invalid sender")
	}

	// decode H value → assign to *v.ID
	tmp := new(H)
	b, err := cbor.Marshal(arr[3])
	if err != nil {
		return fmt.Errorf("re-marshal id: %w", err)
	}
	if err := cbor.Unmarshal(b, tmp); err != nil {
		return fmt.Errorf("unmarshal id: %w", err)
	}
	v.ID = tmp

	return nil
}

// messages keep tracks of all the proposals, prevotes, precommits by creating a map structure as follows:
// height->round->address->[]Message

// Todo: would the following representation of message be better:
//
//	  height -> round -> address -> ID -> Message
//	How would we keep track of nil votes? In golan map key cannot be nil.
//	It is not easy to calculate a zero value when dealing with generics.
type messages[V Hashable[H], H Hash, A Addr] struct {
	proposals  map[height]map[round]map[A]Proposal[V, H, A]
	prevotes   map[height]map[round]map[A]Prevote[H, A]
	precommits map[height]map[round]map[A]Precommit[H, A]
}

func newMessages[V Hashable[H], H Hash, A Addr]() messages[V, H, A] {
	return messages[V, H, A]{
		proposals:  make(map[height]map[round]map[A]Proposal[V, H, A]),
		prevotes:   make(map[height]map[round]map[A]Prevote[H, A]),
		precommits: make(map[height]map[round]map[A]Precommit[H, A]),
	}
}

func addMessages[T any, A Addr](storage map[height]map[round]map[A]T, msg T, a A, h height, r round) {
	if _, ok := storage[h]; !ok {
		storage[h] = make(map[round]map[A]T)
	}

	if _, ok := storage[h][r]; !ok {
		storage[h][r] = make(map[A]T)
	}

	if _, ok := storage[h][r][a]; !ok {
		storage[h][r][a] = msg
	}
}

// Todo: ensure duplicated messages are ignored.
func (m *messages[V, H, A]) addProposal(p Proposal[V, H, A]) {
	addMessages(m.proposals, p, p.Sender, p.Height, p.Round)
}

func (m *messages[V, H, A]) addPrevote(p Prevote[H, A]) {
	addMessages(m.prevotes, p, p.Sender, p.Height, p.Round)
}

func (m *messages[V, H, A]) addPrecommit(p Precommit[H, A]) {
	addMessages(m.precommits, p, p.Sender, p.Height, p.Round)
}

func (m *messages[V, H, A]) allMessages(h height, r round) (map[A]Proposal[V, H, A], map[A]Prevote[H, A],
	map[A]Precommit[H, A],
) {
	// Todo: Should they be copied?
	return m.proposals[h][r], m.prevotes[h][r], m.precommits[h][r]
}

func (m *messages[V, H, A]) deleteHeightMessages(h height) {
	delete(m.proposals, h)
	delete(m.prevotes, h)
	delete(m.precommits, h)
}
