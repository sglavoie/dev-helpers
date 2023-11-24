package clihelpers

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func CountSetFlags(cmd *cobra.Command) (count int) {
	cmd.Flags().Visit(func(f *pflag.Flag) {
		count++
	})
	return count
}

func FatalExit(format string, v ...interface{}) {
	_, err := fmt.Fprintf(os.Stderr, format+"\n", v...)
	if err != nil {
		fmt.Println(err)
	}
	os.Exit(1)
}

func GetFlagBool(cmd *cobra.Command, flagName string) (bool, error) {
	value, err := cmd.Flags().GetBool(flagName)
	if err != nil {
		return false, fmt.Errorf("error retrieving %s: %v", flagName, err)
	}
	return value, nil
}

func GetFlagString(cmd *cobra.Command, flagName string) (string, error) {
	value, err := cmd.Flags().GetString(flagName)
	if err != nil {
		return "", fmt.Errorf("error retrieving %s: %v", flagName, err)
	}
	return value, nil
}

func GetFlagStringSlice(cmd *cobra.Command, flagName string) ([]string, error) {
	value, err := cmd.Flags().GetStringSlice(flagName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving %s: %v", flagName, err)
	}
	return value, nil
}

func GetLineSeparator() string {
	return "--------------------------------------------------------------------------------"
}

func GetSetFlags(cmd *cobra.Command) (flags []string) {
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags = append(flags, f.Name)
	})
	return flags
}

func IsInSlice(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func PrintLineSeparator() {
	fmt.Println(GetLineSeparator())
}

func ReadUserConfirmation() (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("y/N: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error reading input:", err)
		return false, err
	}

	input = strings.TrimSpace(input)

	if input != "y" && input != "Y" {
		return false, nil
	}
	return true, nil
}

func ShowHelpOnNoArgsNoFlagsAndExit(cmd *cobra.Command, args []string) {
	if len(args) == 0 && CountSetFlags(cmd) == 0 {
		err := cmd.Help()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		os.Exit(0)
	}
}

func WarnBeforeProceeding() (bool, error) {
	fmt.Println("Are you sure you want to proceed?")
	return ReadUserConfirmation()
}
