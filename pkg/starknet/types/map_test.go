package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func newDict() *Dictionary {
	return NewDictionary()
}

func TestMapUsingTransactionHash(t *testing.T) {
	// create a new Dictionary
	dict := newDict()

	// Set a new value, in this case TransactionHash
	txnHash := TransactionHash{Hash: common.HexToHash("0x01")}

	dict.Add("hash", txnHash)

	// Check if key exist in the database
	exist := dict.Exist("hash")
	if !exist {
		t.Fail()
	}

	get, ok := dict.Get("hash")
	if !ok {
		t.Fail()
		return
	}
	// check that retrieved value match the one that was saved
	if get.(TransactionHash).Hash.Hex() != txnHash.Hash.Hex() {
		t.Fail()
	}
	// Check if key exist in the database
	dict.Remove("hash")

	// Check if key exist in the database
	exist = dict.Exist("hash")
	if exist {
		t.Fail()
	}
}

func TestMapUsingPagesHash(t *testing.T) {
	// create a new Dictionary
	dict := newDict()

	// Set a new value, in this case TransactionHash
	pagesHash := PagesHash{Bytes: make([][32]byte, 0)}
	var value [32]byte
	copy(value[:], "value")
	pagesHash.Bytes = append(pagesHash.Bytes, value)

	dict.Add("pages", pagesHash)

	// Check if key exist in the database
	exist := dict.Exist("pages")
	if !exist {
		t.Fail()
	}

	get, ok := dict.Get("pages")
	if !ok {
		t.Fail()
		return
	}
	// check that retrieved value match the one that was saved
	if string(get.(PagesHash).Bytes[0][:]) != string(pagesHash.Bytes[0][:]) {
		t.Fail()
	}
	// Check if key exist in the database
	dict.Remove("pages")
	// Check if key exist in the database
	exist = dict.Exist("pages")
	if exist {
		t.Fail()
	}
}

func TestMapUsingFact(t *testing.T) {
	// create a new Dictionary
	dict := newDict()

	// Set a new value, in this case TransactionHash
	fact := Fact{
		StateRoot:      "stateRoot",
		SequenceNumber: 0,
		Value:          "Value",
	}

	dict.Add("fact", fact)

	// Check if key exist in the database
	exist := dict.Exist("fact")
	if !exist {
		t.Fail()
	}

	get, ok := dict.Get("fact")
	if !ok {
		t.Fail()
		return
	}
	// check that retrieved value match the one that was saved
	if get.(Fact).StateRoot != fact.StateRoot || get.(Fact).SequenceNumber != fact.SequenceNumber ||
		get.(Fact).Value != fact.Value {
		t.Fail()
	}
	// Check if key exist in the database
	dict.Remove("fact")
	// Check if key exist in the database
	exist = dict.Exist("fact")
	if exist {
		t.Fail()
	}
}
