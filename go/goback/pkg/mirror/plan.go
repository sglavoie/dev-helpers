package mirror

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// ErrInsufficientSpace is returned when the destination volume cannot hold the
// data the mirror would transfer.
var ErrInsufficientSpace = errors.New("not enough free space on the destination volume")

// Plan is the outcome of a dry run: everything a real mirror would do, and
// nothing it did.
type Plan struct {
	Endpoints Endpoints
	Argv      []string
	Changes   Changes

	// AvailableBytes is the free space of the destination volume as it is
	// now, before any deletion.
	AvailableBytes uint64

	StartedAt time.Time
	Duration  time.Duration
}

// DryRun validates the mirror, itemizes what a real run would change, and
// returns without writing anything. It never prompts and never records
// history.
func DryRun(ctx context.Context, cfg Config, deps Deps) (Plan, error) {
	return dryRun(ctx, cfg, deps, inspected{})
}

// dryRunBound is the final preflight of a real mirror. It validates the
// configured endpoints exactly like DryRun, but points rsync at the bound names
// of the two directories the transfer itself acts on. A preflight given path
// strings reports on whatever those paths hold while rsync reads them, which a
// directory substituted for the length of that scan alone is enough to make
// something other than what the transfer, bound to identities, later opens.
func dryRunBound(ctx context.Context, cfg Config, deps Deps, source, destination string) (Plan, error) {
	return dryRun(ctx, cfg, deps, inspected{source: source, destination: destination})
}

// inspected names the directories a preflight's rsync is pointed at. It is
// empty for a preflight of the configured paths, which is what a dry run and
// the plan shown for approval are.
type inspected struct {
	source      string
	destination string
}

func (i inspected) or(endpoints Endpoints) (string, string) {
	if i.source == "" {
		return endpoints.Source, endpoints.Destination
	}
	return i.source, i.destination
}

func dryRun(ctx context.Context, cfg Config, deps Deps, at inspected) (Plan, error) {
	endpoints, err := Validate(cfg, deps)
	if err != nil {
		return Plan{}, err
	}
	if err := ProbeRsync(ctx, cfg, deps); err != nil {
		return Plan{}, err
	}

	source, destination := at.or(endpoints)
	plan := Plan{
		Endpoints: endpoints,
		Argv:      DryRunArgv(Config{Source: source, Destination: destination, RsyncBinary: cfg.RsyncBinary}),
		StartedAt: deps.Clock.Now(),
	}

	result, err := deps.Runner.Run(ctx, plan.Argv)
	plan.Duration = deps.Clock.Now().Sub(plan.StartedAt)
	if err != nil {
		return Plan{}, fmt.Errorf("could not run the mirror preflight: %w", err)
	}
	if result.ExitCode != 0 {
		return Plan{}, fmt.Errorf("the mirror preflight failed with exit code %d: %s", result.ExitCode, lastLine(result.Stderr))
	}

	plan.Changes, err = ParseChanges(result.Stdout)
	if err != nil {
		return Plan{}, err
	}

	// The destination is measured as it is now. Deletions are delayed
	// until after the transfer, so the space they free cannot be spent on
	// the transfer that precedes them.
	plan.AvailableBytes, err = deps.Capacity.AvailableBytes(endpoints.Destination)
	if err != nil {
		return Plan{}, err
	}
	if plan.Changes.TransferBytes > plan.AvailableBytes {
		return Plan{}, fmt.Errorf("%w: %s needs %s but only %s is free (deletions happen after the transfer, so the space they free is not available to it)",
			ErrInsufficientSpace, endpoints.Destination, HumanBytes(plan.Changes.TransferBytes), HumanBytes(plan.AvailableBytes))
	}

	return plan, nil
}

// Render writes the human-readable change plan.
func (p Plan) Render(w io.Writer) {
	fmt.Fprintf(w, "Mirror preflight (dry run, nothing was written)\n")
	fmt.Fprintf(w, "  source:      %s\n", p.Endpoints.Source)
	fmt.Fprintf(w, "  destination: %s\n", p.Endpoints.Destination)
	fmt.Fprintf(w, "  command:     %s\n\n", FormatArgv(p.Argv))

	if p.Changes.Empty() {
		fmt.Fprintf(w, "The destination is already up to date.\n")
		return
	}

	fmt.Fprintf(w, "  created:  %d\n", p.Changes.Created)
	fmt.Fprintf(w, "  updated:  %d\n", p.Changes.Updated)
	fmt.Fprintf(w, "  deleted:  %d\n", p.Changes.Deleted)
	fmt.Fprintf(w, "  transfer: %s (%d bytes)\n", HumanBytes(p.Changes.TransferBytes), p.Changes.TransferBytes)
	fmt.Fprintf(w, "  free now: %s\n", HumanBytes(p.AvailableBytes))

	if len(p.Changes.Deletions) > 0 {
		fmt.Fprintf(w, "\nWould delete %d entries from the destination:\n", len(p.Changes.Deletions))
		for _, deletion := range p.Changes.Deletions {
			fmt.Fprintf(w, "  %s\n", deletion.Path)
		}
	}
}

func lastLine(output string) string {
	lines := nonEmptyLines(output)
	if len(lines) == 0 {
		return "no error output"
	}
	return lines[len(lines)-1]
}
