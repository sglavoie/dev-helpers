package buildcmd

import (
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
)

func BuildDaily() *RsyncBuilder {
	c := commandToRunDailyCheck()
	c.BuildCheck()
	return c
}

func BuildWeekly() *RsyncBuilder {
	c := commandToRunWeeklyCheck()
	c.BuildCheck()
	return c
}

func BuildMonthly() *RsyncBuilder {
	c := commandToRunMonthlyCheck()
	c.BuildCheck()
	return c
}

func PrintCommandDaily() {
	c := commandToRunDailyNoCheck()
	c.BuildNoCheck()
	c.FormattedPreview()
}

func PrintCommandWeekly() {
	c := commandToRunWeeklyNoCheck()
	c.BuildNoCheck()
	c.FormattedPreview()
}

func PrintCommandMonthly() {
	c := commandToRunMonthlyNoCheck()
	c.BuildNoCheck()
	c.FormattedPreview()
}

func commandToRunDailyCheck() *RsyncBuilder {
	src, dest := mustExitOnInvalidSourceOrDestination()
	return commandToRunDaily(src, dest)
}

func commandToRunDailyNoCheck() *RsyncBuilder {
	src, dest := sourceAndDestination()
	return commandToRunDaily(src, dest)
}

func commandToRunDaily(src, dest string) *RsyncBuilder {
	dest = dest + "/daily"
	b := &RsyncBuilder{
		builder: builder{
			sb:             &strings.Builder{},
			updatedSrc:     src,
			updatedDestDir: dest,
			builderType:    models.Daily{},
		},
	}
	return b
}

func commandToRunWeeklyCheck() *RsyncBuilder {
	src, dest := mustExitOnInvalidSourceOrDestination()
	return commandToRunWeekly(src, dest)
}

func commandToRunWeeklyNoCheck() *RsyncBuilder {
	src, dest := sourceAndDestination()
	return commandToRunWeekly(src, dest)
}

func commandToRunWeekly(src, dest string) *RsyncBuilder {
	src = dest + "/daily/" // append slash to avoid copying the daily directory itself
	dest = dest + "/weekly"
	b := &RsyncBuilder{
		builder: builder{
			sb:             &strings.Builder{},
			updatedSrc:     src,
			updatedDestDir: dest,
			builderType:    models.Weekly{},
		},
	}
	return b
}

func commandToRunMonthlyCheck() *RsyncBuilder {
	src, dest := mustExitOnInvalidSourceOrDestination()
	return commandToRunMonthly(src, dest)
}

func commandToRunMonthlyNoCheck() *RsyncBuilder {
	src, dest := sourceAndDestination()
	return commandToRunMonthly(src, dest)
}

func commandToRunMonthly(src, dest string) *RsyncBuilder {
	src = dest + "/daily/" // append slash to avoid copying the daily directory itself
	dest = dest + "/monthly"
	b := &RsyncBuilder{
		builder: builder{
			sb:             &strings.Builder{},
			updatedSrc:     src,
			updatedDestDir: dest,
			builderType:    models.Monthly{},
		},
	}
	return b
}
