package buildcmd

import (
	"fmt"
	"time"
)

func AsStringDaily() string {
	c := commandToRunDaily()
	c.Build()
	return c.String()
}

func AsStringWeekly() string {
	c := commandToRunWeekly()
	c.Build()
	return c.String()
}

func AsStringMonthly() string {
	c := commandToRunMonthly()
	c.Build()
	return c.String()
}

func PrintCommandDaily() {
	c := commandToRunDaily()
	c.Build()
	c.WrapLongLinesWithBackslashes()
	c.PrintString()
}

func PrintCommandWeekly() {
	c := commandToRunWeekly()
	c.Build()
	c.WrapLongLinesWithBackslashes()
	c.PrintString()
}

func PrintCommandMonthly() {
	c := commandToRunMonthly()
	c.Build()
	c.WrapLongLinesWithBackslashes()
	c.PrintString()
}

func commandToRunDaily() *RsyncBuilderDaily {
	src, dest := mustExitOnInvalidSourceOrDestination()
	dest = dest + "/daily"
	b := &RsyncBuilderDaily{}
	b.setUpdatedSourceDestinationDirs(src, dest)
	return b
}

func commandToRunWeekly() *RsyncBuilderWeekly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	src = dest + "/daily/" // append slash to avoid copying the daily directory itself
	dest = dest + "/weekly"
	b := &RsyncBuilderWeekly{}
	b.setUpdatedSourceDestinationDirs(src, dest)
	return b
}

func commandToRunMonthly() *RsyncBuilderMonthly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	src = dest + "/daily"
	destDir := fmt.Sprintf("%s/monthly", dest)
	destFile := fmt.Sprintf("%s/monthly_%s.tar.gz", destDir, time.Now().Format("20060102"))
	b := &RsyncBuilderMonthly{}
	b.setUpdatedSourceDestinationDirToFile(src, destDir, destFile)
	return b
}
