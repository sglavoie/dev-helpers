package run

import (
	"bytes"
	"fmt"
	"github.com/spf13/viper"
	"goback/pkg/buildcmd"
	"goback/pkg/inputs"
	"os"
	"os/exec"
)

func ExecDailyBackup() {
	execCmd(getAndPrintCommandToRunWithConfirmation("daily"))
}

func ExecWeeklyBackup() {
	execCmd(getAndPrintCommandToRunWithConfirmation("weekly"))
}

func ExecMonthlyBackup() {
	execCmd(getAndPrintCommandToRunWithConfirmation("monthly"))
}

func execCmd(cmdToRun string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("bash", "-c", cmdToRun)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		fmt.Println("Error running command:")
		fmt.Println(err)

		fmt.Println("stderr:")
		fmt.Println(stderr.String())
		os.Exit(1)
	}

	fmt.Println(stdout.String())
}

func getAndPrintCommandToRunWithConfirmation(backupType string) string {
	cmdToRun := buildcmd.GetRsyncCommandToRun(backupType)
	fmt.Println("The following command will be executed:", "\n", cmdToRun)

	if viper.GetBool("confirmExec") {
		confirms := inputs.AskYesNoQuestion("\nDo you wish to proceed?")
		if !confirms {
			os.Exit(0)
		}
	}

	return cmdToRun
}
