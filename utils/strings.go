package utils

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	numASCII = 127
)

// ToPythonicJSON formats a JSON string into pythonic format of JSON string.
func ToPythonicJSON(input string) (string, error) {
	var result strings.Builder
	result.Grow(len(input) + len(input)/5) // Estimate 20% growth

	insideQuotes := false // Whether the current character is inside quotes
	runes := []rune(input)

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		// determines if the current rune is within quotes
		if char == '"' && (i == 0 || runes[i-1] != '\\') {
			insideQuotes = !insideQuotes
			result.WriteRune(char)
			continue
		}

		// Add a space after colons and commas, but only if not inside quotes
		// because these characters can appear inside value strings.
		// We only want to modify the JSON structure, not the string values.
		if !insideQuotes && (char == ':' || char == ',') {
			result.WriteRune(char)
			result.WriteRune(' ')
			continue
		}

		// Handle escape sequences
		if char == '\\' && i+1 < len(runes) {
			nextChar := runes[i+1]
			if nextChar == 'u' && i+5 < len(runes) {
				unicodeSeq := string(runes[i : i+6])
				r, err := strconv.Unquote("\"" + unicodeSeq + "\"")
				if err != nil {
					return "", err
				}
				result.WriteString(r)
				i += 5
			} else {
				result.WriteRune(char)
				result.WriteRune(nextChar)
				i++
			}
		} else if char <= numASCII { // Just write normally if it's ASCII character
			result.WriteRune(char)
		} else {
			// For non-ASCII characters, convert to UTF-16 surrogate pairs
			utf16Chars := utf16.Encode([]rune{char})
			for _, c := range utf16Chars {
				result.WriteString(fmt.Sprintf("\\u%04x", c))
			}
		}
	}

	return result.String(), nil
}
