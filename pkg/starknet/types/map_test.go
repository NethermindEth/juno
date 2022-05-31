package types

import (
	"github.com/NethermindEth/juno/internal/db"
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func newDict(database db.Databaser, prefix string) *Dictionary {
	return NewDictionary(database, prefix)
}

func TestMapUsingTransactionHash(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)

	// create a new Dictionary
	dict := newDict(database, "test")

	// Set a new value, in this case TransactionHash
	txnHash := TransactionHash{Hash: common.HexToHash("0x01")}

	dict.Add("hash", txnHash)

	// Check if key exist in the database
	exist := dict.Exist("hash")
	if !exist {
		t.Fail()
	}

	fetchedVal := TransactionHash{}
	get, err := dict.Get("hash", fetchedVal)
	if err != nil {
		return
	}
	// check that retrieved value match the one that was saved
	if get.(TransactionHash).Hash.Hex() != txnHash.Hash.Hex() {
		t.Fail()
	}
	// Check if key exist in the database
	removed := dict.Remove("hash")
	if !removed {
		t.Fail()
	}
	// Check if key exist in the database
	exist = dict.Exist("hash")
	if exist {
		t.Fail()
	}

}
func TestMapUsingPagesHash(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)

	// create a new Dictionary
	dict := newDict(database, "test")

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

	fetchedVal := PagesHash{}
	get, err := dict.Get("pages", fetchedVal)
	if err != nil {
		t.Fail()
		return
	}
	// check that retrieved value match the one that was saved
	if string(get.(PagesHash).Bytes[0][:]) != string(pagesHash.Bytes[0][:]) {
		t.Fail()
	}
	// Check if key exist in the database
	removed := dict.Remove("pages")
	if !removed {
		t.Fail()
	}
	// Check if key exist in the database
	exist = dict.Exist("pages")
	if exist {
		t.Fail()
	}
}
func TestMapUsingFact(t *testing.T) {
	database := db.NewKeyValueDb(t.TempDir(), 0)

	// create a new Dictionary
	dict := newDict(database, "test")

	// Set a new value, in this case TransactionHash
	fact := Fact{
		StateRoot:   "stateRoot",
		SequenceNumber: "BlockNumber",
		Value:       "Value",
	}

	dict.Add("fact", fact)

	// Check if key exist in the database
	exist := dict.Exist("fact")
	if !exist {
		t.Fail()
	}

	fetchedVal := Fact{}
	get, err := dict.Get("fact", fetchedVal)
	if err != nil {
		return
	}
	// check that retrieved value match the one that was saved
	if get.(Fact).StateRoot != fact.StateRoot || get.(Fact).SequenceNumber != fact.SequenceNumber ||
		get.(Fact).Value != fact.Value {
		t.Fail()
	}
	// Check if key exist in the database
	removed := dict.Remove("fact")
	if !removed {
		t.Fail()
	}
	// Check if key exist in the database
	exist = dict.Exist("fact")
	if exist {
		t.Fail()
	}

}
