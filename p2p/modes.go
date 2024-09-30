package p2p

import (
	"encoding"
	"fmt"

	"github.com/spf13/pflag"
)

// The following are necessary for Cobra and Viper, respectively, to unmarshal
// CLI/config parameters properly.
var (
	_ pflag.Value              = (*SyncMode)(nil)
	_ encoding.TextUnmarshaler = (*SyncMode)(nil)
)

// SyncMode represents the synchronisation mode of the downloader.
// It is a uint32 as it is used with atomic operations.
type SyncMode uint32

const (
	FullSync SyncMode = iota // Synchronize by downloading blocks and applying them to the chain sequentially
	SnapSync                 // Download the chain and the state via snap protocol
)

func (s SyncMode) IsValid() bool {
	return s == FullSync || s == SnapSync
}

func (s SyncMode) String() string {
	switch s {
	case FullSync:
		return "full"
	case SnapSync:
		return "snap"
	default:
		return "unknown"
	}
}

func (s SyncMode) Type() string {
	return "SyncMode"
}

func (s SyncMode) MarshalYAML() (interface{}, error) {
	return s.String(), nil
}

func (s *SyncMode) Set(mode string) error {
	switch mode {
	case "full":
		*s = FullSync
	case "snap":
		*s = SnapSync
	default:
		return fmt.Errorf("unknown sync mode %q, want \"full\" or \"snap\"", mode)
	}
	return nil
}

func (s SyncMode) MarshalText() ([]byte, error) {
	switch s {
	case FullSync:
		return []byte("full"), nil
	case SnapSync:
		return []byte("snap"), nil
	default:
		return nil, fmt.Errorf("unknown sync mode %d", s)
	}
}

func (s *SyncMode) UnmarshalText(text []byte) error {
	return s.Set(string(text))
}
