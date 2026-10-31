package buildcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/sglavoie/dev-helpers/go/goback/pkg/models"
	"github.com/spf13/viper"
)

// VerificationReport describes a comparison with the current live source, not
// a historical baseline. Completed is false for any partial or failed scan.
type VerificationReport struct {
	Profile     string                   `json:"profile"`
	BackupType  string                   `json:"backup_type"`
	Source      string                   `json:"source"`
	Destination string                   `json:"destination"`
	Completed   bool                     `json:"completed"`
	Differences []VerificationDifference `json:"differences"`
	Errors      []string                 `json:"errors"`
}

type VerificationDifference struct {
	Path   string `json:"path"`   // Relative to daily/, with rsync filename escapes decoded.
	Status string `json:"status"` // "missing", "different", or "unreadable".
}

// VerifyDaily checks file contents, entry types, and symlink targets without
// transferring or deleting anything. Only coordination locks are written.
// Configured deletion, ignore-errors, dry-run, and output flags cannot weaken
// the check. No backup execution or history-writing code is invoked.
func VerifyDaily(ctx context.Context) (report VerificationReport, err error) {
	r := commandToRunDailyNoCheck()
	report = VerificationReport{
		Profile: config.ActiveProfileName, BackupType: "daily",
		Source: r.updatedSrc, Destination: r.updatedDestDir,
		Differences: []VerificationDifference{}, Errors: []string{},
	}
	defer func() {
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
		}
	}()
	if err = ctx.Err(); err != nil {
		return report, err
	}
	if err = r.validateSettings(); err != nil {
		return report, err
	}
	if err = r.validateEndpoints(false); err != nil {
		return report, err
	}
	// A dry run would otherwise happily describe a backup that does not exist.
	if _, err = verificationEndpoint(r.updatedDestDir); err != nil {
		return report, fmt.Errorf("verification requires an existing daily backup: %w", err)
	}
	// Do not use the dry-run lock bypass: a verification must exclude writers.
	ctx, release, err := r.Lock(ctx)
	if err != nil {
		return report, err
	}
	defer release()
	if err = r.validateEndpoints(false); err != nil {
		return report, err
	}
	source, err := verificationEndpoint(r.updatedSrc)
	if err != nil {
		return report, err
	}
	destination, err := verificationEndpoint(r.updatedDestDir)
	if err != nil {
		return report, err
	}

	// Enumerate every selected entry, including unchanged files. Rsync dry-run
	// checksum failures can be silent, so goback reads and hashes files itself.
	args := []string{"--recursive", "--links", "--devices", "--specials", "--ignore-times", "--dry-run", "--itemize-changes", "--itemize-changes", "--out-format=%i\t%n"}
	if viper.GetBool(r.builderSettingsPrefix() + "pruneEmptyDirs") {
		args = append(args, "--prune-empty-dirs")
	}
	args = append(args, FilterArgs(models.Daily{})...)
	// Make even colon-containing and dash-prefixed local paths unambiguous.
	// Preserve trailing slashes and /.: cleaning would change rsync's layout.
	sourceArg, err := absoluteVerificationPath(r.updatedSrc)
	if err != nil {
		return report, err
	}
	destinationArg, err := absoluteVerificationPath(r.updatedDestDir)
	if err != nil {
		return report, err
	}
	args = append(args, "--", sourceArg, destinationArg)
	command := exec.CommandContext(ctx, "rsync", args...)
	command.Env = append(os.Environ(), "LC_ALL=C")
	command.WaitDelay = 5 * time.Second
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	report.Differences, err = compareVerification(ctx, stdout.String(), sourceArg, destinationArg)
	if ctx.Err() != nil {
		err = errors.Join(err, fmt.Errorf("verification interrupted: %w", ctx.Err()))
	} else if runErr != nil {
		// In contrast to a backup, even exit 24 means verification is incomplete.
		err = errors.Join(err, fmt.Errorf("rsync verification incomplete: %w", runErr))
	}
	if message := strings.TrimSpace(stderr.String()); message != "" {
		err = errors.Join(err, fmt.Errorf("rsync: %s", message))
	}
	// A path renamed or retargeted during the scan must not produce a match.
	_, releaseCheck, lockErr := r.Lock(ctx)
	if lockErr == nil {
		releaseCheck()
	}
	err = errors.Join(err, lockErr, source.recheck(r.updatedSrc), destination.recheck(r.updatedDestDir))
	if err != nil {
		return report, err
	}
	report.Completed = true
	return report, nil
}

func absoluteVerificationPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return cwd + string(filepath.Separator) + path, nil
}

type verifiedEndpoint struct {
	path string
	info os.FileInfo
}

func verificationEndpoint(path string) (verifiedEndpoint, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return verifiedEndpoint{}, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return verifiedEndpoint{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return verifiedEndpoint{}, err
	}
	if !info.IsDir() {
		return verifiedEndpoint{}, fmt.Errorf("not a directory: %s", path)
	}
	return verifiedEndpoint{resolved, info}, nil
}

func (before verifiedEndpoint) recheck(path string) error {
	after, err := verificationEndpoint(path)
	if err != nil {
		return fmt.Errorf("verification endpoint changed: %w", err)
	}
	if before.path != after.path || !os.SameFile(before.info, after.info) {
		return fmt.Errorf("verification endpoint changed during scan: %s", path)
	}
	return nil
}
