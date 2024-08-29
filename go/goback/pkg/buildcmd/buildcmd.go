package buildcmd

import "strings"

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
	b := &RsyncBuilderDaily{
		builder: builder{
			sb:             &strings.Builder{},
			updatedSrc:     src,
			updatedDestDir: dest,
			builderType:    "daily",
		},
	}
	return b
}

func commandToRunWeekly() *RsyncBuilderWeekly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	src = dest + "/daily/" // append slash to avoid copying the daily directory itself
	dest = dest + "/weekly"
	b := &RsyncBuilderWeekly{
		builder: builder{
			sb:             &strings.Builder{},
			updatedSrc:     src,
			updatedDestDir: dest,
			builderType:    "weekly",
		},
	}
	return b
}

func commandToRunMonthly() *RsyncBuilderMonthly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	src = dest + "/daily/" // append slash to avoid copying the daily directory itself
	dest = dest + "/monthly"
	b := &RsyncBuilderMonthly{
		builder: builder{
			sb:             &strings.Builder{},
			updatedSrc:     src,
			updatedDestDir: dest,
			builderType:    "monthly",
		},
	}
	return b
}
