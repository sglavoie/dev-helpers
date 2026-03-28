package run

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
)

func DailyBackup() {
	c := buildcmd.BuildDaily()
	if !c.PrintCommandToRunWithConfirmation() {
		return
	}
	c.Execute()
}

func WeeklyBackup() {
	c := buildcmd.BuildWeekly()
	if !c.PrintCommandToRunWithConfirmation() {
		return
	}
	c.Execute()
}

func MonthlyBackup() {
	c := buildcmd.BuildMonthly()
	if !c.PrintCommandToRunWithConfirmation() {
		return
	}
	c.Execute()
}
