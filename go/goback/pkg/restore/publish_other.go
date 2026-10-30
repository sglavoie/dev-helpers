//go:build !darwin && !linux

package restore

import "fmt"

func publishDirectory(fromFD int, from string, toFD int, to string) error {
	return fmt.Errorf("directory restore requires exclusive rename support on macOS or Linux")
}
