package clihelpers

import (
	"fmt"
	"github.com/spf13/cobra"
)

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
