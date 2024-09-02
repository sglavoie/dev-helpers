package buildcmd

import (
	"fmt"
	"os"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/inputs"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/printer"
	"github.com/spf13/viper"
)

func (r *builder) PrintCommandToRunWithConfirmation() {
	fmt.Println("The following command will be executed:", "\n", r.CommandString())

	if viper.GetBool("confirmExec") {
		confirms := inputs.AskYesNoQuestion("\nDo you wish to proceed?")
		if !confirms {
			os.Exit(0)
		}
	}
}
func (r *builder) PrintString() {
	fmt.Println(r.sb.String())
}

func (r *builder) WrapLongLinesWithBackslashes() {
	printer.WrapLongLinesWithBackslashes(r.sb, 80)
}
