package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const unexpectedEOFMsg = "unexpected end of input (missing closing '}' or ']'?)"

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

func parseErrorOffset(input []byte, err error) (offset int, ok bool) {
	var syntaxErr *json.SyntaxError
	switch {
	case errors.As(err, &syntaxErr):
		return clamp(int(syntaxErr.Offset)-1, 0, len(input)), true
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return len(input), true
	default:
		// TODO(granza): add a case for SONIC's *decoder.SyntaxError (.Pos) when it lands.
		return 0, false
	}
}

func lineAndColumn(input []byte, offset int) (line, col int) {
	before := input[:offset]
	line = bytes.Count(before, []byte{'\n'}) + 1
	col = offset - (bytes.LastIndexByte(before, '\n') + 1) + 1
	return line, col
}

func errorMessage(err error) string {
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return unexpectedEOFMsg
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		if strings.Contains(syntaxErr.Error(), "unexpected end of JSON input") {
			return unexpectedEOFMsg
		}
		return syntaxErr.Error()
	}
	return err.Error()
}

func prettyParseError(input []byte, err error) string {
	offset, ok := parseErrorOffset(input, err)
	if !ok {
		return err.Error()
	}

	line, col := lineAndColumn(input, offset)
	return fmt.Sprintf("%s at line %d column %d", errorMessage(err), line, col)
}
