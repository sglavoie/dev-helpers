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
	b.setUpdatedSourceDestination(src, dest)
	return b
}

func commandToRunWeekly() *RsyncBuilderWeekly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	src = dest + "/daily/" // append slash to avoid copying the daily directory itself
	dest = dest + "/weekly"
	b := &RsyncBuilderWeekly{}
	b.setUpdatedSourceDestination(src, dest)
	return b
}

func commandToRunMonthly() *RsyncBuilderMonthly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	src = dest + "/daily"
	dest = fmt.Sprintf("%s/monthly/monthly_%s.tar.gz", dest, time.Now().Format("20060102"))
	b := &RsyncBuilderMonthly{}
	b.setUpdatedSourceDestination(src, dest)
	return b
}
