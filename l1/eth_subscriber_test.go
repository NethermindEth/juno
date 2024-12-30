package l1

import (
	"context"
	"testing"

	"github.com/NethermindEth/juno/mocks"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestIPAddressRegistry(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockIPAddressGetter := mocks.NewMockIPAddressGetter(ctrl)
	mockIPAddressGetter.EXPECT().GetIPAddresses(gomock.Any()).Return([]string{}, nil).Times(1)

	mockIPWatcher := mocks.NewMockIPWatcher(ctrl)
	mockIPWatcher.EXPECT().WatchIPAdded(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	mockIPWatcher.EXPECT().WatchIPRemoved(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)

	subscriber := &EthSubscriber{
		ipAddressRegistry:         mockIPAddressGetter,
		ipAddressRegistryFilterer: mockIPWatcher,
	}

	_, err := subscriber.GetIPAddresses(context.Background(), common.Address{})
	require.NoError(t, err)
	_, err = subscriber.WatchIPAdded(context.Background(), nil)
	require.NoError(t, err)
	_, err = subscriber.WatchIPRemoved(context.Background(), nil)
	require.NoError(t, err)
}
