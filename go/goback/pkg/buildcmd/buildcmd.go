package buildcmd

import "strings"

func BuildDaily() *RsyncBuilderDaily {
	c := commandToRunDailyCheck()
	c.BuildCheck()
	return c
}

func BuildWeekly() *RsyncBuilderWeekly {
	c := commandToRunWeeklyCheck()
	c.BuildCheck()
	return c
}

func BuildMonthly() *RsyncBuilderMonthly {
	c := commandToRunMonthlyCheck()
	c.BuildCheck()
	return c
}

func DailyBuilderType() string {
	return RsyncBuilderDaily{
		builder: builder{
			sb:          &strings.Builder{},
			builderType: "daily",
		},
	}.builderType
}

func WeeklyBuilderType() string {
	return RsyncBuilderWeekly{
		builder: builder{
			sb:          &strings.Builder{},
			builderType: "weekly",
		},
	}.builderType
}

func MonthlyBuilderType() string {
	return RsyncBuilderMonthly{
		builder: builder{
			sb:          &strings.Builder{},
			builderType: "monthly",
		},
	}.builderType
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
	src, dest := sourceAndDestination()
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
	src, dest := sourceAndDestination()
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
	src, dest := sourceAndDestination()
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
