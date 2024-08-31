package buildcmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// builder is a struct that implements the RsyncBuilder interface.
type builder struct {
	sb             *strings.Builder
	updatedSrc     string
	updatedDestDir string
	builderType    string
}

// RsyncBuilderDaily is a struct that implements the builder interface for daily backups.
type RsyncBuilderDaily struct {
	builder
}

// RsyncBuilderWeekly is a struct that implements the builder interface for weekly backups.
type RsyncBuilderWeekly struct {
	builder
}

// RsyncBuilderMonthly is a struct that implements the builder interface for monthly backups.
type RsyncBuilderMonthly struct {
	builder
}

func (r *builder) BuildNoCheck() {
	r.build()
}

func (r *builder) BuildCheck() {
	r.build()
	r.validate()
}

func (r *builder) build() {
	r.initBuilder()
	r.appendBooleanFlags()
	r.appendLogFile()
	r.appendExcludedPatterns()
	r.appendSrcDest()
}

func (r *builder) validate() {
	r.validateBeforeRun()
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

func (r *builder) appendLogFile() {
	r.sb.WriteString(" --log-file=")
	r.sb.WriteString(strings.TrimSuffix(viper.ConfigFileUsed(), ".json"))
	r.sb.WriteString("_")
	r.sb.WriteString(r.builderType)
	r.sb.WriteString("_")
	r.sb.WriteString(time.Now().Format("20060102_15_04_05"))
}

func (r *builder) appendSrcDest() {
	r.sb.WriteString(" ")
	r.sb.WriteString(r.updatedSrc)
	r.sb.WriteString(" ")
	r.sb.WriteString(r.updatedDestDir)
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

func (r *builder) builderSettingsPrefix() string {
	return "rsync." + r.builderType + "."
}

func (r *builder) initBuilder() {
	r.sb = &strings.Builder{}
	r.sb.WriteString("rsync")
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

func (r *builder) validateBeforeRun() {
	if _, err := os.Stat(r.updatedDestDir); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("r.updatedDestination directory %s does not exist", r.updatedDestDir))
	}

	if _, err := os.Stat(r.updatedSrc); os.IsNotExist(err) {
		log.Fatal(fmt.Sprintf("source directory %s does not exist", r.updatedSrc))
	}

	if _, err := os.ReadDir(r.updatedSrc); err != nil {
		log.Fatal(fmt.Sprintf("source directory %s is empty", r.updatedSrc))
	}

	if r.updatedSrc == r.updatedDestDir {
		log.Fatal(fmt.Sprintf("source and r.updatedDestination are the same: %s", r.updatedSrc))
	}

	if strings.HasPrefix(r.updatedDestDir, r.updatedSrc) {
		log.Fatal(fmt.Sprintf("source directory %s is a parent of r.updatedDestination directory %s", r.updatedSrc, r.updatedDestDir))
	}
}

func (r *builder) appendBooleanFlags() {
	cfgPrefix := r.builderSettingsPrefix()
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

func (r *builder) appendExcludedPatterns() {
	cfgPrefix := r.builderSettingsPrefix()
	for _, pattern := range viper.GetStringSlice(cfgPrefix + "excludedPatterns") {
		r.sb.WriteString(" --exclude=\"")
		r.sb.WriteString(pattern)
		r.sb.WriteString("\"")
	}
}
