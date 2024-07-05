// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/juno/core/trie (interfaces: ClassesTrie)
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_trie.go -package=mocks github.com/NethermindEth/juno/core/trie ClassesTrie
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	felt "github.com/NethermindEth/juno/core/felt"
	trie "github.com/NethermindEth/juno/core/trie"
	gomock "go.uber.org/mock/gomock"
)

// MockClassesTrie is a mock of ClassesTrie interface.
type MockClassesTrie struct {
	ctrl     *gomock.Controller
	recorder *MockClassesTrieMockRecorder
}

// MockClassesTrieMockRecorder is the mock recorder for MockClassesTrie.
type MockClassesTrieMockRecorder struct {
	mock *MockClassesTrie
}

// NewMockClassesTrie creates a new mock instance.
func NewMockClassesTrie(ctrl *gomock.Controller) *MockClassesTrie {
	mock := &MockClassesTrie{ctrl: ctrl}
	mock.recorder = &MockClassesTrieMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClassesTrie) EXPECT() *MockClassesTrieMockRecorder {
	return m.recorder
}

// FeltToKey mocks base method.
func (m *MockClassesTrie) FeltToKey(arg0 *felt.Felt) trie.Key {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FeltToKey", arg0)
	ret0, _ := ret[0].(trie.Key)
	return ret0
}

// FeltToKey indicates an expected call of FeltToKey.
func (mr *MockClassesTrieMockRecorder) FeltToKey(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FeltToKey", reflect.TypeOf((*MockClassesTrie)(nil).FeltToKey), arg0)
}

// NodesFromRoot mocks base method.
func (m *MockClassesTrie) NodesFromRoot(arg0 *trie.Key) ([]trie.StorageNode, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodesFromRoot", arg0)
	ret0, _ := ret[0].([]trie.StorageNode)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NodesFromRoot indicates an expected call of NodesFromRoot.
func (mr *MockClassesTrieMockRecorder) NodesFromRoot(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodesFromRoot", reflect.TypeOf((*MockClassesTrie)(nil).NodesFromRoot), arg0)
}
