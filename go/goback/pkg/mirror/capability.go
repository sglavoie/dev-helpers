package mirror

import (
	"context"
	"fmt"
	"strings"
)

// requiredCapabilities are the rsync build options the fixed metadata policy
// depends on. The rsync shipped with macOS is missing several of them, and it
// silently degrades rather than failing, so the mirror refuses to run instead
// of producing an incomplete copy.
var requiredCapabilities = []struct {
	capability string
	flag       string
}{
	{"hardlinks", "--hard-links"},
	{"acls", "--acls"},
	{"xattrs", "--xattrs"},
	{"crtimes", "--crtimes"},
}

// ProbeRsync checks that the configured rsync exists and was built with every
// capability the mirror needs.
func ProbeRsync(ctx context.Context, cfg Config, deps Deps) error {
	result, err := deps.Runner.Run(ctx, []string{cfg.RsyncBinary, "--version"})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s --version failed with exit code %d: %s", cfg.RsyncBinary, result.ExitCode, strings.TrimSpace(result.Stderr))
	}

	capabilities := parseCapabilities(result.Stdout)
	if len(capabilities) == 0 {
		return fmt.Errorf("%s does not report its capabilities, so it cannot be checked against what the mirror needs (hard links, ACLs, extended attributes, and creation times); install rsync 3 and point the mirror at it", cfg.RsyncBinary)
	}

	var missing []string
	for _, required := range requiredCapabilities {
		if _, ok := capabilities[required.capability]; !ok {
			missing = append(missing, fmt.Sprintf("%s (%s)", required.capability, required.flag))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s was built without %s: the mirror copies full metadata and cannot fall back, so install an rsync 3 with those capabilities", cfg.RsyncBinary, strings.Join(missing, ", "))
	}
	return nil
}

// parseCapabilities reads the comma-separated capability list rsync prints
// under "Capabilities:". Unsupported entries are printed as "no <name>", so
// they never match a required capability.
func parseCapabilities(version string) map[string]struct{} {
	capabilities := make(map[string]struct{})
	inSection := false
	for _, line := range strings.Split(version, "\n") {
		if strings.HasPrefix(line, "Capabilities:") {
			inSection = true
			continue
		}
		if !inSection {
			continue
		}
		// The list is indented; the next unindented line ends it.
		if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, " ") {
			break
		}
		for _, item := range strings.Split(line, ",") {
			item = strings.ToLower(strings.TrimSpace(item))
			if item != "" {
				capabilities[item] = struct{}{}
			}
		}
	}
	return capabilities
}
