package filters

import (
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/gotime/internal/models"
)

func TestFilter_Keywords(t *testing.T) {
	entries := []models.Entry{
		{
			ID:        "1",
			Keyword:   "coding",
			StartTime: time.Now(),
			Tags:      []string{"go", "backend"},
		},
		{
			ID:        "2",
			Keyword:   "meeting",
			StartTime: time.Now(),
			Tags:      []string{"standup", "team"},
		},
		{
			ID:        "3",
			Keyword:   "documentation",
			StartTime: time.Now(),
			Tags:      []string{"writing", "docs"},
		},
	}

	tests := []struct {
		name             string
		keywords         []string
		excludeKeywords  bool
		expectedIDs      []string
		description      string
	}{
		{
			name:             "normal keyword filter - single keyword",
			keywords:         []string{"coding"},
			excludeKeywords:  false,
			expectedIDs:      []string{"1"},
			description:      "should return only entries matching the keyword",
		},
		{
			name:             "normal keyword filter - multiple keywords",
			keywords:         []string{"coding", "meeting"},
			excludeKeywords:  false,
			expectedIDs:      []string{"1", "2"},
			description:      "should return entries matching any of the keywords",
		},
		{
			name:             "exclude keyword filter - single keyword",
			keywords:         []string{"meeting"},
			excludeKeywords:  true,
			expectedIDs:      []string{"1", "3"},
			description:      "should return entries NOT matching the keyword",
		},
		{
			name:             "exclude keyword filter - multiple keywords",
			keywords:         []string{"coding", "meeting"},
			excludeKeywords:  true,
			expectedIDs:      []string{"3"},
			description:      "should return entries NOT matching any of the keywords",
		},
		{
			name:             "no keywords filter",
			keywords:         []string{},
			excludeKeywords:  false,
			expectedIDs:      []string{"1", "2", "3"},
			description:      "should return all entries when no keywords specified",
		},
		{
			name:             "no keywords with exclude flag",
			keywords:         []string{},
			excludeKeywords:  true,
			expectedIDs:      []string{"1", "2", "3"},
			description:      "should return all entries when no keywords to exclude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &Filter{
				Keywords:        tt.keywords,
				ExcludeKeywords: tt.excludeKeywords,
				TimeRange:       TimeRangeWeek, // Set a default time range
			}

			filtered := filter.Apply(entries)

			if len(filtered) != len(tt.expectedIDs) {
				t.Errorf("Expected %d entries, got %d. %s", len(tt.expectedIDs), len(filtered), tt.description)
			}

			for i, entry := range filtered {
				if i >= len(tt.expectedIDs) {
					break
				}
				if entry.ID != tt.expectedIDs[i] {
					t.Errorf("Expected entry ID %s at position %d, got %s", tt.expectedIDs[i], i, entry.ID)
				}
			}
		})
	}
}

