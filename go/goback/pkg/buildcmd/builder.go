package buildcmd

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/db"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/eject"
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
	cmd := exec.Command("bash", "-c", r.CommandString())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	start := time.Now()
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error running rsync command: ", err)
	}
	r.executionTime = time.Since(start).String()
	r.updateDBWithUsage()

	if ejectCfg := viper.GetBool("ejectOnExit"); ejectCfg {
		eject.Eject()
	}
}

func (r *builder) build() {
	r.initBuilder()
	r.appendBooleanFlags()
	r.appendLogFile()
	r.appendExcludedPatterns()
	r.appendSrcDest()
}

func (r *builder) builderSettingsPrefix() string {
	return "rsync." + r.builderType.String() + "."
}

func (r *builder) initBuilder() {
	r.sb = &strings.Builder{}
	r.sb.WriteString("rsync")
}

func (r *builder) insertIntoDb() {
	createdAt := time.Now().Format("2006-01-02 15:04:05")
	_, err := r.db.Exec("INSERT INTO backups VALUES(NULL,?,?,?,?);", createdAt, r.builderType.String(), r.executionTime, r.CommandString())
	cobra.CheckErr(err)
}

func (r *builder) updateDBWithUsage() {
	db.CreateDatabaseFileIfNotExists()

	db.WithDb(func(sqldb *sql.DB) {
		db.CreateTableIfNotExists(sqldb)
		r.db = sqldb
		r.insertIntoDb()
	})
}
