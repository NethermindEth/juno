package jsonrpc

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrettyParseErrorAtWindowStart(t *testing.T) {
	var window windowBuffer
	_, err := window.Write([]byte(strings.Repeat(" ", 100) + strings.Repeat("1", maxWindowSize)))
	assert.NoError(t, err)

	typeErr := &json.SemanticError{
		ByteOffset:  100,
		JSONPointer: "/jsonrpc",
		JSONKind:    '0',
		GoType:      reflect.TypeFor[string](),
	}
	got := prettyParseError(&window, typeErr)

	want := strings.Repeat("1", 74) + "...\n^\n" +
		`field "jsonrpc" should be string, got number [line 1, position 1]`
	assert.Equal(t, want, got)
}
