package db

import (
	"bytes"
	"testing"
)

func TestBlockSpecificDatabase_Put(t *testing.T) {
	tests := [...]struct {
		Key         []byte
		Value       []byte
		BlockNumber uint64
	}{
		{
			Key:         []byte("Key1"),
			Value:       []byte("Value1"),
			BlockNumber: 0,
		},
		{
			Key:         []byte("Key1"),
			Value:       []byte("Value2"),
			BlockNumber: 2,
		},
		{
			Key:         []byte("Key1"),
			Value:       []byte("Value3"),
			BlockNumber: 5,
		},
		{
			Key:         []byte("Key2"),
			Value:       []byte("Value1"),
			BlockNumber: 1,
		},
		{
			Key:         []byte("Key2"),
			Value:       []byte("Value2"),
			BlockNumber: 3,
		},
		{
			Key:         []byte("Key2"),
			Value:       []byte("Value3"),
			BlockNumber: 4,
		},
	}

	database := NewKeyValueDb(t.TempDir(), 0)
	db := NewBlockSpecificDatabase(database)

	for _, test := range tests {
		err := db.Put(test.Key, test.BlockNumber, test.Value)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	}
	db.Close()
}

func TestBlockSpecificDatabase_Get(t *testing.T) {
	data := [...]struct {
		Key         []byte
		Value       []byte
		BlockNumber uint64
	}{
		{
			Key:         []byte("Key1"),
			Value:       []byte("Value1"),
			BlockNumber: 0,
		},
		{
			Key:         []byte("Key1"),
			Value:       []byte("Value2"),
			BlockNumber: 2,
		},
		{
			Key:         []byte("Key2"),
			Value:       []byte("Value1"),
			BlockNumber: 3,
		},
		{
			Key:         []byte("Key1"),
			Value:       []byte("Value3"),
			BlockNumber: 5,
		},
	}

	database := NewKeyValueDb(t.TempDir(), 0)
	db := NewBlockSpecificDatabase(database)

	for _, d := range data {
		err := db.Put(d.Key, d.BlockNumber, d.Value)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	}

	tests := [...]struct {
		Key         []byte
		BlockNumber uint64
		Want        []byte
	}{
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value1"),
			BlockNumber: 0,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value1"),
			BlockNumber: 1,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value2"),
			BlockNumber: 2,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value2"),
			BlockNumber: 3,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value3"),
			BlockNumber: 5,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value3"),
			BlockNumber: 50000,
		},
		{
			Key:         []byte("Key2"),
			Want:        nil,
			BlockNumber: 1,
		},
		{
			Key:         []byte("Key3"),
			Want:        nil,
			BlockNumber: 50000,
		},
	}
	for _, test := range tests {
		result, err := db.Get(test.Key, test.BlockNumber)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
		if bytes.Compare(result, test.Want) != 0 {
			t.Errorf("db.Get(%s, %d) = %s, want: %s", string(test.Key), test.BlockNumber, string(result), test.Want)
		}
	}
	db.Close()
}
