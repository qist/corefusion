package coreconfig

import (
	"encoding/json"
	"strings"
)

// StripJSONC removes single-line (//) and multi-line (/* */) comments from JSONC.
func StripJSONC(data []byte) []byte {
	s := string(data)
	var result strings.Builder
	inString := false
	inSingleLineComment := false
	inMultiLineComment := false
	escaped := false

	for i := 0; i < len(s); i++ {
		ch := s[i]

		if inSingleLineComment {
			if ch == '\n' {
				inSingleLineComment = false
				result.WriteByte(ch)
			}
			continue
		}

		if inMultiLineComment {
			if ch == '*' && i+1 < len(s) && s[i+1] == '/' {
				inMultiLineComment = false
				i++
			}
			continue
		}

		if escaped {
			result.WriteByte(ch)
			escaped = false
			continue
		}

		if ch == '\\' && inString {
			result.WriteByte(ch)
			escaped = true
			continue
		}

		if ch == '"' {
			inString = !inString
			result.WriteByte(ch)
			continue
		}

		if inString {
			result.WriteByte(ch)
			continue
		}

		// Not in string, check for comments
		if ch == '/' && i+1 < len(s) {
			next := s[i+1]
			if next == '/' {
				inSingleLineComment = true
				i++
				continue
			}
			if next == '*' {
				inMultiLineComment = true
				i++
				continue
			}
		}

		result.WriteByte(ch)
	}

	return []byte(result.String())
}

// ParseJSONC parses JSONC (JSON with Comments) into a value.
func ParseJSONC(data []byte, v interface{}) error {
	stripped := StripJSONC(data)
	return json.Unmarshal(stripped, v)
}

// ValidJSONC checks if JSONC is valid.
func ValidJSONC(data []byte) bool {
	stripped := StripJSONC(data)
	return json.Valid(stripped)
}
