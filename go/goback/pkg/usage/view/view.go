package view

import (
	"database/sql"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

func View(e int, t models.BackupTypes) {
	profile := config.ProfileFlag
	callback := func(rows *sql.Rows) { SqlToText(rows) }

	if _, ok := t.(models.NoBackupType); ok {
		if profile != "" {
			queryAllBackupTypesWithProfile(e, profile, callback)
		} else {
			queryAllBackupTypes(e, callback)
		}
	} else {
		if profile != "" {
			queryBackupTypeWithProfile(e, t, profile, callback)
		} else {
			queryBackupType(e, t, callback)
		}
	}
}

func queryAllBackupTypes(e int, callback func(*sql.Rows)) {
	db.QueryRows("SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM backups ORDER BY created_at DESC LIMIT ?", callback, e)
}

func queryAllBackupTypesWithProfile(e int, profile string, callback func(*sql.Rows)) {
	db.QueryRows("SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM backups WHERE profile = ? ORDER BY created_at DESC LIMIT ?", callback, profile, e)
}

func queryBackupType(e int, t models.BackupTypes, callback func(*sql.Rows)) {
	db.QueryRows("SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM backups WHERE backup_type = ? ORDER BY created_at DESC LIMIT ?", callback, t.String(), e)
}

func queryBackupTypeWithProfile(e int, t models.BackupTypes, profile string, callback func(*sql.Rows)) {
	db.QueryRows("SELECT id, created_at, backup_type, execution_time, command, profile, exit_code FROM backups WHERE backup_type = ? AND profile = ? ORDER BY created_at DESC LIMIT ?", callback, t.String(), profile, e)
}
