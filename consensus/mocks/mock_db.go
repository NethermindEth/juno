// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/juno/consensus/db (interfaces: TendermintDB)
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_db.go -package=mocks github.com/NethermindEth/juno/consensus/db TendermintDB
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	db "github.com/NethermindEth/juno/consensus/db"
	types "github.com/NethermindEth/juno/consensus/types"
	gomock "go.uber.org/mock/gomock"
)

// MockTendermintDB is a mock of TendermintDB interface.
type MockTendermintDB[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	ctrl     *gomock.Controller
	recorder *MockTendermintDBMockRecorder[V, H, A]
	isgomock struct{}
}

// MockTendermintDBMockRecorder is the mock recorder for MockTendermintDB.
type MockTendermintDBMockRecorder[V types.Hashable[H], H types.Hash, A types.Addr] struct {
	mock *MockTendermintDB[V, H, A]
}

// NewMockTendermintDB creates a new mock instance.
func NewMockTendermintDB[V types.Hashable[H], H types.Hash, A types.Addr](ctrl *gomock.Controller) *MockTendermintDB[V, H, A] {
	mock := &MockTendermintDB[V, H, A]{ctrl: ctrl}
	mock.recorder = &MockTendermintDBMockRecorder[V, H, A]{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTendermintDB[V, H, A]) EXPECT() *MockTendermintDBMockRecorder[V, H, A] {
	return m.recorder
}

// DeleteWALEntries mocks base method.
func (m *MockTendermintDB[V, H, A]) DeleteWALEntries(height types.Height) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteWALEntries", height)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteWALEntries indicates an expected call of DeleteWALEntries.
func (mr *MockTendermintDBMockRecorder[V, H, A]) DeleteWALEntries(height any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteWALEntries", reflect.TypeOf((*MockTendermintDB[V, H, A])(nil).DeleteWALEntries), height)
}

// Flush mocks base method.
func (m *MockTendermintDB[V, H, A]) Flush() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Flush")
	ret0, _ := ret[0].(error)
	return ret0
}

// Flush indicates an expected call of Flush.
func (mr *MockTendermintDBMockRecorder[V, H, A]) Flush() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Flush", reflect.TypeOf((*MockTendermintDB[V, H, A])(nil).Flush))
}

// GetWALEntries mocks base method.
func (m *MockTendermintDB[V, H, A]) GetWALEntries(height types.Height) ([]db.WalEntry[V, H, A], error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetWALEntries", height)
	ret0, _ := ret[0].([]db.WalEntry[V, H, A])
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWALEntries indicates an expected call of GetWALEntries.
func (mr *MockTendermintDBMockRecorder[V, H, A]) GetWALEntries(height any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetWALEntries", reflect.TypeOf((*MockTendermintDB[V, H, A])(nil).GetWALEntries), height)
}

// SetWALEntry mocks base method.
func (m *MockTendermintDB[V, H, A]) SetWALEntry(entry types.Message[V, H, A]) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetWALEntry", entry)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetWALEntry indicates an expected call of SetWALEntry.
func (mr *MockTendermintDBMockRecorder[V, H, A]) SetWALEntry(entry any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetWALEntry", reflect.TypeOf((*MockTendermintDB[V, H, A])(nil).SetWALEntry), entry)
}
