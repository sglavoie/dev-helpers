package run

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/buildcmd"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/inputs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func DailyBackup() {
	c := buildcmd.AsStringDaily()
	printCommandToRunWithConfirmation(c)
	execCmd(c)
}

func WeeklyBackup() {
	c := buildcmd.AsStringWeekly()
	printCommandToRunWithConfirmation(c)
	execCmd(c)
}

func MonthlyBackup() {
	c := buildcmd.AsStringMonthly()
	printCommandToRunWithConfirmation(c)
	execCmd(c)
}

func execCmd(cmdToRun string) {
	cmd := exec.Command("bash", "-c", cmdToRun)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()

	if err != nil {
		cobra.CheckErr(err)
	}
}

func printCommandToRunWithConfirmation(c string) {
	fmt.Println("The following command will be executed:", "\n", c)

	if viper.GetBool("confirmExec") {
		confirms := inputs.AskYesNoQuestion("\nDo you wish to proceed?")
		if !confirms {
			os.Exit(0)
		}
	}
}
