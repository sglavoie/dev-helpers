package buildcmd

import (
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/viper"
)

// IsConfigured reports whether a backup type (e.g. "weekly", "monthly") has
// an rsync config section for the active profile.
func IsConfigured(backupType string) bool {
	return viper.IsSet(config.ActiveProfilePrefix() + "rsync." + backupType + ".archive")
}

func BuildDaily() (*RsyncBuilder, error) {
	c, err := commandToRunDailyCheck()
	if err != nil {
		return nil, err
	}
	if err := c.BuildCheck(); err != nil {
		return nil, err
	}
	return c, nil
}

func BuildWeekly() (*RsyncBuilder, error) {
	c, err := commandToRunWeeklyCheck()
	if err != nil {
		return nil, err
	}
	if err := c.BuildCheck(); err != nil {
		return nil, err
	}
	return c, nil
}

func BuildMonthly() (*RsyncBuilder, error) {
	c, err := commandToRunMonthlyCheck()
	if err != nil {
		return nil, err
	}
	if err := c.BuildCheck(); err != nil {
		return nil, err
	}
	return c, nil
}

func PrintCommandDaily() error {
	c := commandToRunDailyNoCheck()
	c.BuildNoCheck()
	c.FormattedPreview()
	return nil
}

func PrintCommandWeekly() error {
	c := commandToRunWeeklyNoCheck()
	c.BuildNoCheck()
	c.FormattedPreview()
	return nil
}

func PrintCommandMonthly() error {
	c := commandToRunMonthlyNoCheck()
	c.BuildNoCheck()
	c.FormattedPreview()
	return nil
}

func commandToRunDailyCheck() (*RsyncBuilder, error) {
	src, dest, err := validateSourceAndDestination()
	if err != nil {
		return nil, err
	}
	return commandToRunDaily(src, dest), nil
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

func commandToRunWeeklyCheck() (*RsyncBuilder, error) {
	src, dest, err := validateSourceAndDestination()
	if err != nil {
		return nil, err
	}
	return commandToRunWeekly(src, dest), nil
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

func commandToRunMonthlyCheck() (*RsyncBuilder, error) {
	src, dest, err := validateSourceAndDestination()
	if err != nil {
		return nil, err
	}
	return commandToRunMonthly(src, dest), nil
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
