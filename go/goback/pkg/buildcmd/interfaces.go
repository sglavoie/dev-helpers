package buildcmd

import (
	"database/sql"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

// builder is a struct that implements the builder interface.
type builder struct {
	sb             *strings.Builder
	executionTime  string
	updatedDestDir string
	updatedSrc     string
	builderType    models.BackupTypes
	db             *sql.DB
}

// RsyncBuilderDaily is a struct that implements the builder interface for daily backups.
type RsyncBuilderDaily struct {
	builder
}

// RsyncBuilderWeekly is a struct that implements the builder interface for weekly backups.
type RsyncBuilderWeekly struct {
	builder
}

// RsyncBuilderMonthly is a struct that implements the builder interface for monthly backups.
type RsyncBuilderMonthly struct {
	builder
}
