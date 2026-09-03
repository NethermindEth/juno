package jsonrpc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
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
	window          []byte
	consumedBytes   int
	newlinesSeen    int
	linePrefixRunes int
}

func (c *windowBuffer) Write(p []byte) (int, error) {
	c.consumedBytes += len(p)
	c.newlinesSeen += bytes.Count(p, []byte{'\n'})

	if len(p) >= maxWindowSize {
		start := nextRuneStartAt(p, len(p)-maxWindowSize)
		c.advanceLinePrefix(c.window, p[:start])
		c.window = append(c.window[:0], p[start:]...)
		return len(p), nil
	}

	c.window = append(c.window, p...)
	if overflow := len(c.window) - maxWindowSize; overflow > 0 {
		start := nextRuneStartAt(c.window, overflow)
		c.advanceLinePrefix(nil, c.window[:start])
		c.window = c.window[:copy(c.window, c.window[start:])]
	}

	return len(p), nil
}

func nextRuneStartAt(p []byte, offset int) int {
	for offset < len(p) && !utf8.RuneStart(p[offset]) {
		offset++
	}
	return offset
}

func (c *windowBuffer) advanceLinePrefix(first, second []byte) {
	if lastNewline := bytes.LastIndexByte(second, '\n'); lastNewline >= 0 {
		c.linePrefixRunes = countRuneStarts(second[lastNewline+1:])
		return
	}
	if lastNewline := bytes.LastIndexByte(first, '\n'); lastNewline >= 0 {
		c.linePrefixRunes = countRuneStarts(first[lastNewline+1:]) + countRuneStarts(second)
		return
	}
	c.linePrefixRunes += countRuneStarts(first) + countRuneStarts(second)
}

func countRuneStarts(p []byte) int {
	count := 0
	for len(p) >= 32 {
		count += countRuneStartsInWord(binary.LittleEndian.Uint64(p))
		count += countRuneStartsInWord(binary.LittleEndian.Uint64(p[8:]))
		count += countRuneStartsInWord(binary.LittleEndian.Uint64(p[16:]))
		count += countRuneStartsInWord(binary.LittleEndian.Uint64(p[24:]))
		p = p[32:]
	}
	for len(p) >= 8 {
		count += countRuneStartsInWord(binary.LittleEndian.Uint64(p))
		p = p[8:]
	}
	for _, b := range p {
		if utf8.RuneStart(b) {
			count++
		}
	}
	return count
}

func countRuneStartsInWord(word uint64) int {
	const highBits = uint64(0x8080808080808080)
	if word&highBits == 0 {
		return 8
	}

	count := 0
	for range 8 {
		if utf8.RuneStart(byte(word)) {
			count++
		}
		word >>= 8
	}
	return count
}

func errorOffset(inputLength int, err error) (offset int, ok bool) {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &syntaxErr):
		return min(int(syntaxErr.Offset)-1, inputLength), true
	case errors.As(err, &typeErr):
		return min(int(typeErr.Offset)-1, inputLength), true
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return inputLength, true
	default:
		return 0, false
	}
}

func lineAndColumn(c *windowBuffer, markerPos int) (line, relativeCol, absoluteCol int) {
	line = c.newlinesSeen - bytes.Count(c.window[markerPos:], []byte{'\n'}) + 1
	lineStart := bytes.LastIndexByte(c.window[:markerPos], '\n') + 1
	relativeCol = countRuneStarts(c.window[lineStart:markerPos]) + 1
	absoluteCol = relativeCol
	if lineStart == 0 {
		absoluteCol += c.linePrefixRunes
	}
	return line, relativeCol, absoluteCol
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
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
	)
	switch {
	case errors.As(err, &syntaxErr):
		return describeSyntaxError(input, offset, err)
	case errors.As(err, &typeErr):
		return describeTypeError(typeErr)
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return "unexpected end of input"
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
	line, relativeCol, absoluteCol := lineAndColumn(c, markerPos)
	msg := fmt.Sprintf("%s [line %d, position %d]", describeError(c.window, markerPos, err), line, absoluteCol)

	return drawMarker(c.window, windowStart, markerPos, relativeCol, msg)
}
