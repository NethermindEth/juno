package db

import (
	"bytes"
	"testing"
)

type TestKey []byte

func (x *TestKey) Marshal() ([]byte, error) {
	return *x, nil
}

type TestValue []byte

func (x *TestValue) Marshal() ([]byte, error) {
	return *x, nil
}

func (x *TestValue) Unmarshal(data []byte) error {
	*x = data
	return nil
}

func TestBlockSpecificDatabase_Put(t *testing.T) {
	tests := [...]struct {
		Key         TestKey
		Value       TestValue
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

	database := New(t.TempDir(), 0)
	db := NewBlockSpecificDatabase(database)

	for _, test := range tests {
		db.Put(&test.Key, test.BlockNumber, &test.Value)
	}
	db.Close()
}

func TestBlockSpecificDatabase_Get(t *testing.T) {
	data := [...]struct {
		Key         TestKey
		Value       TestValue
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

	database := New(t.TempDir(), 0)
	db := NewBlockSpecificDatabase(database)

	for _, d := range data {
		db.Put(&d.Key, d.BlockNumber, &d.Value)
	}

	tests := [...]struct {
		Key         TestKey
		BlockNumber uint64
		Want        TestValue
		Ok          bool
	}{
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value1"),
			BlockNumber: 0,
			Ok:          true,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value1"),
			BlockNumber: 1,
			Ok:          true,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value2"),
			BlockNumber: 2,
			Ok:          true,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value2"),
			BlockNumber: 3,
			Ok:          true,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value3"),
			BlockNumber: 5,
			Ok:          true,
		},
		{
			Key:         []byte("Key1"),
			Want:        []byte("Value3"),
			BlockNumber: 50000,
			Ok:          true,
		},
		{
			Key:         []byte("Key2"),
			Want:        []byte{},
			BlockNumber: 1,
			Ok:          false,
		},
		{
			Key:         []byte("Key3"),
			Want:        []byte{},
			BlockNumber: 50000,
			Ok:          false,
		},
	}
	for _, test := range tests {
		var result TestValue
		ok := db.Get(&test.Key, test.BlockNumber, &result)
		if ok != test.Ok {
			t.Errorf("unexpected OK result for Get(%s,%d)", string(test.Key), test.BlockNumber)
		}
		if ok && bytes.Compare(result, test.Want) != 0 {
			t.Errorf("db.Get(%s, %d) = %s, want: %s", string(test.Key), test.BlockNumber, string(result), test.Want)
		}
	}
	db.Close()
}
