package models

type Alias struct {
	Name      string `json:"name"`
	CommandID string `json:"commandId"`
}

type Aliases map[string]string

// Command represents a single command configuration
type Command struct {
	Command     string   `json:"command"`
	Id          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type Commands map[string]Command

type Group struct {
	Name        string   `json:"name"`
	CommandIDs  []string `json:"commandIds"`
	StopOnError bool     `json:"stopOnError"`
}

type Groups map[string]Group

type Config struct {
	Aliases  Aliases  `json:"aliases"`
	Commands Commands `json:"commands"`
	Groups   Groups   `json:"groups"`
	Settings Settings `json:"settings"`
}

type Settings struct {
	ConfirmBeforeRun bool   `json:"confirmBeforeRun"`
	Editor           string `json:"editor"`
}
