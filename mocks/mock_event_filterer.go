// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/juno/blockchain (interfaces: EventFilterer)
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_event_filterer.go -package=mocks github.com/NethermindEth/juno/blockchain EventFilterer
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	blockchain "github.com/NethermindEth/juno/blockchain"
	felt "github.com/NethermindEth/juno/core/felt"
	gomock "go.uber.org/mock/gomock"
)

// MockEventFilterer is a mock of EventFilterer interface.
type MockEventFilterer struct {
	ctrl     *gomock.Controller
	recorder *MockEventFiltererMockRecorder
	isgomock struct{}
}

// MockEventFiltererMockRecorder is the mock recorder for MockEventFilterer.
type MockEventFiltererMockRecorder struct {
	mock *MockEventFilterer
}

// NewMockEventFilterer creates a new mock instance.
func NewMockEventFilterer(ctrl *gomock.Controller) *MockEventFilterer {
	mock := &MockEventFilterer{ctrl: ctrl}
	mock.recorder = &MockEventFiltererMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockEventFilterer) EXPECT() *MockEventFiltererMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockEventFilterer) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockEventFiltererMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockEventFilterer)(nil).Close))
}

// Events mocks base method.
func (m *MockEventFilterer) Events(cToken *blockchain.ContinuationToken, chunkSize uint64) ([]*blockchain.FilteredEvent, *blockchain.ContinuationToken, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Events", cToken, chunkSize)
	ret0, _ := ret[0].([]*blockchain.FilteredEvent)
	ret1, _ := ret[1].(*blockchain.ContinuationToken)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Events indicates an expected call of Events.
func (mr *MockEventFiltererMockRecorder) Events(cToken, chunkSize any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Events", reflect.TypeOf((*MockEventFilterer)(nil).Events), cToken, chunkSize)
}

// SetRangeEndBlockByHash mocks base method.
func (m *MockEventFilterer) SetRangeEndBlockByHash(filterRange blockchain.EventFilterRange, blockHash *felt.Felt) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetRangeEndBlockByHash", filterRange, blockHash)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetRangeEndBlockByHash indicates an expected call of SetRangeEndBlockByHash.
func (mr *MockEventFiltererMockRecorder) SetRangeEndBlockByHash(filterRange, blockHash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetRangeEndBlockByHash", reflect.TypeOf((*MockEventFilterer)(nil).SetRangeEndBlockByHash), filterRange, blockHash)
}

// SetRangeEndBlockByNumber mocks base method.
func (m *MockEventFilterer) SetRangeEndBlockByNumber(filterRange blockchain.EventFilterRange, blockNumber uint64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetRangeEndBlockByNumber", filterRange, blockNumber)
	ret0, _ := ret[0].(error)
	return ret0
}

// SetRangeEndBlockByNumber indicates an expected call of SetRangeEndBlockByNumber.
func (mr *MockEventFiltererMockRecorder) SetRangeEndBlockByNumber(filterRange, blockNumber any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetRangeEndBlockByNumber", reflect.TypeOf((*MockEventFilterer)(nil).SetRangeEndBlockByNumber), filterRange, blockNumber)
}

// WithLimit mocks base method.
func (m *MockEventFilterer) WithLimit(limit uint) *blockchain.EventFilter {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithLimit", limit)
	ret0, _ := ret[0].(*blockchain.EventFilter)
	return ret0
}

// WithLimit indicates an expected call of WithLimit.
func (mr *MockEventFiltererMockRecorder) WithLimit(limit any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithLimit", reflect.TypeOf((*MockEventFilterer)(nil).WithLimit), limit)
}
