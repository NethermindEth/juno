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
	isgomock struct{}
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
func (m *MockVM) Call(callInfo *vm.CallInfo, blockInfo *vm.BlockInfo, state core.StateReader, network *utils.Network, maxSteps uint64) ([]*felt.Felt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Call", callInfo, blockInfo, state, network, maxSteps)
	ret0, _ := ret[0].([]*felt.Felt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Call indicates an expected call of Call.
func (mr *MockVMMockRecorder) Call(callInfo, blockInfo, state, network, maxSteps any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Call", reflect.TypeOf((*MockVM)(nil).Call), callInfo, blockInfo, state, network, maxSteps)
}

// Execute mocks base method.
func (m *MockVM) Execute(txns []core.Transaction, declaredClasses []core.Class, paidFeesOnL1 []*felt.Felt, blockInfo *vm.BlockInfo, state core.StateReader, network *utils.Network, skipChargeFee, skipValidate, errOnRevert bool) ([]*felt.Felt, []core.GasConsumed, []vm.TransactionTrace, []vm.TransactionReceipt, uint64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Execute", txns, declaredClasses, paidFeesOnL1, blockInfo, state, network, skipChargeFee, skipValidate, errOnRevert)
	ret0, _ := ret[0].([]*felt.Felt)
	ret1, _ := ret[1].([]core.GasConsumed)
	ret2, _ := ret[2].([]vm.TransactionTrace)
	ret3, _ := ret[3].([]vm.TransactionReceipt)
	ret4, _ := ret[4].(uint64)
	ret5, _ := ret[5].(error)
	return ret0, ret1, ret2, ret3, ret4, ret5
}

// Execute indicates an expected call of Execute.
func (mr *MockVMMockRecorder) Execute(txns, declaredClasses, paidFeesOnL1, blockInfo, state, network, skipChargeFee, skipValidate, errOnRevert any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Execute", reflect.TypeOf((*MockVM)(nil).Execute), txns, declaredClasses, paidFeesOnL1, blockInfo, state, network, skipChargeFee, skipValidate, errOnRevert)
}
