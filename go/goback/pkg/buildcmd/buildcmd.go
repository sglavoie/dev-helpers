package buildcmd

import "strings"

func AsStringDaily() string {
	c := commandToRunDailyCheck()
	c.BuildCheck()
	return c.String()
}

func AsStringWeekly() string {
	c := commandToRunWeeklyCheck()
	c.BuildCheck()
	return c.String()
}

func AsStringMonthly() string {
	c := commandToRunMonthlyCheck()
	c.BuildCheck()
	return c.String()
}

func PrintCommandDaily() {
	c := commandToRunDailyNoCheck()
	c.BuildNoCheck()
	c.WrapLongLinesWithBackslashes()
	c.PrintString()
}

func PrintCommandWeekly() {
	c := commandToRunWeeklyNoCheck()
	c.BuildNoCheck()
	c.WrapLongLinesWithBackslashes()
	c.PrintString()
}

func PrintCommandMonthly() {
	c := commandToRunMonthlyNoCheck()
	c.BuildNoCheck()
	c.WrapLongLinesWithBackslashes()
	c.PrintString()
}

func commandToRunDailyCheck() *RsyncBuilderDaily {
	src, dest := mustExitOnInvalidSourceOrDestination()
	return commandToRunDaily(src, dest)
}

func commandToRunDailyNoCheck() *RsyncBuilderDaily {
	src, dest := getSourceAndDestination()
	return commandToRunDaily(src, dest)
}

func commandToRunDaily(src, dest string) *RsyncBuilderDaily {
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

func commandToRunWeeklyCheck() *RsyncBuilderWeekly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	return commandToRunWeekly(src, dest)
}

func commandToRunWeeklyNoCheck() *RsyncBuilderWeekly {
	src, dest := getSourceAndDestination()
	return commandToRunWeekly(src, dest)
}

func commandToRunWeekly(src, dest string) *RsyncBuilderWeekly {
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

func commandToRunMonthlyCheck() *RsyncBuilderMonthly {
	src, dest := mustExitOnInvalidSourceOrDestination()
	return commandToRunMonthly(src, dest)
}

func commandToRunMonthlyNoCheck() *RsyncBuilderMonthly {
	src, dest := getSourceAndDestination()
	return commandToRunMonthly(src, dest)
}

func commandToRunMonthly(src, dest string) *RsyncBuilderMonthly {
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
