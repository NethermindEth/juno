package jsonrpc

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxWindowSize  = 512
	maxLineWidth   = 80
	maxContextRows = 3
)

// windowBuffer keeps the last maxWindowSize bytes read
type windowBuffer struct {
	window        []byte
	consumedBytes int
	newlinesSeen  int
}

func (c *windowBuffer) Write(p []byte) (int, error) {
	c.consumedBytes += len(p)
	c.newlinesSeen += bytes.Count(p, []byte{'\n'})

	if len(p) >= maxWindowSize {
		c.window = append(c.window[:0], p[len(p)-maxWindowSize:]...)
		return len(p), nil
	}

	if overflow := len(c.window) + len(p) - maxWindowSize; overflow > 0 {
		c.window = c.window[:copy(c.window, c.window[overflow:])]
	}
	c.window = append(c.window, p...)

	return len(p), nil
}

func errorOffset(inputLength int, err error) (offset int, ok bool) {
	var (
		syntaxErr *jsontext.SyntacticError
		typeErr   *json.SemanticError
	)
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return inputLength, true
	case errors.As(err, &syntaxErr):
		return min(int(syntaxErr.ByteOffset), inputLength), true
	case errors.As(err, &typeErr):
		return min(int(typeErr.ByteOffset), inputLength), true
	default:
		return 0, false
	}
}

// pastComma moves offset from a comma to the next token, as the decoder reports a
// missing value at the comma before it
func pastComma(input []byte, offset int) int {
	if offset >= len(input) || input[offset] != ',' {
		return offset
	}
	rest := bytes.TrimLeft(input[offset+1:], " \t\r\n")
	if len(rest) == 0 {
		return offset
	}
	return len(input) - len(rest)
}

func lineAndColumn(c *windowBuffer, markerPos int) (line, col int) {
	line = c.newlinesSeen - bytes.Count(c.window[markerPos:], []byte{'\n'}) + 1
	lineStart := bytes.LastIndexByte(c.window[:markerPos], '\n') + 1
	col = utf8.RuneCount(c.window[lineStart:markerPos]) + 1
	return line, col
}

// expectedToken maps the decoder reason, which uses parser jargon, to a
// user-friendly message. prev is the last token byte before the error.
func expectedToken(reason string, prev byte) (string, bool) {
	switch {
	case strings.Contains(reason, "after object name"):
		return "':'", true
	case strings.Contains(reason, "after object value"):
		return "',' or '}'", true
	case strings.Contains(reason, "after array element"):
		return "',' or ']'", true
	case strings.Contains(reason, "at start of string"),
		strings.Contains(reason, "at start of value") && prev == '{':
		return "a string key or '}'", true
	case strings.Contains(reason, "at start of value") && (prev == ':' || prev == '['):
		return "a value", true
	default:
		return "", false
	}
}

func previousToken(input []byte, offset int) byte {
	trimmed := bytes.TrimRight(input[:offset], " \t\r\n")
	if len(trimmed) == 0 {
		return 0
	}
	return trimmed[len(trimmed)-1]
}

func kindName(kind jsontext.Kind) string {
	switch kind {
	case 't', 'f':
		return "bool"
	case '{':
		return "object"
	case '[':
		return "array"
	default:
		return kind.String()
	}
}

func describeTypeError(e *json.SemanticError) string {
	if e.JSONPointer != "" {
		field := strings.Join(slices.Collect(e.JSONPointer.Tokens()), ".")
		return fmt.Sprintf("field %q should be %s, got %s", field, e.GoType, kindName(e.JSONKind))
	}
	return fmt.Sprintf("expected a JSON object, got %s", kindName(e.JSONKind))
}

func describeSyntaxError(input []byte, offset int, err *jsontext.SyntacticError) string {
	reason := err.Err.Error()
	if strings.Contains(reason, "after top-level value") {
		return "unexpected data after the request"
	}
	if offset >= len(input) {
		return reason
	}
	symbol, _ := utf8.DecodeRune(input[offset:])
	prev := previousToken(input, offset)
	if (symbol == '}' || symbol == ']') && prev == ',' {
		return fmt.Sprintf("unexpected trailing comma before %q", symbol)
	}
	expected, ok := expectedToken(reason, prev)
	if !ok {
		return reason
	}
	return fmt.Sprintf("unexpected %q, expected %s", symbol, expected)
}

func describeError(input []byte, offset int, err error) string {
	var (
		syntaxErr *jsontext.SyntacticError
		typeErr   *json.SemanticError
	)
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return "unexpected end of input"
	case errors.As(err, &syntaxErr):
		return describeSyntaxError(input, offset, syntaxErr)
	case errors.As(err, &typeErr):
		return describeTypeError(typeErr)
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
	maxContextSize := maxLineWidth - 2*len(ellipsis)
	pivotIdx := min(pivot-1, len(runes))
	start := max(0, pivotIdx-maxContextSize/2)
	end := min(start+maxContextSize, len(runes))
	start = max(0, end-maxContextSize)

	left, right := "", ""
	if start > 0 {
		left = ellipsis
	}
	if end < len(runes) {
		right = ellipsis
	}
	return left + string(runes[start:end]) + right, pivotIdx - start + len(left) + 1
}

func precedingLines(window []byte, windowStart, markerPos int) []string {
	lineStart := bytes.LastIndexByte(window[:markerPos], '\n') + 1

	var rows []string
	for len(rows) < maxContextRows && lineStart > 0 {
		prevEnd := lineStart - 1
		prevStart := bytes.LastIndexByte(window[:prevEnd], '\n') + 1
		if prevStart == 0 && windowStart > 0 {
			break // the topmost line was cut off by the window
		}

		row, _ := truncateAround(string(window[prevStart:prevEnd]), 1, maxLineWidth)
		rows = append(rows, row)
		lineStart = prevStart
	}

	slices.Reverse(rows)
	return rows
}

func drawMarker(window []byte, windowStart, markerPos, col int, msg string) string {
	var fullMsg strings.Builder
	for _, row := range precedingLines(window, windowStart, markerPos) {
		fullMsg.WriteString(row)
		fullMsg.WriteByte('\n')
	}

	line, markerCol := truncateAround(offendingLine(window, markerPos), col, maxLineWidth)
	gap := strings.Repeat(" ", markerCol-1)

	fmt.Fprintf(&fullMsg, "%s\n%s^\n%s", line, gap, msg)
	return fullMsg.String()
}

func prettyParseError(c *windowBuffer, err error) string {
	absOffset, ok := errorOffset(c.consumedBytes, err)
	if !ok {
		return err.Error()
	}

	windowStart := c.consumedBytes - len(c.window)
	if absOffset < windowStart {
		// Do not show the caret if the error is no longer inside the message
		return describeError(nil, 0, err)
	}

	markerPos := absOffset - windowStart
	if errors.As(err, new(*jsontext.SyntacticError)) {
		markerPos = pastComma(c.window, markerPos)
	}
	line, col := lineAndColumn(c, markerPos)
	msg := fmt.Sprintf("%s [line %d, position %d]", describeError(c.window, markerPos, err), line, col)

	return drawMarker(c.window, windowStart, markerPos, col, msg)
}
