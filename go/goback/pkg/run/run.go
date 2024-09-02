package run

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
)

func DailyBackup() {
	c := buildcmd.BuildDaily()
	c.PrintCommandToRunWithConfirmation()
	c.Execute()
}

func WeeklyBackup() {
	c := buildcmd.BuildWeekly()
	c.PrintCommandToRunWithConfirmation()
	c.Execute()
}

func MonthlyBackup() {
	c := buildcmd.BuildMonthly()
	c.PrintCommandToRunWithConfirmation()
	c.Execute()
}
