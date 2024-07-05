// Code generated by MockGen. DO NOT EDIT.
// Source: ./state.go
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_state.go -package=mocks -source=./state.go
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	core "github.com/NethermindEth/juno/core"
	felt "github.com/NethermindEth/juno/core/felt"
	trie "github.com/NethermindEth/juno/core/trie" 
	gomock "go.uber.org/mock/gomock"
)

// MockStateHistoryReader is a mock of StateHistoryReader interface.
type MockStateHistoryReader struct {
	ctrl     *gomock.Controller
	recorder *MockStateHistoryReaderMockRecorder
}

// MockStateHistoryReaderMockRecorder is the mock recorder for MockStateHistoryReader.
type MockStateHistoryReaderMockRecorder struct {
	mock *MockStateHistoryReader
}

// NewMockStateHistoryReader creates a new mock instance.
func NewMockStateHistoryReader(ctrl *gomock.Controller) *MockStateHistoryReader {
	mock := &MockStateHistoryReader{ctrl: ctrl}
	mock.recorder = &MockStateHistoryReaderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStateHistoryReader) EXPECT() *MockStateHistoryReaderMockRecorder {
	return m.recorder
}

// Class mocks base method.
func (m *MockStateHistoryReader) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Class", classHash)
	ret0, _ := ret[0].(*core.DeclaredClass)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Class indicates an expected call of Class.
func (mr *MockStateHistoryReaderMockRecorder) Class(classHash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Class", reflect.TypeOf((*MockStateHistoryReader)(nil).Class), classHash)
}

// ClassesTrie mocks base method.
func (m *MockStateHistoryReader) ClassesTrie() (*trie.Trie, func() error, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClassesTrie")
	ret0, _ := ret[0].(*trie.Trie)
	ret1, _ := ret[1].(func() error)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ClassesTrie indicates an expected call of ClassesTrie.
func (mr *MockStateHistoryReaderMockRecorder) ClassesTrie() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClassesTrie", reflect.TypeOf((*MockStateHistoryReader)(nil).ClassesTrie))
}

// ContractClassHash mocks base method.
func (m *MockStateHistoryReader) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractClassHash", addr)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractClassHash indicates an expected call of ContractClassHash.
func (mr *MockStateHistoryReaderMockRecorder) ContractClassHash(addr any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractClassHash", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractClassHash), addr)
}

// ContractClassHashAt mocks base method.
func (m *MockStateHistoryReader) ContractClassHashAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractClassHashAt", addr, blockNumber)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractClassHashAt indicates an expected call of ContractClassHashAt.
func (mr *MockStateHistoryReaderMockRecorder) ContractClassHashAt(addr, blockNumber any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractClassHashAt", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractClassHashAt), addr, blockNumber)
}

// ContractIsAlreadyDeployedAt mocks base method.
func (m *MockStateHistoryReader) ContractIsAlreadyDeployedAt(addr *felt.Felt, blockNumber uint64) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractIsAlreadyDeployedAt", addr, blockNumber)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractIsAlreadyDeployedAt indicates an expected call of ContractIsAlreadyDeployedAt.
func (mr *MockStateHistoryReaderMockRecorder) ContractIsAlreadyDeployedAt(addr, blockNumber any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractIsAlreadyDeployedAt", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractIsAlreadyDeployedAt), addr, blockNumber)
}

// ContractNonce mocks base method.
func (m *MockStateHistoryReader) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractNonce", addr)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractNonce indicates an expected call of ContractNonce.
func (mr *MockStateHistoryReaderMockRecorder) ContractNonce(addr any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractNonce", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractNonce), addr)
}

// ContractNonceAt mocks base method.
func (m *MockStateHistoryReader) ContractNonceAt(addr *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractNonceAt", addr, blockNumber)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractNonceAt indicates an expected call of ContractNonceAt.
func (mr *MockStateHistoryReaderMockRecorder) ContractNonceAt(addr, blockNumber any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractNonceAt", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractNonceAt), addr, blockNumber)
}

// ContractStorage mocks base method.
func (m *MockStateHistoryReader) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractStorage", addr, key)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractStorage indicates an expected call of ContractStorage.
func (mr *MockStateHistoryReaderMockRecorder) ContractStorage(addr, key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractStorage", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractStorage), addr, key)
}

// ContractStorageAt mocks base method.
func (m *MockStateHistoryReader) ContractStorageAt(addr, key *felt.Felt, blockNumber uint64) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractStorageAt", addr, key, blockNumber)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractStorageAt indicates an expected call of ContractStorageAt.
func (mr *MockStateHistoryReaderMockRecorder) ContractStorageAt(addr, key, blockNumber any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractStorageAt", reflect.TypeOf((*MockStateHistoryReader)(nil).ContractStorageAt), addr, key, blockNumber)
}

// MockStateReader is a mock of StateReader interface.
type MockStateReader struct {
	ctrl     *gomock.Controller
	recorder *MockStateReaderMockRecorder
}

// MockStateReaderMockRecorder is the mock recorder for MockStateReader.
type MockStateReaderMockRecorder struct {
	mock *MockStateReader
}

// NewMockStateReader creates a new mock instance.
func NewMockStateReader(ctrl *gomock.Controller) *MockStateReader {
	mock := &MockStateReader{ctrl: ctrl}
	mock.recorder = &MockStateReaderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStateReader) EXPECT() *MockStateReaderMockRecorder {
	return m.recorder
}

// Class mocks base method.
func (m *MockStateReader) Class(classHash *felt.Felt) (*core.DeclaredClass, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Class", classHash)
	ret0, _ := ret[0].(*core.DeclaredClass)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Class indicates an expected call of Class.
func (mr *MockStateReaderMockRecorder) Class(classHash any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Class", reflect.TypeOf((*MockStateReader)(nil).Class), classHash)
}

// ClassesTrie mocks base method.
func (m *MockStateReader) ClassesTrie() (*trie.Trie, func() error, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClassesTrie")
	ret0, _ := ret[0].(*trie.Trie)
	ret1, _ := ret[1].(func() error)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// ClassesTrie indicates an expected call of ClassesTrie.
func (mr *MockStateReaderMockRecorder) ClassesTrie() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClassesTrie", reflect.TypeOf((*MockStateReader)(nil).ClassesTrie))
}

// ContractClassHash mocks base method.
func (m *MockStateReader) ContractClassHash(addr *felt.Felt) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractClassHash", addr)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractClassHash indicates an expected call of ContractClassHash.
func (mr *MockStateReaderMockRecorder) ContractClassHash(addr any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractClassHash", reflect.TypeOf((*MockStateReader)(nil).ContractClassHash), addr)
}

// ContractNonce mocks base method.
func (m *MockStateReader) ContractNonce(addr *felt.Felt) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractNonce", addr)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractNonce indicates an expected call of ContractNonce.
func (mr *MockStateReaderMockRecorder) ContractNonce(addr any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractNonce", reflect.TypeOf((*MockStateReader)(nil).ContractNonce), addr)
}

// ContractStorage mocks base method.
func (m *MockStateReader) ContractStorage(addr, key *felt.Felt) (*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ContractStorage", addr, key)
	ret0, _ := ret[0].(*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ContractStorage indicates an expected call of ContractStorage.
func (mr *MockStateReaderMockRecorder) ContractStorage(addr, key any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ContractStorage", reflect.TypeOf((*MockStateReader)(nil).ContractStorage), addr, key)
}
