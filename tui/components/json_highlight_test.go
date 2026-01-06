package components

import (
	"strings"
	"testing"
)

func TestHighlightJSON_Basic(t *testing.T) {
	input := `{
  "name": "test",
  "count": 42,
  "active": true,
  "data": null
}`

	result := HighlightJSON(input)

	// Should contain the original values (though colored)
	if !strings.Contains(result, "name") {
		t.Error("missing key 'name'")
	}
	if !strings.Contains(result, "test") {
		t.Error("missing string value 'test'")
	}
	if !strings.Contains(result, "42") {
		t.Error("missing number 42")
	}
	if !strings.Contains(result, "true") {
		t.Error("missing boolean true")
	}
	if !strings.Contains(result, "null") {
		t.Error("missing null")
	}
}

func TestHighlightJSON_NestedObjects(t *testing.T) {
	input := `{
  "user": {
    "id": 123,
    "profile": {
      "bio": "Hello"
    }
  }
}`

	result := HighlightJSON(input)

	if !strings.Contains(result, "user") {
		t.Error("missing nested key 'user'")
	}
	if !strings.Contains(result, "profile") {
		t.Error("missing nested key 'profile'")
	}
	if !strings.Contains(result, "123") {
		t.Error("missing nested number")
	}
}

func TestHighlightJSON_Arrays(t *testing.T) {
	input := `{
  "items": [
    "a",
    "b",
    123
  ]
}`

	result := HighlightJSON(input)

	if !strings.Contains(result, "items") {
		t.Error("missing key 'items'")
	}
	if !strings.Contains(result, "[") || !strings.Contains(result, "]") {
		t.Error("missing array brackets")
	}
}

func TestHighlightJSON_EscapedStrings(t *testing.T) {
	input := `{
  "message": "hello \"world\""
}`

	result := HighlightJSON(input)

	if !strings.Contains(result, "message") {
		t.Error("missing key")
	}
	// The escaped quotes should be preserved
	if !strings.Contains(result, `\"world\"`) {
		t.Error("escaped quotes not preserved")
	}
}

func TestFindStringEnd(t *testing.T) {
	tests := []struct {
		input    string
		start    int
		expected int
	}{
		{`"hello"`, 0, 7},
		{`"hello \"world\""`, 0, 17},
		{`  "test"`, 2, 8},
		{`""`, 0, 2},
	}

	for _, tc := range tests {
		result := findStringEnd(tc.input, tc.start)
		if result != tc.expected {
			t.Errorf("findStringEnd(%q, %d) = %d, want %d", tc.input, tc.start, result, tc.expected)
		}
	}
}
