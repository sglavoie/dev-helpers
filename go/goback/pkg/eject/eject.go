// Package eject unmounts the external volumes goback reads from and writes to.
// It only ever runs diskutil eject, only ever against a mount point strictly
// beneath /Volumes, and every effect goes through Deps, so ejecting can be
// exercised without a drive. Nothing here terminates the process: each volume
// produces an Outcome and the caller decides what a failure means.
package eject

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/config"
	"github.com/spf13/viper"
)

// Status is what happened to one volume.
type Status string

const (
	// StatusAbsent means the volume is not mounted. It is not a failure: a
	// drive that is already unplugged is in the state that was asked for.
	StatusAbsent Status = "absent"

	// StatusEjected means diskutil unmounted the volume.
	StatusEjected Status = "ejected"

	// StatusFailed means the volume was mounted and diskutil refused.
	StatusFailed Status = "failed"
)

// Outcome is the result of considering a single volume.
type Outcome struct {
	Volume string
	Status Status
	Output string
	Err    error
}

// Summary is the one-line description of the outcome.
func (o Outcome) Summary() string {
	switch o.Status {
	case StatusAbsent:
		return fmt.Sprintf("%s: not mounted, skipped", o.Volume)
	case StatusFailed:
		return fmt.Sprintf("%s: failed to eject: %v", o.Volume, o.Err)
	default:
		return fmt.Sprintf("%s: ejected", o.Volume)
	}
}

// Report is the outcome of every volume that was considered, in the order they
// were attempted.
type Report struct {
	Outcomes []Outcome
}

// Counts returns how many volumes were ejected, skipped as absent, and failed.
func (r Report) Counts() (ejected, absent, failed int) {
	for _, outcome := range r.Outcomes {
		switch outcome.Status {
		case StatusEjected:
			ejected++
		case StatusAbsent:
			absent++
		case StatusFailed:
			failed++
		}
	}
	return ejected, absent, failed
}

// Failed reports whether any mounted volume could not be ejected.
func (r Report) Failed() bool {
	_, _, failed := r.Counts()
	return failed > 0
}

// Err names every mounted volume that could not be ejected, and is nil when
// none failed. An absent volume is never an error.
func (r Report) Err() error {
	var failures []string
	for _, outcome := range r.Outcomes {
		if outcome.Status == StatusFailed {
			failures = append(failures, outcome.Volume)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("failed to eject %s", strings.Join(failures, ", "))
}

// Render writes one line per volume followed by a count, so a partial result is
// visible even when the caller turns Err into a nonzero exit.
func (r Report) Render(w io.Writer) {
	if len(r.Outcomes) == 0 {
		fmt.Fprintf(w, "No configured volume under %s to eject\n", VolumesRoot)
		return
	}

	for _, outcome := range r.Outcomes {
		fmt.Fprintln(w, outcome.Summary())
		if outcome.Status == StatusFailed && strings.TrimSpace(outcome.Output) != "" {
			fmt.Fprintln(w, strings.TrimRight(outcome.Output, "\n"))
		}
	}

	ejected, absent, failed := r.Counts()
	fmt.Fprintf(w, "%d ejected, %d skipped, %d failed\n", ejected, absent, failed)
}

// All ejects every volume the given paths live on, once each and in a
// deterministic order. Every mounted volume is attempted whatever the ones
// before it did, so one refusal never hides the rest.
func All(paths []string, deps Deps) Report {
	var report Report
	for _, volume := range Volumes(paths) {
		report.Outcomes = append(report.Outcomes, ejectVolume(volume, deps))
	}
	return report
}

func ejectVolume(volume string, deps Deps) Outcome {
	if !deps.Mounts.Mounted(volume) {
		return Outcome{Volume: volume, Status: StatusAbsent}
	}

	output, err := deps.Runner.Run([]string{"diskutil", "eject", volume})
	if err != nil {
		return Outcome{Volume: volume, Status: StatusFailed, Output: output, Err: err}
	}
	return Outcome{Volume: volume, Status: StatusEjected, Output: output}
}

// Eject unmounts the volume holding the active profile's destination.
func Eject(out io.Writer, deps Deps) error {
	dest := strings.TrimSpace(viper.GetString(config.ActiveProfilePrefix() + "destination"))
	if dest == "" {
		return errors.New("destination not set")
	}
	if _, ok := VolumeOf(dest); !ok {
		return fmt.Errorf("destination %q is not on a volume under %s", dest, VolumesRoot)
	}
	return EjectPaths(out, []string{dest}, deps)
}

// EjectPaths unmounts every volume the given destinations live on. It is what
// a snapshot run with ejectOnExit uses once all its profiles are done.
func EjectPaths(out io.Writer, destinations []string, deps Deps) error {
	report := All(destinations, deps)
	report.Render(out)
	return report.Err()
}
