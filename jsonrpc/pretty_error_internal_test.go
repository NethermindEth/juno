package jsonrpc

import (
	"bytes"
	"strings"
	"testing"
)

func TestLineAndColumnIncludesDiscardedLinePrefix(t *testing.T) {
	tests := map[string]struct {
		input     string
		chunkSize int
		wantLine  int
		wantCol   int
	}{
		"single large write": {
			input:     strings.Repeat("x", 700) + "@",
			chunkSize: 701,
			wantLine:  1,
			wantCol:   701,
		},
		"unicode split across writes": {
			input:     strings.Repeat("👍", 160) + "@",
			chunkSize: 127,
			wantLine:  1,
			wantCol:   161,
		},
		"discarded newline resets prefix": {
			input:     strings.Repeat("x", 600) + "\n" + strings.Repeat("👍", 160) + "@",
			chunkSize: 127,
			wantLine:  2,
			wantCol:   161,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var buffer windowBuffer
			input := []byte(test.input)
			for len(input) > 0 {
				n := min(test.chunkSize, len(input))
				if _, err := buffer.Write(input[:n]); err != nil {
					t.Fatalf("write failed: %v", err)
				}
				input = input[n:]
			}

			markerPos := bytes.IndexByte(buffer.window, '@')
			if markerPos < 0 {
				t.Fatal("marker is not in the retained window")
			}
			line, col := lineAndColumn(&buffer, markerPos)
			if line != test.wantLine || col != test.wantCol {
				t.Fatalf("lineAndColumn() = (%d, %d), want (%d, %d)", line, col, test.wantLine, test.wantCol)
			}
		})
	}
}
