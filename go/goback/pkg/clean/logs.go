package clean

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"sort"
	"strings"
)

// KeepLatest removes the oldest logs, keeping the n most recent ones.
func KeepLatest(n int) {
	if n < 0 {
		log.Fatal("Number of logs to keep must be a positive integer")
	}

	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	f, err := os.ReadDir(home)
	cobra.CheckErr(err)

	files := filterLogs(f)
	sortLogs(home, files)
	removeLogs(n, home, files)
}

// filterLogs filters the files to keep only the logs.
func filterLogs(files []os.DirEntry) []string {
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

		if fn[:7] == prefixPattern && !strings.Contains(fn[7:], ".") {
			filtered = append(filtered, file.Name())
		}
	}

	return filtered
}

// removeLogs removes the oldest logs, keeping the n most recent ones.
func removeLogs(n int, home string, files []string) {
	var toRemove []string
	if n < len(files) {
		toRemove = files[n:]
	} else {
		fmt.Println("Nothing to remove: keeping", len(files))
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
