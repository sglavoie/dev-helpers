package db

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordBackupWarnsOnDatabaseInitializationFailure(t *testing.T) {
	for _, kind := range []string{"directory", "corrupt database", "invalid schema"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			path := filepath.Join(home, ".goback.db")
			switch kind {
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "corrupt database":
				if err := os.WriteFile(path, []byte("not a database"), 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid schema":
				sqldb, err := open()
				if err != nil {
					t.Fatal(err)
				}
				if _, err = sqldb.Exec("DROP TABLE backups; CREATE VIEW backups AS SELECT 1 AS id"); err != nil {
					t.Fatal(err)
				}
				if err := sqldb.Close(); err != nil {
					t.Fatal(err)
				}
			}
			var out bytes.Buffer
			previous := log.Writer()
			log.SetOutput(&out)
			t.Cleanup(func() { log.SetOutput(previous) })
			RecordBackup(HistoryEntry{CreatedAt: time.Now(), BackupType: "daily", Profile: "test"})
			if !strings.Contains(out.String(), "history unavailable") {
				t.Fatalf("missing warning: %s", &out)
			}
		})
	}
}
