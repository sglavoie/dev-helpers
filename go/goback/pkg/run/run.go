package run

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
)

func DailyBackup() error {
	c, err := buildcmd.BuildDaily()
	if err != nil {
		return err
	}
	if !c.PrintCommandToRunWithConfirmation() {
		return nil
	}
	return c.Execute()
}

func WeeklyBackup() error {
	c, err := buildcmd.BuildWeekly()
	if err != nil {
		return err
	}
	if !c.PrintCommandToRunWithConfirmation() {
		return nil
	}
	return c.Execute()
}

func MonthlyBackup() error {
	c, err := buildcmd.BuildMonthly()
	if err != nil {
		return err
	}
	if !c.PrintCommandToRunWithConfirmation() {
		return nil
	}
	return c.Execute()
}
