package cleanlogs

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"sort"
	"strings"
)

// KeepLatestOf removes the oldest logs, keeping the n most recent ones for the given backup type.
func KeepLatestOf(n int, t string) {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	f, err := os.ReadDir(home)
	cobra.CheckErr(err)

	files := filterLogs(f, t)
	sortLogs(home, files)
	removeLogs(n, home, files, t)
}

// filterLogs filters the files to keep only the logs of the given backup type.
func filterLogs(files []os.DirEntry, t string) []string {
	prefixPattern := ".goback"
	var filtered []string
	prefixLength := len(prefixPattern)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		fn := file.Name()

		if len(fn) < prefixLength {
			continue
		}

		if fn[:7] == prefixPattern && !strings.Contains(fn[7:], ".") && strings.Contains(fn, t) {
			filtered = append(filtered, file.Name())
		}
	}

	return filtered
}

// removeLogs removes the oldest logs, keeping the n most recent ones for the given backup type.
func removeLogs(n int, home string, files []string, t string) {
	var toRemove []string
	if n < len(files) {
		toRemove = files[n:]
	} else {
		fmt.Printf("[%s] Nothing to remove: keeping %d\n", t, len(files))
		return
	}

	for _, file := range toRemove {
		fp := home + "/" + file
		fmt.Println("Removing", fp)
		err := os.Remove(fp)
		cobra.CheckErr(err)
	}
}

// sortLogs sorts the files by modification time in descending order.
func sortLogs(home string, files []string) {
	sort.Slice(files, func(i, j int) bool {
		fi, err := os.Stat(home + "/" + files[i])
		cobra.CheckErr(err)
		fj, err := os.Stat(home + "/" + files[j])
		cobra.CheckErr(err)

		return fi.ModTime().After(fj.ModTime())
	})
}
