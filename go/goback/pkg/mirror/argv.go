package mirror

import (
	"fmt"
	"strings"
)

// outFormat is the machine-stable itemization format the parser reads: an
// eleven-character change summary, a space, then the path. It is pinned here
// instead of relying on the default of --itemize-changes so the parser can
// never be broken by a symlink target or a locale.
const outFormat = "%i %n"

// mirrorFlags are the fixed rsync flags of every mirror. They are not
// configurable: the mirror promises an exact copy with full metadata, and each
// flag below is part of that promise.
func mirrorFlags() []string {
	return []string{
		// Full supported APFS metadata.
		"--archive",
		"--hard-links",
		"--acls",
		"--xattrs",
		"--crtimes",

		// Exact mirror: destination-only content is removed, but only
		// after the transfer succeeded, and never while rsync is
		// reporting I/O errors reading the source.
		"--delete",
		"--delete-delay",

		// Resumable partial data, kept out of the destination tree.
		"--partial-dir=" + PartialDir,

		"--out-format=" + outFormat,
		"--stats",
	}
}

// excludedFromTransfer reports whether a source entry is left out of every
// mirror by the fixed policy above. rsync excludes a relative --partial-dir
// from the transfer itself, so a source holding nothing else reads as non-empty
// while mirroring it would still delete the whole destination.
func excludedFromTransfer(name string) bool {
	return name == PartialDir
}

// realFlags are the flags only a real transfer carries. They are about what a
// person watching the transfer sees, so they must never appear in a dry run,
// whose statistics are parsed rather than read.
func realFlags() []string {
	return []string{
		"--info=progress2",
		"--human-readable",
	}
}

// deletionBoundaryFlags make rsync read the reviewed deletion boundary from a
// file. --from0 is what allows a rule to name a file whose name holds a
// newline: an argument or a line-delimited file would silently split it into
// two rules, and the entry it means to name would be protected by accident.
func deletionBoundaryFlags(rulesPath string) []string {
	return []string{
		"--from0",
		"--filter=merge " + rulesPath,
	}
}

// DryRunArgv builds the argument vector of a preflight. There is no shell, so
// no path needs quoting and a filename containing spaces, quotes, or globs is
// passed through untouched.
func DryRunArgv(cfg Config) []string {
	return argv(cfg, []string{"--dry-run"})
}

// TransferArgv builds the argument vector of a real transfer. It always carries
// a deletion boundary, because the transfer is a separate rsync process that
// scans the destination on its own and would otherwise delete whatever it finds
// there that no plan ever listed.
func TransferArgv(cfg Config, rulesPath string) []string {
	return argv(cfg, append(realFlags(), deletionBoundaryFlags(rulesPath)...))
}

func argv(cfg Config, extra []string) []string {
	out := []string{cfg.RsyncBinary}
	out = append(out, mirrorFlags()...)
	out = append(out, extra...)
	return append(out, sourceArg(cfg.Source), destinationArg(cfg.Destination))
}

// DeletionRules renders the boundary a real transfer obeys: every destination
// path the mirror walked may be deleted, and everything else is protected. An
// entry created after that walk therefore survives a transfer that finds it,
// instead of being deleted as destination-only content nobody reviewed.
//
// Rules are terminated by a null byte because rsync reads a merged rule file
// that way under --from0, which is the only form a filename holding a newline
// survives.
func DeletionRules(paths []string) []byte {
	var rules strings.Builder
	for _, path := range paths {
		rules.WriteString("R /" + filterPattern(path))
		rules.WriteByte(0)
	}
	rules.WriteString("P /**")
	rules.WriteByte(0)
	return []byte(rules.String())
}

// filterPattern escapes the characters rsync reads as wildcards, so a rule
// matches the single path it names rather than everything that looks like it. A
// backslash is an escape only in a pattern that holds a wildcard, so a name
// without one is passed through exactly as it is.
func filterPattern(path string) string {
	if !strings.ContainsAny(path, "*?[") {
		return path
	}

	var pattern strings.Builder
	for _, char := range path {
		if strings.ContainsRune(`*?[\`, char) {
			pattern.WriteByte('\\')
		}
		pattern.WriteRune(char)
	}
	return pattern.String()
}

// sourceArg ends the source with a slash so rsync copies its contents rather
// than the directory itself, which is what keeps an extra Media level from
// appearing inside the destination.
func sourceArg(source string) string {
	return strings.TrimSuffix(source, "/") + "/"
}

// destinationArg carries no trailing slash: the destination is the directory
// the source contents land in.
func destinationArg(destination string) string {
	if destination == "/" {
		return destination
	}
	return strings.TrimSuffix(destination, "/")
}

// FormatArgv renders an argument vector for display, quoting only the
// arguments that would be ambiguous to read.
func FormatArgv(argv []string) string {
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		if strings.ContainsAny(arg, " \t\n\"'") {
			quoted = append(quoted, fmt.Sprintf("%q", arg))
			continue
		}
		quoted = append(quoted, arg)
	}
	return strings.Join(quoted, " ")
}
