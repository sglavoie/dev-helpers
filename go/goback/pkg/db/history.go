package db

import (
	"log"
	"time"
)

// HistoryEntry is one row of the backups table.
type HistoryEntry struct {
	CreatedAt     time.Time
	BackupType    string
	ExecutionTime string
	Command       string
	Profile       string
	ExitCode      int
}

// MirrorProfile is the profile recorded for a mirror. A mirror is one global
// operation described by the top-level mirror configuration, so it belongs to
// no profile.
const MirrorProfile = "global"

// CompanionBackupType returns the backup_type recorded for a companion.
func CompanionBackupType(id string) string {
	return "companion/" + id
}

// RecordBackup appends an entry to the backup history. A history write is
// never worth failing a backup over, so a failure is only logged.
func RecordBackup(entry HistoryEntry) {
	if err := recordBackup(entry); err != nil {
		log.Printf("warning: failed to record backup in history; history unavailable for %s, profile %q (exit code %d): %v", entry.BackupType, entry.Profile, entry.ExitCode, err)
	}
}

func recordBackup(entry HistoryEntry) (err error) {
	sqldb, err := open()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sqldb.Close(); err == nil {
			err = closeErr
		}
	}()
	_, err = sqldb.Exec(
		"INSERT INTO backups VALUES(NULL,?,?,?,?,?,?);",
		entry.CreatedAt.Format("2006-01-02 15:04:05"),
		entry.BackupType,
		entry.ExecutionTime,
		entry.Command,
		entry.Profile,
		entry.ExitCode,
	)
	return err
}
