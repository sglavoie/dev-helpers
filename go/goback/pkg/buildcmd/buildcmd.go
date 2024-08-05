package buildcmd

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
	"time"
)

func GetRsyncCommand() string {
	var sb strings.Builder
	sb.WriteString("rsync")
	addBooleanFlags(&sb)
	addLogFile(&sb)
	wrapLongLinesWithBackslashes(&sb)
	return sb.String()
}

func PrintRsyncCommand() {
	fmt.Println(GetRsyncCommand())
}

func addBooleanFlags(sb *strings.Builder) {
	if viper.GetBool("rsyncFlags.archive") {
		sb.WriteString(" --archive")
	}
	if viper.GetBool("rsyncFlags.delete") {
		sb.WriteString(" --delete")
	}
	if viper.GetBool("rsyncFlags.deleteExcluded") {
		sb.WriteString(" --delete-excluded")
	}
	if viper.GetBool("rsyncFlags.dryRun") {
		sb.WriteString(" --dry-run")
	}
	if viper.GetBool("rsyncFlags.force") {
		sb.WriteString(" --force")
	}
	if viper.GetBool("rsyncFlags.hardLinks") {
		sb.WriteString(" --hard-links")
	}
	if viper.GetBool("rsyncFlags.ignoreErrors") {
		sb.WriteString(" --ignore-errors")
	}
	if viper.GetBool("rsyncFlags.pruneEmptyDirs") {
		sb.WriteString(" --prune-empty-dirs")
	}
	if viper.GetBool("rsyncFlags.verbose") {
		sb.WriteString(" --verbose")
	}
}

func addLogFile(sb *strings.Builder) {
	sb.WriteString(" --log-file=")
	sb.WriteString(strings.TrimSuffix(viper.ConfigFileUsed(), ".json"))
	sb.WriteString("_" + time.Now().Format("20060102_15_04_05"))
}

func wrapLongLinesWithBackslashes(sb *strings.Builder) {
	chunks := splitIntoChunks(sb.String(), 80)
	sb.Reset()
	for i, chunk := range chunks {
		sb.WriteString(chunk)
		if i < len(chunks)-1 {
			sb.WriteString(" \\\n")
		}
	}
}

func splitIntoChunks(s string, chunkSize int) []string {
	var chunks []string
	for len(s) > chunkSize {
		i := strings.LastIndex(s[:chunkSize], " ")
		if i == -1 {
			i = chunkSize
		}
		chunks = append(chunks, s[:i])
		s = s[i:]
	}
	chunks = append(chunks, s)
	return chunks
}
