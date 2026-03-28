package buildcmd

import (
	"database/sql"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

// builder is a struct that implements the builder interface.
type builder struct {
	sb                 *strings.Builder
	executionTime      string
	updatedDestDir     string
	updatedSrc         string
	builderType        models.BackupTypes
	db                 *sql.DB
	hasIncludePatterns bool
	exitCode           int
}

// RsyncBuilder is a struct that implements the builder interface.
type RsyncBuilder struct {
	builder
}
