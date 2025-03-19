package plugin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/NethermindEth/juno/mocks"
	"github.com/NethermindEth/juno/plugin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestService(t *testing.T) {
	t.Run("shutdown ok", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		p := mocks.NewMockJunoPlugin(ctrl)
		p.EXPECT().Shutdown().Return(nil)
		service := plugin.NewService(p)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		// after ^ this ctx already cancelled

		err := service.Run(ctx)
		require.NoError(t, err)
	})
	t.Run("shutdown with error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		shutdownErr := errors.New("error during shutdown")

		p := mocks.NewMockJunoPlugin(ctrl)
		p.EXPECT().Shutdown().Return(shutdownErr)
		service := plugin.NewService(p)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		// after ^ this ctx already cancelled

		err := service.Run(ctx)
		require.Equal(t, shutdownErr, err)
	})
}
