package models

// Command represents a single command configuration
type Command struct {
	Command     string   `json:"command"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Config struct {
	Commands map[string]Command `json:"commands"`
}
