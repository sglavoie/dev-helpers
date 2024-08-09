package buildcmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

func GetRsyncCommandToRun(backupType string) string {
	src, dest := exitOnInvalidSourceOrDestination()
	switch backupType {
	case "daily":
		dest = dest + "/daily"
	case "weekly":
		src = dest + "/daily/" // append slash to avoid copying the daily directory itself
		dest = dest + "/weekly"
	case "monthly":
		src = dest + "/daily"
		dest = dest + "/monthly/monthly_" + time.Now().Format("20060102") + ".tar.gz"
		return getCompressionCommand(src, dest)
	default:
		log.Fatal("invalid backup type")
	}
	sb := getRsyncCommandBuilder()
	addUpdatedSourceDestination(sb, src, dest)
	return getRsyncCommandString(sb)
}

func PrintRsyncCommand() {
	sb := getRsyncCommandBuilder()
	addRawSourceDestination(sb)
	wrapLongLinesWithBackslashes(sb)
	fmt.Println(getRsyncCommandString(sb))
}

func getCompressionCommand(src string, dest string) string {
	if _, err := os.Stat(dest); err == nil {
		log.Fatal("destination file already exists: " + dest)
	}

	destDir := strings.Join(strings.Split(dest, "/")[:len(strings.Split(dest, "/"))-1], "/")
	return "mkdir -p " + destDir + " && " + "tar -czvf " + dest + " " + src
}

func getRsyncCommandBuilder() *strings.Builder {
	var sb strings.Builder
	sb.WriteString("rsync")
	addBooleanFlags(&sb)
	addLogFile(&sb)
	addExcludedPatterns(&sb)
	return &sb
}

func getRsyncCommandString(sb *strings.Builder) string {
	return sb.String()
}
