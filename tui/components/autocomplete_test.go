package components

import (
	"reflect"
	"testing"
)

func TestFilterSuggestions_EmptyInput(t *testing.T) {
	suggestions := []string{"status", "user_id", "created_at"}
	result := FilterSuggestions("", suggestions)

	if !reflect.DeepEqual(result, suggestions) {
		t.Errorf("expected all suggestions for empty input, got %v", result)
	}
}

func TestFilterSuggestions_EmptySuggestions(t *testing.T) {
	result := FilterSuggestions("test", []string{})

	if len(result) != 0 {
		t.Errorf("expected empty result for empty suggestions, got %v", result)
	}
}

func TestFilterSuggestions_PrefixMatch(t *testing.T) {
	suggestions := []string{"status", "user_id", "created_at", "updated_at"}
	result := FilterSuggestions("st", suggestions)

	expected := []string{"status"}
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("expected %v, got %v", expected, result)
	}
}

func TestFilterSuggestions_CaseInsensitive(t *testing.T) {
	suggestions := []string{"Status", "USER_ID", "created_at"}

	// Lowercase input matches uppercase suggestion
	result := FilterSuggestions("sta", suggestions)
	if len(result) != 1 || result[0] != "Status" {
		t.Errorf("expected [Status], got %v", result)
	}

	// Uppercase input matches lowercase suggestion
	result = FilterSuggestions("CRE", suggestions)
	if len(result) != 1 || result[0] != "created_at" {
		t.Errorf("expected [created_at], got %v", result)
	}
}

func TestFilterSuggestions_ContainsMatch(t *testing.T) {
	suggestions := []string{"status", "user_id", "created_at", "updated_at"}

	// "at" should match created_at, updated_at (contains), and status (contains "at")
	result := FilterSuggestions("at", suggestions)

	// status contains "at", created_at and updated_at end with "at"
	if len(result) != 3 {
		t.Errorf("expected 3 matches, got %v", result)
	}
}

func TestFilterSuggestions_PrefixBeforeContains(t *testing.T) {
	suggestions := []string{"user_status", "status", "status_code"}

	// "status" should return prefix matches first, then contains
	result := FilterSuggestions("status", suggestions)

	// status and status_code are prefix matches (should come first)
	// user_status is a contains match (should come after)
	if len(result) != 3 {
		t.Errorf("expected 3 matches, got %v", result)
	}

	// First two should be prefix matches
	if result[0] != "status" && result[0] != "status_code" {
		t.Errorf("expected prefix match first, got %v", result[0])
	}

	// user_status should be last (contains match)
	if result[2] != "user_status" {
		t.Errorf("expected user_status last, got %v", result[2])
	}
}

func TestFilterSuggestions_NoMatch(t *testing.T) {
	suggestions := []string{"status", "user_id", "created_at"}
	result := FilterSuggestions("xyz", suggestions)

	if len(result) != 0 {
		t.Errorf("expected no matches, got %v", result)
	}
}

func TestFilterSuggestions_ExactMatch(t *testing.T) {
	suggestions := []string{"status", "user_id", "created_at"}
	result := FilterSuggestions("status", suggestions)

	if len(result) != 1 || result[0] != "status" {
		t.Errorf("expected [status], got %v", result)
	}
}

func TestAutocomplete_GetSelectedSuggestion(t *testing.T) {
	ac := NewAutocomplete("test")
	ac.SetSuggestions([]string{"apple", "banana", "cherry"})

	// Initial selection is index 0
	if ac.GetSelectedSuggestion() != "apple" {
		t.Errorf("expected apple, got %s", ac.GetSelectedSuggestion())
	}

	// After filtering
	ac.SetValue("b")
	if ac.GetSelectedSuggestion() != "banana" {
		t.Errorf("expected banana after filtering, got %s", ac.GetSelectedSuggestion())
	}
}

func TestAutocomplete_GetSelectedSuggestion_Empty(t *testing.T) {
	ac := NewAutocomplete("test")

	// No suggestions
	if ac.GetSelectedSuggestion() != "" {
		t.Errorf("expected empty string, got %s", ac.GetSelectedSuggestion())
	}

	// With suggestions but filtered to empty
	ac.SetSuggestions([]string{"apple", "banana"})
	ac.SetValue("xyz")

	if ac.GetSelectedSuggestion() != "" {
		t.Errorf("expected empty string for no matches, got %s", ac.GetSelectedSuggestion())
	}
}

func TestAutocomplete_SetValue(t *testing.T) {
	ac := NewAutocomplete("test")
	ac.SetSuggestions([]string{"status", "user_id"})

	ac.SetValue("stat")
	if ac.Value() != "stat" {
		t.Errorf("expected stat, got %s", ac.Value())
	}

	// Filtered should update
	if len(ac.filtered) != 1 || ac.filtered[0] != "status" {
		t.Errorf("expected filtered to be [status], got %v", ac.filtered)
	}
}
