package components

import (
	"regexp"
	"strings"
)

var (
	// Matches "key": at start of line (with optional leading whitespace)
	keyPattern = regexp.MustCompile(`^(\s*)"([^"]+)"(:)`)
	// Matches string values (after colon or in array)
	stringPattern = regexp.MustCompile(`"([^"\\]|\\.)*"`)
	// Matches numbers
	numberPattern = regexp.MustCompile(`\b-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?\b`)
	// Matches booleans
	boolPattern = regexp.MustCompile(`\b(true|false)\b`)
	// Matches null
	nullPattern = regexp.MustCompile(`\bnull\b`)
)

// HighlightJSON applies syntax highlighting to pretty-printed JSON
func HighlightJSON(json string) string {
	lines := strings.Split(json, "\n")
	var result []string

	for _, line := range lines {
		result = append(result, highlightLine(line))
	}

	return strings.Join(result, "\n")
}

func highlightLine(line string) string {
	// Handle empty lines
	if strings.TrimSpace(line) == "" {
		return line
	}

	// First, try to match a key at the start
	keyMatch := keyPattern.FindStringSubmatchIndex(line)
	if keyMatch != nil {
		// keyMatch indices: [full_start, full_end, ws_start, ws_end, key_start, key_end, colon_start, colon_end]
		indent := line[keyMatch[2]:keyMatch[3]]
		key := line[keyMatch[4]:keyMatch[5]]
		rest := line[keyMatch[1]:]

		// Color the key (including quotes) and colon
		coloredKey := JSONKey.Render(`"` + key + `"`)
		coloredColon := JSONBracket.Render(":")

		// Highlight the rest of the line (the value part)
		coloredRest := highlightValue(rest)

		return indent + coloredKey + coloredColon + coloredRest
	}

	// No key found - this is a value line (array element or closing bracket)
	return highlightValue(line)
}

func highlightValue(s string) string {
	// Process character by character to handle overlapping patterns correctly
	result := &strings.Builder{}
	i := 0

	for i < len(s) {
		// Check for string
		if s[i] == '"' {
			end := findStringEnd(s, i)
			if end > i {
				result.WriteString(JSONString.Render(s[i:end]))
				i = end
				continue
			}
		}

		// Check for brackets
		if s[i] == '{' || s[i] == '}' || s[i] == '[' || s[i] == ']' {
			result.WriteString(JSONBracket.Render(string(s[i])))
			i++
			continue
		}

		// Check for true/false
		if i+4 <= len(s) && s[i:i+4] == "true" {
			result.WriteString(JSONBool.Render("true"))
			i += 4
			continue
		}
		if i+5 <= len(s) && s[i:i+5] == "false" {
			result.WriteString(JSONBool.Render("false"))
			i += 5
			continue
		}

		// Check for null
		if i+4 <= len(s) && s[i:i+4] == "null" {
			result.WriteString(JSONNull.Render("null"))
			i += 4
			continue
		}

		// Check for number
		if numMatch := numberPattern.FindStringIndex(s[i:]); numMatch != nil && numMatch[0] == 0 {
			result.WriteString(JSONNumber.Render(s[i : i+numMatch[1]]))
			i += numMatch[1]
			continue
		}

		// Default: keep character as-is (whitespace, commas, etc.)
		result.WriteByte(s[i])
		i++
	}

	return result.String()
}

// findStringEnd finds the end of a JSON string starting at position start
func findStringEnd(s string, start int) int {
	if start >= len(s) || s[start] != '"' {
		return start
	}

	i := start + 1
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i += 2 // skip escaped char
			continue
		}
		if s[i] == '"' {
			return i + 1
		}
		i++
	}
	return start // unclosed string, return original position
}
