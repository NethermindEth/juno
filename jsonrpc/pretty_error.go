package jsonrpc

import (
	"bytes"
	"encoding/binary"
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
	window          []byte
	consumedBytes   int
	newlinesSeen    int
	linePrefixRunes int
}

func (c *windowBuffer) Write(p []byte) (int, error) {
	c.consumedBytes += len(p)
	c.newlinesSeen += bytes.Count(p, []byte{'\n'})

	if len(p) >= maxWindowSize+utf8.UTFMax {
		start := nextRuneBoundary(p, len(p)-maxWindowSize)
		c.advanceLinePrefix(c.window, p[:start])
		c.window = append(c.window[:0], p[start:]...)
		return len(p), nil
	}

	// Join short reads before trimming: a rune may span the two buffers.
	c.window = append(c.window, p...)
	if overflow := len(c.window) - maxWindowSize; overflow > 0 {
		start := nextRuneBoundary(c.window, overflow)
		c.advanceLinePrefix(nil, c.window[:start])
		c.window = c.window[:copy(c.window, c.window[start:])]
	}

	return len(p), nil
}

func nextRuneBoundary(p []byte, offset int) int {
	// Decode from before the cut so invalid continuation bytes count as runes,
	// while a valid rune crossing the cut is discarded in full.
	start := max(0, offset-utf8.UTFMax+1)
	for start < offset {
		_, size := utf8.DecodeRune(p[start:])
		start += size
	}
	return start
}

func (c *windowBuffer) advanceLinePrefix(discardedWindow, discardedInput []byte) {
	if c.newlinesSeen > 0 {
		if lastNewline := bytes.LastIndexByte(discardedInput, '\n'); lastNewline >= 0 {
			c.linePrefixRunes = countRunes(discardedInput[lastNewline+1:])
			return
		}
		if lastNewline := bytes.LastIndexByte(discardedWindow, '\n'); lastNewline >= 0 {
			c.linePrefixRunes = countJoinedRunes(discardedWindow[lastNewline+1:], discardedInput)
			return
		}
	}
	c.linePrefixRunes += countJoinedRunes(discardedWindow, discardedInput)
}

func countJoinedRunes(window, input []byte) int {
	count := countRunes(window) + countRunes(input)
	if len(window) == 0 || len(input) == 0 {
		return count
	}

	start := len(window) - 1
	for start > 0 && len(window)-start < utf8.UTFMax && !utf8.RuneStart(window[start]) {
		start--
	}
	tail := window[start:]
	if !utf8.FullRune(tail) {
		var joined [utf8.UTFMax]byte
		n := copy(joined[:], tail)
		n += copy(joined[n:], input)
		if _, size := utf8.DecodeRune(joined[:n]); size > len(tail) {
			// Each fragment counted as invalid bytes on its own; together they
			// form one valid rune.
			count -= size - 1
		}
	}
	return count
}

func countRunes(p []byte) int {
	const highBits = uint64(0x8080808080808080)
	count := 0
	for len(p) >= 32 {
		word := binary.LittleEndian.Uint64(p) | binary.LittleEndian.Uint64(p[8:]) |
			binary.LittleEndian.Uint64(p[16:]) | binary.LittleEndian.Uint64(p[24:])
		if word&highBits != 0 {
			break
		}
		count += 32
		p = p[32:]
	}
	return count + utf8.RuneCount(p)
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

func lineAndColumn(c *windowBuffer, markerPos int) (line, relativeCol, absoluteCol int) {
	line = c.newlinesSeen - bytes.Count(c.window[markerPos:], []byte{'\n'}) + 1
	lineStart := bytes.LastIndexByte(c.window[:markerPos], '\n') + 1
	relativeCol = utf8.RuneCount(c.window[lineStart:markerPos]) + 1
	absoluteCol = relativeCol
	if lineStart == 0 {
		absoluteCol += c.linePrefixRunes
	}
	return line, relativeCol, absoluteCol
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
	line, relativeCol, absoluteCol := lineAndColumn(c, markerPos)
	msg := fmt.Sprintf(
		"%s [line %d, position %d]", describeError(c.window, markerPos, err), line, absoluteCol,
	)

	return drawMarker(c.window, windowStart, markerPos, relativeCol, msg)
}
