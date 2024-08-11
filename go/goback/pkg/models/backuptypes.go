package models

// BackupTypes is an attempt at using enums in Go
type BackupTypes interface {
	isBackupType()
}

type Daily struct{}

func (Daily) isBackupType() {}

type Weekly struct{}

func (Weekly) isBackupType() {}

type Monthly struct{}

func (Monthly) isBackupType() {}
