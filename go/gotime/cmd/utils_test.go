package cmd

import (
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func createTestConfigForUtils() *models.Config {
	now := time.Now()
	return &models.Config{
		Entries: []models.Entry{
			{
				ID:        "entry1",
				ShortID:   1,
				Keyword:   "coding",
				Tags:      []string{"project1"},
				Duration:  3600,
				Active:    false,
				StartTime: now.Add(-24 * time.Hour),
			},
			{
				ID:        "entry2",
				ShortID:   2,
				Keyword:   "meeting",
				Tags:      []string{"work"},
				Duration:  1800,
				Active:    true,
				StartTime: now.Add(-12 * time.Hour),
			},
			{
				ID:        "entry3",
				ShortID:   3,
				Keyword:   "documentation",
				Tags:      []string{"writing"},
				Duration:  2400,
				Active:    false,
				StartTime: now.Add(-6 * time.Hour),
			},
		},
	}
}

func TestParseKeywordOrID(t *testing.T) {
	cfg := createTestConfigForUtils()

	t.Log("=== TESTING ParseKeywordOrID FUNCTION ===")

	// Test parsing as ID
	t.Log("Test 1: Parse valid ID")
	parsed, err := ParseKeywordOrID("2", cfg)
	if err != nil {
		t.Fatalf("Expected no error parsing '2' as ID, got: %v", err)
	}

	if parsed.Type != ArgumentTypeID {
		t.Errorf("Expected ArgumentTypeID, got %d", parsed.Type)
	}
	if parsed.ID != 2 {
		t.Errorf("Expected ID 2, got %d", parsed.ID)
	}
	if parsed.Entry.Keyword != "meeting" {
		t.Errorf("Expected entry keyword 'meeting', got '%s'", parsed.Entry.Keyword)
	}
	if parsed.Keyword != "meeting" {
		t.Errorf("Expected convenience keyword 'meeting', got '%s'", parsed.Keyword)
	}

	// Test parsing non-existent ID
	t.Log("Test 2: Parse non-existent ID")
	_, err = ParseKeywordOrID("99", cfg)
	if err == nil {
		t.Errorf("Expected error for non-existent ID 99")
	}
	expectedErr := "no entry found with short ID 99"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}

	// Test parsing as keyword
	t.Log("Test 3: Parse as keyword")
	parsed, err = ParseKeywordOrID("documentation", cfg)
	if err != nil {
		t.Fatalf("Expected no error parsing 'documentation' as keyword, got: %v", err)
	}

	if parsed.Type != ArgumentTypeKeyword {
		t.Errorf("Expected ArgumentTypeKeyword, got %d", parsed.Type)
	}
	if parsed.Keyword != "documentation" {
		t.Errorf("Expected keyword 'documentation', got '%s'", parsed.Keyword)
	}
	if parsed.Entry != nil {
		t.Errorf("Expected no entry for keyword parsing, got %v", parsed.Entry)
	}

	// Test parsing number-like keyword
	t.Log("Test 4: Parse number outside valid ID range as keyword")
	parsed, err = ParseKeywordOrID("2000", cfg)
	if err != nil {
		t.Fatalf("Expected no error parsing '2000' as keyword, got: %v", err)
	}

	if parsed.Type != ArgumentTypeKeyword {
		t.Errorf("Expected ArgumentTypeKeyword for '2000', got %d", parsed.Type)
	}
	if parsed.Keyword != "2000" {
		t.Errorf("Expected keyword '2000', got '%s'", parsed.Keyword)
	}

	// Test parsing zero as keyword
	t.Log("Test 5: Parse '0' as keyword")
	parsed, err = ParseKeywordOrID("0", cfg)
	if err != nil {
		t.Fatalf("Expected no error parsing '0' as keyword, got: %v", err)
	}

	if parsed.Type != ArgumentTypeKeyword {
		t.Errorf("Expected ArgumentTypeKeyword for '0', got %d", parsed.Type)
	}
	if parsed.Keyword != "0" {
		t.Errorf("Expected keyword '0', got '%s'", parsed.Keyword)
	}

	// Test empty argument
	t.Log("Test 6: Parse empty argument")
	_, err = ParseKeywordOrID("", cfg)
	if err == nil {
		t.Errorf("Expected error for empty argument")
	}
	expectedErr = "argument cannot be empty"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
	}

	t.Log("✅ ParseKeywordOrID tests completed successfully")
}
