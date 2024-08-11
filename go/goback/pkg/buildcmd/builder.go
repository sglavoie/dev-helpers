package buildcmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// builder is a struct that implements the rsyncCommonBuilder interface.
type builder struct {
	sb          *strings.Builder
	updatedSrc  string
	updatedDest string
}

func (r *builder) PrintString() {
	fmt.Println(r.sb.String())
}

func (r *builder) String() string {
	return r.sb.String()
}

func (r *builder) WrapLongLinesWithBackslashes() {
	chunks := splitIntoChunks(r.sb.String(), 80)
	r.sb.Reset()
	for i, chunk := range chunks {
		r.sb.WriteString(chunk)
		if i < len(chunks)-1 {
			r.sb.WriteString(" \\\n")
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

func (r *builder) appendLogFile() {
	r.sb.WriteString(" --log-file=")
	r.sb.WriteString(strings.TrimSuffix(viper.ConfigFileUsed(), ".json"))
	r.sb.WriteString("_" + time.Now().Format("20060102_15_04_05"))
}

func (r *builder) appendSrcDest() {
	r.sb.WriteString(" ")
	r.sb.WriteString(r.updatedSrc)
	r.sb.WriteString(" ")
	r.sb.WriteString(r.updatedDest)
}

// AddRawSourceDestination reads a source and a destination from the config file.
// It currently handles a single mapping for simplicity.
func (r *builder) addRawSourceDestination() {
	src, dest := mustExitOnInvalidSourceOrDestination()
	r.sb.WriteString(" ")
	r.sb.WriteString(src)
	r.sb.WriteString(" ")
	r.sb.WriteString(dest)
}

func (r *builder) setUpdatedSourceDestination(src, dest string) {
	r.updatedSrc = src
	r.updatedDest = dest
}

func (r *builder) validateBeforeRun() {
	if _, err := os.Stat(r.updatedDest); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("r.updatedDestination directory %s does not exist", r.updatedDest))
	}

	if _, err := os.Stat(r.updatedSrc); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("source directory %s does not exist", r.updatedSrc))
	}

	if _, err := os.ReadDir(r.updatedSrc); err != nil {
		log.Fatal(fmt.Sprintf("source directory %s is empty", r.updatedSrc))
	}

	if r.updatedSrc == r.updatedDest {
		log.Fatal(fmt.Sprintf("source and r.updatedDestination are the same: %s", r.updatedSrc))
	}

	if strings.HasPrefix(r.updatedDest, r.updatedSrc) {
		log.Fatal(fmt.Sprintf("source directory %s is a parent of r.updatedDestination directory %s", r.updatedSrc, r.updatedDest))
	}
}

// RsyncBuilderDaily is a struct that implements the builder interface for daily backups.
type RsyncBuilderDaily struct {
	builder
}

func (r *RsyncBuilderDaily) Build() {
	r.initBuilder()
	r.appendBooleanFlags()
	r.appendLogFile()
	r.appendExcludedPatterns()
	r.appendSrcDest()
	r.validateBeforeRun()
}

func (r *RsyncBuilderDaily) initBuilder() {
	r.sb = &strings.Builder{}
	r.sb.WriteString("rsync")
}

func (r *RsyncBuilderDaily) appendBooleanFlags() {
	r.setBooleanFlagsByBackupType("rsync.daily.")
}

func (r *RsyncBuilderDaily) appendExcludedPatterns() {
	r.setExcludedPatternsByBackupType("rsync.daily.")
}

// RsyncBuilderWeekly is a struct that implements the builder interface for weekly backups.
type RsyncBuilderWeekly struct {
	builder
}

func (r *RsyncBuilderWeekly) Build() {
	r.initBuilder()
	r.setBooleanFlags()
	r.appendLogFile()
	r.setExcludedPatterns()
	r.appendSrcDest()
	r.validateBeforeRun()
}

func (r *RsyncBuilderWeekly) initBuilder() {
	r.sb = &strings.Builder{}
	r.sb.WriteString("rsync")
}

func (r *RsyncBuilderWeekly) setBooleanFlags() {
	r.setBooleanFlagsByBackupType("rsync.weekly.")
}

func (r *RsyncBuilderWeekly) setExcludedPatterns() {
	r.setExcludedPatternsByBackupType("rsync.weekly.")
}

// RsyncBuilderMonthly is a struct that implements the builder interface for monthly backups.
type RsyncBuilderMonthly struct {
	builder
	destDir string
}

func (r *RsyncBuilderMonthly) initBuilder() {
	r.sb = &strings.Builder{}
}

func (r *RsyncBuilderMonthly) Build() {
	r.initBuilder()
	r.setDestDir()
	r.appendCompressionCommand()
	r.validateBeforeRun()
}

func (r *RsyncBuilderMonthly) appendCompressionCommand() {
	r.sb.WriteString(fmt.Sprintf("mkdir -p %s && tar -czvf %s %s", r.destDir, r.updatedDest, r.updatedSrc))
}

func (r *RsyncBuilderMonthly) setDestDir() {
	r.destDir = strings.Join(strings.Split(r.updatedDest, "/")[:len(strings.Split(r.updatedDest, "/"))-1], "/")
}

func (r *builder) setBooleanFlagsByBackupType(cfgPrefix string) {
	if viper.GetBool(cfgPrefix + "archive") {
		r.sb.WriteString(" --archive")
	}
	if viper.GetBool(cfgPrefix + "delete") {
		r.sb.WriteString(" --delete")
	}
	if viper.GetBool(cfgPrefix + "deleteExcluded") {
		r.sb.WriteString(" --delete-excluded")
	}
	if viper.GetBool(cfgPrefix + "dryRun") {
		r.sb.WriteString(" --dry-run")
	}
	if viper.GetBool(cfgPrefix + "force") {
		r.sb.WriteString(" --force")
	}
	if viper.GetBool(cfgPrefix + "hardLinks") {
		r.sb.WriteString(" --hard-links")
	}
	if viper.GetBool(cfgPrefix + "ignoreErrors") {
		r.sb.WriteString(" --ignore-errors")
	}
	if viper.GetBool(cfgPrefix + "pruneEmptyDirs") {
		r.sb.WriteString(" --prune-empty-dirs")
	}
	if viper.GetBool(cfgPrefix + "verbose") {
		r.sb.WriteString(" --verbose")
	}

	if viper.GetBool("showProgress") {
		r.sb.WriteString(" --progress")
	}
}

func (r *builder) setExcludedPatternsByBackupType(cfgPrefix string) {
	for _, pattern := range viper.GetStringSlice(cfgPrefix + "excludedPatterns") {
		r.sb.WriteString(" --exclude=\"")
		r.sb.WriteString(pattern)
		r.sb.WriteString("\"")
	}
}
