package clihelpers

import (
	"bufio"
	"fmt"
	"log"
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
		log.Fatal(err)
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

func GetSetFlags(cmd *cobra.Command) (flags []string) {
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags = append(flags, f.Name)
	})
	return flags
}

func PrintLineSeparator() {
	fmt.Println("--------------------------------------------------------------------------------")
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

func WarnBeforeProceeding() (bool, error) {
	fmt.Println("Are you sure you want to proceed?")
	return ReadUserConfirmation()
}
