package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToPythonicJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Poop emoji as UTF-16 surrogate pair",
			input:    `{"message":"Hello 💩!"}`,
			expected: `{"message": "Hello \ud83d\udca9!"}`,
		},
		{
			name:     "Simple JSON object",
			input:    `{"name":"John","age":30}`,
			expected: `{"name": "John", "age": 30}`,
		},
		{
			name:     "Nested JSON object",
			input:    `{"person":{"name":"Alice","age":25},"city":"New York"}`,
			expected: `{"person": {"name": "Alice", "age": 25}, "city": "New York"}`,
		},
		{
			name:     "JSON array",
			input:    `["apple","banana","cherry"]`,
			expected: `["apple", "banana", "cherry"]`,
		},
		{
			name:     "Mixed types",
			input:    `{"name":"Bob","age":40,"hobbies":["reading","cycling"],"married":true}`,
			expected: `{"name": "Bob", "age": 40, "hobbies": ["reading", "cycling"], "married": true}`,
		},
		{
			name:     "Unicode characters",
			input:    `{"greeting":"こんにちは","city":"東京"}`,
			expected: `{"greeting": "\u3053\u3093\u306b\u3061\u306f", "city": "\u6771\u4eac"}`,
		},
		{
			name:     "Escaped characters",
			input:    `{"message":"Line 1\nLine 2\tTabbed"}`,
			expected: `{"message": "Line 1\nLine 2\tTabbed"}`,
		},
		{
			name:     "Empty object and array",
			input:    `{"empty_obj":{},"empty_arr":[]}`,
			expected: `{"empty_obj": {}, "empty_arr": []}`,
		},
		{
			name:     "Special characters in strings",
			input:    `{"special":"!@#$%^&*()_+{}[]|\\:;<>,.?/"}`,
			expected: `{"special": "!@#$%^&*()_+{}[]|\\:;<>,.?/"}`,
		},
		{
			name:     "ASCII characters",
			input:    `{"ascii":"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!\"#$%&'()*+,-./:;<=>?@[\\]^_` + "`" + `{|}~"}`,
			expected: `{"ascii": "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!\"#$%&'()*+,-./:;<=>?@[\\]^_` + "`" + `{|}~"}`,
		},
		{
			name:     "Semicolon with space in between quotes",
			input:    `{"key":"value : value"}`,
			expected: `{"key": "value : value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ToPythonicJSON(tt.input)
			require.NoError(t, err)

			// Check if the result is valid JSON
			var jsonObj interface{}
			err = json.Unmarshal([]byte(result), &jsonObj)
			require.NoError(t, err, "Result should be valid JSON")

			assert.Equal(t, tt.expected, result)
		})
	}
}
