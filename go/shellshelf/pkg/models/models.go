package models

type Alias struct {
	CommandID string `json:"commandID"`
	Name      string `json:"name"`
}

// Command represents a single command configuration
type Command struct {
	Command     string   `json:"command"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Config struct {
	Aliases  map[string]string  `json:"aliases"`
	Commands map[string]Command `json:"commands"`
	Settings Settings           `json:"settings"`
}

type Settings struct {
	Editor string `json:"editor"`
}
