package cmd

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// formatTagsWithColors applies color coding to tags based on their characteristics
func formatTagsWithColors(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}

	var coloredTags []string
	for _, tag := range tags {
		coloredTag := colorizeTag(tag)
		coloredTags = append(coloredTags, coloredTag)
	}

	return strings.Join(coloredTags, ", ")
}

// colorizeTag applies color to a single tag based on its content/category
func colorizeTag(tag string) string {
	// Define color schemes for different tag categories
	var tagStyle lipgloss.Style

	// Technical/programming tags
	if isMatchingTag(tag, []string{"go", "golang", "rust", "python", "javascript", "typescript", "java", "cpp", "c++", "c#", "php", "ruby", "swift", "kotlin"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("75")) // Light blue
	} else if isMatchingTag(tag, []string{"cli", "api", "web", "frontend", "backend", "database", "db", "sql", "mongodb", "redis"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("141")) // Purple
	} else if isMatchingTag(tag, []string{"meeting", "call", "standup", "retro", "planning", "review"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("208")) // Orange
	} else if isMatchingTag(tag, []string{"bug", "fix", "debug", "debugging", "hotfix", "patch"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // Red
	} else if isMatchingTag(tag, []string{"feature", "new", "enhancement", "improvement", "upgrade"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82")) // Green
	} else if isMatchingTag(tag, []string{"test", "testing", "qa", "unit", "integration", "e2e"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("226")) // Yellow
	} else if isMatchingTag(tag, []string{"docs", "documentation", "readme", "wiki", "guide"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117")) // Light cyan
	} else if isMatchingTag(tag, []string{"refactor", "cleanup", "maintenance", "tech-debt", "optimization"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("177")) // Light purple
	} else if isMatchingTag(tag, []string{"urgent", "critical", "high-priority", "important", "asap"}) {
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true) // Bold red
	} else {
		// Default tag color
		tagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")) // Light gray
	}

	return tagStyle.Render(tag)
}

// isMatchingTag checks if a tag matches any in a list of keywords (case-insensitive)
func isMatchingTag(tag string, keywords []string) bool {
	tagLower := strings.ToLower(tag)
	for _, keyword := range keywords {
		if strings.Contains(tagLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

// formatDurationWithWarning formats duration with visual warnings for long durations
func formatDurationWithWarning(durationSeconds int, isActive bool) string {
	durationStr := formatDuration(durationSeconds)

	if isActive {
		if durationSeconds > 28800 { // > 8 hours
			warningStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")). // Red
				Bold(true)
			return warningStyle.Render(durationStr + " ⚠️")
		} else if durationSeconds > 14400 { // > 4 hours
			cautionStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")) // Orange
			return cautionStyle.Render(durationStr + " 🔶")
		} else {
			// Normal active duration with progress indicator
			activeStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")) // Green
			return activeStyle.Render(durationStr + " ▶")
		}
	} else {
		// Stopped entries - highlight long durations
		if durationSeconds > 28800 { // > 8 hours
			warningStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("196")). // Red
				Bold(true)
			return warningStyle.Render(durationStr)
		} else if durationSeconds > 14400 { // > 4 hours
			cautionStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")) // Orange
			return cautionStyle.Render(durationStr)
		} else {
			return durationStr
		}
	}
}

// formatKeywordWithStyle formats keywords with consistent styling
func formatKeywordWithStyle(keyword string, isActive bool) string {
	if isActive {
		activeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("117")). // Light cyan
			Bold(true)
		return activeStyle.Render(keyword)
	} else {
		normalStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")) // White
		return normalStyle.Render(keyword)
	}
}
