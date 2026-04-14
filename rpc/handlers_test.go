package rpc

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/NethermindEth/juno/blockchain"
	"github.com/NethermindEth/juno/core"
	"github.com/NethermindEth/juno/feed"
	"github.com/NethermindEth/juno/jsonrpc"
	"github.com/NethermindEth/juno/mocks"
	rpcv10 "github.com/NethermindEth/juno/rpc/v10"
	rpcv6 "github.com/NethermindEth/juno/rpc/v6"
	rpcv8 "github.com/NethermindEth/juno/rpc/v8"
	rpcv9 "github.com/NethermindEth/juno/rpc/v9"
	"github.com/NethermindEth/juno/sync"
	"github.com/NethermindEth/juno/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestVersion(t *testing.T) {
	const version = "1.2.3-rc1"

	handler := New(nil, nil, nil, version, nil, nil)

	ver, err := handler.Version()
	require.Nil(t, err)
	assert.Equal(t, version, ver)
}

func TestRun(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	t.Cleanup(mockCtrl.Finish)

	l1Sub := feed.New[*core.L1Head]()
	newHeadsSub := feed.New[*core.Block]()
	reorgSub := feed.New[*sync.ReorgBlockRange]()
	preConfirmedSub := feed.New[*core.PreConfirmed]()
	preLatestSub := feed.New[*core.PreLatest]()

	mockBcReader := mocks.NewMockReader(mockCtrl)
	mockSyncReader := mocks.NewMockSyncReader(mockCtrl)
	mockBcReader.EXPECT().SubscribeL1Head().Return(
		blockchain.L1HeadSubscription{Subscription: l1Sub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribeNewHeads().Return(
		sync.NewHeadSubscription{Subscription: newHeadsSub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribeReorg().Return(
		sync.ReorgSubscription{Subscription: reorgSub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribePreConfirmed().Return(
		sync.PreConfirmedDataSubscription{Subscription: preConfirmedSub.Subscribe()},
	).AnyTimes()
	mockSyncReader.EXPECT().SubscribePreLatest().Return(
		sync.PreLatestDataSubscription{Subscription: preLatestSub.Subscribe()},
	).AnyTimes()

	handler := &Handler{
		rpcv6Handler:  rpcv6.New(mockBcReader, mockSyncReader, nil, nil, nil),
		rpcv8Handler:  rpcv8.New(mockBcReader, mockSyncReader, nil, nil),
		rpcv9Handler:  rpcv9.New(mockBcReader, mockSyncReader, nil, nil),
		rpcv10Handler: rpcv10.New(mockBcReader, mockSyncReader, nil, nil),
		version:       "",
	}

	ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	t.Cleanup(cancel)

	err := handler.Run(ctx)
	require.NoError(t, err)
}

// TestHandlerParamValidatorCompatibility verifies that every handler registered in a version's
// method list has parameter types compatible with that version's validator. This prevents panics
// caused by cross-version type mismatch(e.g., rpcv9.TransactionType validated by rpcv10.Validator)
func TestHandlerParamValidatorCompatibility(t *testing.T) {
	t.Parallel()

	log := utils.NewNopZapLogger()
	handler := &Handler{
		rpcv6Handler:  rpcv6.New(nil, nil, nil, nil, log),
		rpcv8Handler:  rpcv8.New(nil, nil, nil, log),
		rpcv9Handler:  rpcv9.New(nil, nil, nil, log),
		rpcv10Handler: rpcv10.New(nil, nil, nil, log),
		version:       "test",
	}

	methodsV10, _ := handler.MethodsV0_10()
	methodsV09, _ := handler.MethodsV0_9()
	methodsV08, _ := handler.MethodsV0_8()

	versionTests := []struct {
		name      string
		methods   []jsonrpc.Method
		validator *validator.Validate
	}{
		{"v0.10", methodsV10, rpcv10.Validator()},
		{"v0.9", methodsV09, rpcv9.Validator()},
		{"v0.8", methodsV08, rpcv8.Validator()},
	}

	contextType := reflect.TypeFor[context.Context]()

	for _, vt := range versionTests {
		t.Run(vt.name, func(t *testing.T) {
			t.Parallel()
			for _, method := range vt.methods {
				t.Run(method.Name, func(t *testing.T) {
					t.Parallel()
					require.NotNil(t, method.Handler, "registered method %q must have a handler", method.Name)

					handlerType := reflect.TypeOf(method.Handler)
					require.Equal(
						t,
						reflect.Func,
						handlerType.Kind(),
						"registered method %q handler must be a function", method.Name,
					)

					startIdx := 0
					if handlerType.NumIn() > 0 && handlerType.In(0).Implements(contextType) {
						startIdx = 1
					}

					for i := startIdx; i < handlerType.NumIn(); i++ {
						visited := make(map[reflect.Type]bool)
						assertValidatorCompatible(t, vt.validator, handlerType.In(i), method.Name, i, visited)
					}
				})
			}
		})
	}
}

// assertValidatorCompatible creates a zero-value of the given type and validates it through
// the validator, asserting no panic. Recurses into struct fields, slice elements, and map values
// to catch nested type mismatches (e.g., LimitSlice[BroadcastedTransaction] where the zero-value
// slice is nil and dive tags won't exercise the element type).
func assertValidatorCompatible(
	t *testing.T,
	v *validator.Validate,
	paramType reflect.Type,
	methodName string,
	paramIdx int,
	visited map[reflect.Type]bool,
) {
	t.Helper()

	actualType := paramType
	for actualType.Kind() == reflect.Pointer {
		actualType = actualType.Elem()
	}

	if visited[actualType] {
		return
	}
	visited[actualType] = true

	switch actualType.Kind() {
	case reflect.Struct:
		zeroVal := reflect.New(actualType).Interface()
		require.NotPanics(t, func() {
			_ = v.Struct(zeroVal)
		}, "validator panicked for method %s, param %d (type %s)", methodName, paramIdx, paramType)

		// Walk exported fields to catch nested types not exercised by zero-value validation
		// (e.g., LimitSlice with nil Data slice won't trigger dive into element type)
		for field := range actualType.Fields() {
			if !field.IsExported() {
				continue
			}
			assertValidatorCompatible(t, v, field.Type, methodName, paramIdx, visited)
		}

	case reflect.Slice, reflect.Array:
		assertValidatorCompatible(t, v, actualType.Elem(), methodName, paramIdx, visited)

	case reflect.Map:
		assertValidatorCompatible(t, v, actualType.Elem(), methodName, paramIdx, visited)
	}
}
