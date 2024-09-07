package models

// BackupTypes is an attempt at using enums in Go
type BackupTypes interface {
	String() string
}

type Daily struct{}

func (Daily) String() string {
	return "daily"
}

type Weekly struct{}

func (Weekly) String() string {
	return "weekly"
}

type Monthly struct{}

func (Monthly) String() string {
	return "monthly"
}

type NoBackupType struct{}

func (NoBackupType) String() string {
	return ""
}
