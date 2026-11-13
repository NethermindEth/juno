package rpcsource

import (
	"encoding"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

type Mode uint8

const (
	Follow Mode = iota
	CatchUp
)

const (
	followName  = "follow"
	catchUpName = "catch-up"
)

var (
	_ pflag.Value              = (*Mode)(nil)
	_ encoding.TextUnmarshaler = (*Mode)(nil)
	_ encoding.TextMarshaler   = Follow
)

var errUnknownMode = fmt.Errorf("unknown RPC sync mode (known: %s, %s)", followName, catchUpName)

func (m Mode) String() string {
	switch m {
	case Follow:
		return followName
	case CatchUp:
		return catchUpName
	default:
		return fmt.Sprintf("Mode(%d)", uint8(m))
	}
}

func (m Mode) MarshalText() ([]byte, error) {
	return []byte(m.String()), nil
}

func (m Mode) MarshalYAML() (any, error) {
	return m.String(), nil
}

func (m *Mode) Set(s string) error {
	switch strings.ToLower(s) {
	case followName:
		*m = Follow
	case catchUpName:
		*m = CatchUp
	default:
		return errUnknownMode
	}

	return nil
}

func (m *Mode) Type() string {
	return "RPCSyncMode"
}

func (m *Mode) UnmarshalText(text []byte) error {
	return m.Set(string(text))
}
