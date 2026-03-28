package buildcmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func (r *builder) BuildNoCheck() {
	r.build()
}

func (r *builder) BuildCheck() {
	r.build()
	r.validateBeforeRun()
}

func (r *builder) Execute() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := exec.CommandContext(ctx, "bash", "-c", r.CommandString())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	r.executionTime = time.Since(start).String()

	if ctx.Err() != nil {
		fmt.Println("\nBackup interrupted, cleaning up...")
		return
	}

	if err != nil {
		fmt.Println("Error running rsync command: ", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			r.exitCode = exitErr.ExitCode()
		} else {
			r.exitCode = 1
		}
	}
	if !viper.GetBool("cliDryRun") {
		r.updateDBWithUsage()
	}
}

func (r *builder) build() {
	r.initBuilder()
	r.appendBooleanFlags()
	// r.appendLogFile()
	r.appendIncludedPatterns()
	r.appendExcludedPatterns()
	r.appendSrcDest()
}

func (r *builder) builderSettingsPrefix() string {
	return config.ActiveProfilePrefix() + "rsync." + r.builderType.String() + "."
}

func (r *builder) initBuilder() {
	r.sb = &strings.Builder{}
	r.sb.WriteString("rsync")
}

func (r *builder) insertIntoDb() {
	createdAt := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.Exec("INSERT INTO backups VALUES(NULL,?,?,?,?,?,?);", createdAt, r.builderType.String(), r.executionTime, r.CommandString(), config.ActiveProfileName, r.exitCode)
	cobra.CheckErr(err)
}

func (r *builder) updateDBWithUsage() {
	db.CreateDatabaseFileIfNotExists()

	db.WithDb(func(sqldb *sql.DB) {
		db.CreateTableIfNotExists(sqldb)
		db.MigrateProfileColumn(sqldb)
		db.MigrateExitCodeColumn(sqldb)
		r.db = sqldb
		r.insertIntoDb()
	})
}
