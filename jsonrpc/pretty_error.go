package jsonrpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

func parseErrorOffset(input []byte, err error) (offset int, ok bool) {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &syntaxErr):
		return min(int(syntaxErr.Offset)-1, len(input)), true
	case errors.As(err, &typeErr):
		return min(int(typeErr.Offset)-1, len(input)), true
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return len(input), true
	default:
		// TODO(granza): when we add SONIC, the errors will be already pretty.
		return 0, false
	}
}

func lineAndColumn(input []byte, offset int) (line, col int) {
	before := input[:offset]
	line = bytes.Count(before, []byte{'\n'}) + 1
	lineStart := bytes.LastIndexByte(before, '\n') + 1
	col = utf8.RuneCount(before[lineStart:]) + 1
	return line, col
}

func expectedToken(reason string) (string, bool) {
	// The stdlib reason uses parser jargon
	// This maps it to user-friendly messages
	switch {
	case strings.Contains(reason, "beginning of object key string"):
		return "a string key or '}'", true
	case strings.Contains(reason, "after object key:value pair"):
		return "',' or '}'", true
	case strings.Contains(reason, "after object key"):
		return "':'", true
	case strings.Contains(reason, "after array element"):
		return "',' or ']'", true
	case strings.Contains(reason, "beginning of value"):
		return "a value", true
	default:
		return "", false
	}
}

func precededByComma(input []byte, offset int) bool {
	trimmed := bytes.TrimRight(input[:offset], " \t\r\n")
	return len(trimmed) > 0 && trimmed[len(trimmed)-1] == ','
}

func describeTypeError(e *json.UnmarshalTypeError) string {
	if e.Field != "" {
		return fmt.Sprintf("field %q should be %s, got %s", e.Field, e.Type, e.Value)
	}
	return fmt.Sprintf("expected a JSON object, got %s", e.Value)
}

func describeSyntaxError(input []byte, offset int, err error) string {
	expected, ok := expectedToken(err.Error())
	if !ok || offset >= len(input) {
		return err.Error()
	}
	symbol, _ := utf8.DecodeRune(input[offset:])
	if (symbol == '}' || symbol == ']') && precededByComma(input, offset) {
		return fmt.Sprintf("unexpected trailing comma before %q", symbol)
	}
	return fmt.Sprintf("unexpected %q, expected %s", symbol, expected)
}

func describeError(input []byte, offset int, err error) string {
	var (
		typeErr   *json.UnmarshalTypeError
		syntaxErr *json.SyntaxError
	)
	switch {
	case errors.As(err, &typeErr):
		return describeTypeError(typeErr)
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return "unexpected end of input"
	case errors.As(err, &syntaxErr):
		return describeSyntaxError(input, offset, err)
	default:
		return err.Error()
	}
}

func offendingLine(input []byte, offset int) string {
	start := bytes.LastIndexByte(input[:offset], '\n') + 1
	if end := bytes.IndexByte(input[offset:], '\n'); end >= 0 {
		return string(input[start : offset+end])
	}
	return string(input[start:])
}

// Returns the truncated string around a pivot and the new index of it
func truncateAround(line string, pivot, maxLineWidth int) (string, int) {
	runes := []rune(line)
	if len(runes) <= maxLineWidth {
		return line, pivot
	}

	const ellipsis = "..."
	maxSize := maxLineWidth - 2*len(ellipsis)
	idx := min(pivot-1, len(runes))
	start := max(0, idx-maxSize/2)
	end := min(start+maxSize, len(runes))
	start = max(0, end-maxSize)

	left, right := "", ""
	if start > 0 {
		left = ellipsis
	}
	if end < len(runes) {
		right = ellipsis
	}
	return left + string(runes[start:end]) + right, idx - start + len(left) + 1
}

func drawMarker(input []byte, offset, col int, msg string) string {
	const maxLineWidth = 80

	line, markerCol := truncateAround(offendingLine(input, offset), col, maxLineWidth)
	gap := strings.Repeat(" ", markerCol-1)
	return fmt.Sprintf("%s\n%s^\n%s", line, gap, msg)
}

func prettyParseError(input []byte, err error) string {
	offset, ok := parseErrorOffset(input, err)
	if !ok {
		return err.Error()
	}

	line, col := lineAndColumn(input, offset)
	msg := fmt.Sprintf("%s [line %d, column %d]", describeError(input, offset, err), line, col)
	return drawMarker(input, offset, col, msg)
}
