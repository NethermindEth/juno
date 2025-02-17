// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/juno/rpc (interfaces: Gateway)
//
// Generated by this command:
//
//	mockgen -destination=../mocks/mock_gateway_handler.go -package=mocks github.com/NethermindEth/juno/rpc Gateway
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	json "encoding/json"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockGateway is a mock of Gateway interface.
type MockGateway struct {
	ctrl     *gomock.Controller
	recorder *MockGatewayMockRecorder
	isgomock struct{}
}

// MockGatewayMockRecorder is the mock recorder for MockGateway.
type MockGatewayMockRecorder struct {
	mock *MockGateway
}

// NewMockGateway creates a new mock instance.
func NewMockGateway(ctrl *gomock.Controller) *MockGateway {
	mock := &MockGateway{ctrl: ctrl}
	mock.recorder = &MockGatewayMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGateway) EXPECT() *MockGatewayMockRecorder {
	return m.recorder
}

// AddTransaction mocks base method.
func (m *MockGateway) AddTransaction(arg0 context.Context, arg1 json.RawMessage) (json.RawMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddTransaction", arg0, arg1)
	ret0, _ := ret[0].(json.RawMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddTransaction indicates an expected call of AddTransaction.
func (mr *MockGatewayMockRecorder) AddTransaction(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddTransaction", reflect.TypeOf((*MockGateway)(nil).AddTransaction), arg0, arg1)
}
