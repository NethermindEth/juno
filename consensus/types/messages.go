package types

// Todo: Signature over the messages needs to be handled somewhere. There are 2 options:
//	1. Add the signature to each message and extend the Validator Set interface to include VerifyMessageSignature
//	method.
//	2. The P2P layer signs the message before gossiping to other validators and verifies the signature before passing
//	the message to the consensus engine.
//  The benefit of P2P layer handling the verification of the signature is the that the consensus layer can assume
//  the message is from a validator in the validator set. However, this means that the P2P layer would need to be aware
//  of the validator set and would need access to the blockchain which may not be a good idea.

type Message[V Hashable[H], H Hash, A Addr] interface {
	Header() *MessageHeader[A]
}

type Proposal[V Hashable[H], H Hash, A Addr] struct {
	MessageHeader[A]
	ValidRound Round `cbor:"valid_round"`
	Value      *V    `cbor:"value"`
}

func (p *Proposal[V, H, A]) Header() *MessageHeader[A] {
	return &p.MessageHeader
}

type (
	Prevote[H Hash, A Addr]   Vote[H, A]
	Precommit[H Hash, A Addr] Vote[H, A]
)

func (p *Prevote[H, A]) Header() *MessageHeader[A] {
	return &p.MessageHeader
}

func (p *Precommit[H, A]) Header() *MessageHeader[A] {
	return &p.MessageHeader
}

type Vote[H Hash, A Addr] struct {
	MessageHeader[A]
	ID *H `cbor:"id"`
}

type MessageHeader[A Addr] struct {
	Height Height `cbor:"height"`
	Round  Round  `cbor:"round"`
	Sender A      `cbor:"sender"`
}
