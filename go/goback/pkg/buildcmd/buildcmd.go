package buildcmd

import (
	"fmt"
	"github.com/spf13/viper"
	"strings"
	"time"
)

func getRsyncCommandBuilder() *strings.Builder {
	var sb strings.Builder
	sb.WriteString("rsync")
	addBooleanFlags(&sb)
	addLogFile(&sb)
	addExcludedPatterns(&sb)
	addSrcDest(&sb)
	return &sb
}

func getRsyncCommandString(sb *strings.Builder) string {
	return sb.String()
}

func PrintRsyncCommand() {
	sb := getRsyncCommandBuilder()
	wrapLongLinesWithBackslashes(sb)
	fmt.Println(getRsyncCommandString(sb))
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

func addExcludedPatterns(sb *strings.Builder) {
	for _, pattern := range viper.GetStringSlice("excludedPatterns") {
		sb.WriteString(" --exclude=\"")
		sb.WriteString(pattern)
		sb.WriteString("\"")
	}
}

// addSrcDest reads a map of source to destination directories from the config file.
// It currently handles a single mapping for simplicity.
func addSrcDest(sb *strings.Builder) {
	for src, dest := range viper.GetStringMapString("srcDest") {
		sb.WriteString(" ")
		sb.WriteString(src)
		sb.WriteString(" ")
		sb.WriteString(dest)
		break
	}
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
