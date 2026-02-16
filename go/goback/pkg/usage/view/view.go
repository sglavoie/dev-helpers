package view

import (
	"database/sql"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

func View(e int, t models.BackupTypes) {
	var rows *sql.Rows
	profile := config.ProfileFlag

	if _, ok := t.(models.NoBackupType); ok {
		if profile != "" {
			rows = queryAllBackupTypesWithProfile(e, profile)
		} else {
			rows = queryAllBackupTypes(e)
		}
	} else {
		if profile != "" {
			rows = queryBackupTypeWithProfile(e, t, profile)
		} else {
			rows = queryBackupType(e, t)
		}
	}
	db.WithRows(rows, func(rows *sql.Rows) {
		SqlToText(rows)
	})
}

func queryAllBackupTypes(e int) *sql.Rows {
	return db.WithQuery("SELECT id, created_at, backup_type, execution_time, command, profile FROM backups ORDER BY created_at DESC LIMIT ?", e)
}

func queryAllBackupTypesWithProfile(e int, profile string) *sql.Rows {
	return db.WithQuery("SELECT id, created_at, backup_type, execution_time, command, profile FROM backups WHERE profile = ? ORDER BY created_at DESC LIMIT ?", profile, e)
}

func queryBackupType(e int, t models.BackupTypes) *sql.Rows {
	return db.WithQuery("SELECT id, created_at, backup_type, execution_time, command, profile FROM backups WHERE backup_type = ? ORDER BY created_at DESC LIMIT ?", t.String(), e)
}

func queryBackupTypeWithProfile(e int, t models.BackupTypes, profile string) *sql.Rows {
	return db.WithQuery("SELECT id, created_at, backup_type, execution_time, command, profile FROM backups WHERE backup_type = ? AND profile = ? ORDER BY created_at DESC LIMIT ?", t.String(), profile, e)
}
