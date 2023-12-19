// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/juno/vm (interfaces: VM)
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_vm.go -package=mocks github.com/NethermindEth/juno/vm VM
//
// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	core "github.com/NethermindEth/juno/core"
	felt "github.com/NethermindEth/juno/core/felt"
	utils "github.com/NethermindEth/juno/utils"
	vm "github.com/NethermindEth/juno/vm"
	gomock "go.uber.org/mock/gomock"
)

// MockVM is a mock of VM interface.
type MockVM struct {
	ctrl     *gomock.Controller
	recorder *MockVMMockRecorder
}

// MockVMMockRecorder is the mock recorder for MockVM.
type MockVMMockRecorder struct {
	mock *MockVM
}

// NewMockVM creates a new mock instance.
func NewMockVM(ctrl *gomock.Controller) *MockVM {
	mock := &MockVM{ctrl: ctrl}
	mock.recorder = &MockVMMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockVM) EXPECT() *MockVMMockRecorder {
	return m.recorder
}

// Call mocks base method.
func (m *MockVM) Call(arg0, arg1, arg2 *felt.Felt, arg3 []felt.Felt, arg4, arg5 uint64, arg6 core.StateReader, arg7 utils.Network) ([]*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Call", arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7)
	ret0, _ := ret[0].([]*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Call indicates an expected call of Call.
func (mr *MockVMMockRecorder) Call(arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Call", reflect.TypeOf((*MockVM)(nil).Call), arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7)
}

// Execute mocks base method.
func (m *MockVM) Execute(arg0 []core.Transaction, arg1 []core.Class, arg2, arg3 uint64, arg4 *felt.Felt, arg5 core.StateReader, arg6 utils.Network, arg7 []*felt.Felt, arg8, arg9, arg10 bool, arg11, arg12 *felt.Felt, arg13 bool) ([]*felt.Felt, []vm.TransactionTrace, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10, arg11, arg12, arg13)
	ret0, _ := ret[0].([]*felt.Felt)
	ret1, _ := ret[1].([]vm.TransactionTrace)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Execute indicates an expected call of Execute.
func (mr *MockVMMockRecorder) Execute(arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10, arg11, arg12, arg13 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockVM)(nil).Execute), arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, arg8, arg9, arg10, arg11, arg12, arg13)
}
