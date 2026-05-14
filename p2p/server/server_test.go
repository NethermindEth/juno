package server

import (
	"testing"

	synccommon "github.com/starknet-io/starknet-p2p-specs/p2p/proto/sync/common"
	"github.com/stretchr/testify/require"
)

func TestNewIterator_Direction(t *testing.T) {
	tests := []struct {
		name      string
		direction synccommon.Iteration_Direction
		wantErr   string
	}{
		{"forward", synccommon.Iteration_Forward, ""},
		{"backward", synccommon.Iteration_Backward, ""},
		{"unknown", synccommon.Iteration_Direction(99), "unknown direction value: 99"},
	}

	h := &Server{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := &synccommon.Iteration{
				Direction: tt.direction,
				Start:     &synccommon.Iteration_BlockNumber{BlockNumber: 1},
				Limit:     1,
				Step:      1,
			}

			iter, err := h.newIterator(it)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Nil(t, iter)
			} else {
				require.NoError(t, err)
				require.NotNil(t, iter)
			}
		})
	}
}