func TestFilter_Tags(t *testing.T) {
	entries := []models.Entry{
		{
			ID:        "1",
			Keyword:   "coding",
			StartTime: time.Now(),
			Tags:      []string{"go", "backend"},
		},
		{
			ID:        "2",
			Keyword:   "meeting",
			StartTime: time.Now(),
			Tags:      []string{"standup", "team"},
		},
		{
			ID:        "3",
			Keyword:   "documentation",
			StartTime: time.Now(),
			Tags:      []string{"writing", "docs"},
		},
		{
			ID:        "4",
			Keyword:   "coding",
			StartTime: time.Now(),
			Tags:      []string{"go", "frontend"},
		},
	}

	tests := []struct {
		name         string
		tags         []string
		excludeTags  bool
		expectedIDs  []string
		description  string
	}{
		{
			name:         "normal tag filter - single tag",
			tags:         []string{"go"},
			excludeTags:  false,
			expectedIDs:  []string{"1", "4"},
			description:  "should return entries with the specified tag",
		},
		{
			name:         "normal tag filter - multiple tags",
			tags:         []string{"team", "docs"},
			excludeTags:  false,
			expectedIDs:  []string{"2", "3"},
			description:  "should return entries with any of the specified tags",
		},
		{
			name:         "exclude tag filter - single tag",
			tags:         []string{"go"},
			excludeTags:  true,
			expectedIDs:  []string{"2", "3"},
			description:  "should return entries NOT having the specified tag",
		},
		{
			name:         "exclude tag filter - multiple tags",
			tags:         []string{"team", "docs"},
			excludeTags:  true,
			expectedIDs:  []string{"1", "4"},
			description:  "should return entries NOT having any of the specified tags",
		},
		{
			name:         "no tags filter",
			tags:         []string{},
			excludeTags:  false,
			expectedIDs:  []string{"1", "2", "3", "4"},
			description:  "should return all entries when no tags specified",
		},
		{
			name:         "no tags with exclude flag",
			tags:         []string{},
			excludeTags:  true,
			expectedIDs:  []string{"1", "2", "3", "4"},
			description:  "should return all entries when no tags to exclude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &Filter{
				Tags:        tt.tags,
				ExcludeTags: tt.excludeTags,
				TimeRange:   TimeRangeWeek, // Set a default time range
			}

			filtered := filter.Apply(entries)

			if len(filtered) != len(tt.expectedIDs) {
				t.Errorf("Expected %d entries, got %d. %s", len(tt.expectedIDs), len(filtered), tt.description)
				t.Logf("Expected IDs: %v", tt.expectedIDs)
				actualIDs := make([]string, len(filtered))
				for i, entry := range filtered {
					actualIDs[i] = entry.ID
				}
				t.Logf("Actual IDs: %v", actualIDs)
			}

			for i, entry := range filtered {
				if i >= len(tt.expectedIDs) {
					break
				}
				if entry.ID != tt.expectedIDs[i] {
					t.Errorf("Expected entry ID %s at position %d, got %s", tt.expectedIDs[i], i, entry.ID)
				}
			}
		})
	}
}

func TestFilter_CombinedKeywordAndTags(t *testing.T) {
	entries := []models.Entry{
		{
			ID:        "1",
			Keyword:   "coding",
			StartTime: time.Now(),
			Tags:      []string{"go", "backend"},
		},
		{
			ID:        "2",
			Keyword:   "coding",
			StartTime: time.Now(),
			Tags:      []string{"javascript", "frontend"},
		},
		{
			ID:        "3",
			Keyword:   "meeting",
			StartTime: time.Now(),
			Tags:      []string{"go", "team"},
		},
	}

	tests := []struct {
		name             string
		keywords         []string
		excludeKeywords  bool
		tags             []string
		excludeTags      bool
		expectedIDs      []string
		description      string
	}{
		{
			name:             "keywords and tags both normal",
			keywords:         []string{"coding"},
			excludeKeywords:  false,
			tags:             []string{"go"},
			excludeTags:      false,
			expectedIDs:      []string{"1"},
			description:      "should return entries matching both keywords and tags",
		},
		{
			name:             "keywords normal, tags excluded",
			keywords:         []string{"coding"},
			excludeKeywords:  false,
			tags:             []string{"go"},
			excludeTags:      true,
			expectedIDs:      []string{"2"},
			description:      "should return entries matching keywords but not having the tags",
		},
		{
			name:             "keywords excluded, tags normal",
			keywords:         []string{"meeting"},
			excludeKeywords:  true,
			tags:             []string{"go"},
			excludeTags:      false,
			expectedIDs:      []string{"1"},
			description:      "should return entries not matching keywords but having the tags",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &Filter{
				Keywords:        tt.keywords,
				ExcludeKeywords: tt.excludeKeywords,
				Tags:            tt.tags,
				ExcludeTags:     tt.excludeTags,
				TimeRange:       TimeRangeWeek,
			}

			filtered := filter.Apply(entries)

			if len(filtered) != len(tt.expectedIDs) {
				t.Errorf("Expected %d entries, got %d. %s", len(tt.expectedIDs), len(filtered), tt.description)
			}

			for i, entry := range filtered {
				if i >= len(tt.expectedIDs) {
					break
				}
				if entry.ID != tt.expectedIDs[i] {
					t.Errorf("Expected entry ID %s at position %d, got %s", tt.expectedIDs[i], i, entry.ID)
				}
			}
		})
	}
}