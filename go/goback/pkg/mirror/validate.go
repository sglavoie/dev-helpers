package mirror

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// volumesDir is where macOS mounts external volumes.
const volumesDir = "/Volumes"

// Endpoints are the validated paths of a mirror operation.
type Endpoints struct {
	Source string

	// Destination is the configured destination. It must already be a
	// directory: the mirror never creates it, so there is no leaf whose
	// identity this validation could not record.
	Destination string

	// SourceDevice and DestinationDevice are the filesystem identities the
	// endpoints had when they were validated. A drive that is unmounted or
	// swapped keeps its path, so comparing these is the only way to notice
	// that the endpoints approved by the user are no longer the ones about
	// to be written.
	SourceDevice      string
	DestinationDevice string

	// SourceResolved and DestinationResolved are the endpoints with every
	// symlink followed, as validation accepted them. They are what rsync is
	// given, so a path replaced by a symlink after validation cannot redirect
	// either of them.
	SourceResolved      string
	DestinationResolved string

	// SourceIdentity and DestinationIdentity name the directories the
	// endpoints were, rather than the paths they were reached through. A path
	// that comes to hold another ordinary directory keeps its resolution and
	// its device, so these are what the transfer's binding is held to: rsync
	// acts on the directories this validation inspected or on nothing.
	SourceIdentity      string
	DestinationIdentity string
}

// Validate checks every precondition of a mirror without writing anything. It
// runs before rsync is even probed, so a source that is missing, unreadable,
// empty, or on the same filesystem as the destination can never reach the
// stage that produces a deletion plan.
func Validate(cfg Config, deps Deps) (Endpoints, error) {
	source, destination, err := cleanPaths(cfg)
	if err != nil {
		return Endpoints{}, err
	}

	if err := validateSource(source, deps); err != nil {
		return Endpoints{}, err
	}

	endpoints := Endpoints{Source: source, Destination: destination}
	if err := validateDestination(destination, deps); err != nil {
		return Endpoints{}, err
	}

	endpoints.SourceResolved, err = resolveEndpoint(source, deps)
	if err != nil {
		return Endpoints{}, err
	}
	endpoints.DestinationResolved, err = resolveEndpoint(destination, deps)
	if err != nil {
		return Endpoints{}, err
	}

	if err := validateVolume(source, endpoints.SourceResolved, deps); err != nil {
		return Endpoints{}, err
	}
	if err := validateVolume(destination, endpoints.DestinationResolved, deps); err != nil {
		return Endpoints{}, err
	}

	endpoints.SourceDevice, err = deps.Devices.Device(source)
	if err != nil {
		return Endpoints{}, fmt.Errorf("cannot identify the filesystem of the source %s: %w", source, err)
	}
	endpoints.DestinationDevice, err = deps.Devices.Device(destination)
	if err != nil {
		return Endpoints{}, fmt.Errorf("cannot identify the filesystem of the destination %s: %w", destination, err)
	}
	if endpoints.SourceDevice == endpoints.DestinationDevice {
		return Endpoints{}, fmt.Errorf("source %s and destination %s are on the same filesystem: a mirror deletes everything in the destination that is not in the source, so both endpoints must be separate mounted volumes", source, destination)
	}

	// The identities are recorded last, once the endpoints have passed every
	// rule, so what they name is exactly what this validation accepted. They
	// are what the transfer is bound to, so an endpoint replaced by another
	// ordinary directory afterwards is refused rather than mirrored.
	endpoints.SourceIdentity, err = deps.Devices.Identity(endpoints.SourceResolved)
	if err != nil {
		return Endpoints{}, fmt.Errorf("cannot identify the directory the source %s is: %w", source, err)
	}
	endpoints.DestinationIdentity, err = deps.Devices.Identity(endpoints.DestinationResolved)
	if err != nil {
		return Endpoints{}, fmt.Errorf("cannot identify the directory the destination %s is: %w", destination, err)
	}

	return endpoints, nil
}

func cleanPaths(cfg Config) (string, string, error) {
	if cfg.Source == "" || cfg.Destination == "" {
		return "", "", fmt.Errorf("both a mirror source and a mirror destination are required")
	}
	if !filepath.IsAbs(cfg.Source) {
		return "", "", fmt.Errorf("mirror source %s is not an absolute path", cfg.Source)
	}
	if !filepath.IsAbs(cfg.Destination) {
		return "", "", fmt.Errorf("mirror destination %s is not an absolute path", cfg.Destination)
	}

	source := filepath.Clean(cfg.Source)
	destination := filepath.Clean(cfg.Destination)
	if source == destination {
		return "", "", fmt.Errorf("mirror source and destination are the same path: %s", source)
	}
	if contains(source, destination) {
		return "", "", fmt.Errorf("mirror destination %s is inside the source %s", destination, source)
	}
	if contains(destination, source) {
		return "", "", fmt.Errorf("mirror source %s is inside the destination %s, which the mirror would delete", source, destination)
	}
	return source, destination, nil
}

func validateSource(source string, deps Deps) error {
	info, err := deps.FS.Stat(source)
	if os.IsNotExist(err) {
		return fmt.Errorf("mirror source %s does not exist: refusing to mirror an absent source onto a populated destination", source)
	}
	if err != nil {
		return fmt.Errorf("cannot read the mirror source %s: %w", source, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("mirror source %s is not a directory", source)
	}

	entries, err := deps.FS.ReadDir(source)
	if err != nil {
		return fmt.Errorf("cannot read the mirror source %s: %w", source, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("mirror source %s is empty: mirroring it would delete everything in the destination", source)
	}

	transferable := 0
	for _, entry := range entries {
		if !excludedFromTransfer(entry.Name()) {
			transferable++
		}
	}
	if transferable == 0 {
		return fmt.Errorf("mirror source %s is empty of everything the mirror would transfer: it holds only %s, which rsync excludes from every mirror, so mirroring it would delete everything in the destination", source, PartialDir)
	}
	return nil
}

// validateDestination requires an existing destination directory. The mirror
// does not create one: macOS has no atomic create-and-open for directories, so
// a creation would have to look its own result up by name, and nothing the
// kernel offers distinguishes the directory it made from one another process
// running as this user put at that name in the meantime. Refusing an absent
// destination is what keeps every directory the mirror writes into one whose
// identity was recorded before anything was decided.
func validateDestination(destination string, deps Deps) error {
	info, err := deps.FS.Stat(destination)
	if os.IsNotExist(err) {
		return fmt.Errorf("mirror destination %s does not exist: create it yourself before mirroring, since the mirror never creates its destination", destination)
	}
	if err != nil {
		return fmt.Errorf("cannot read the mirror destination %s: %w", destination, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("mirror destination %s exists but is not a directory", destination)
	}
	// A dry run reads the destination and would report a full plan for one
	// nothing can be written into, so the approval would be asked for a
	// transfer that can only fail partway through.
	if err := deps.FS.Writable(destination); err != nil {
		return fmt.Errorf("%s is not writable: %w", destination, err)
	}
	return nil
}

// validateVolume rejects a /Volumes/<name> path whose volume root is not a
// real mount. A stale directory left behind on the system disk after an
// unplugged drive looks exactly like the mounted volume, so mirroring onto it
// would silently fill the internal disk, and mirroring from it would report
// the whole destination as deletable.
func validateVolume(path, resolved string, deps Deps) error {
	root, ok := volumeRoot(path)
	if !ok {
		return nil
	}

	rootDevice, err := deps.Devices.Device(root)
	if err != nil {
		return fmt.Errorf("cannot identify the filesystem of %s: %w", root, err)
	}
	systemDevice, err := deps.Devices.Device("/")
	if err != nil {
		return fmt.Errorf("cannot identify the filesystem of /: %w", err)
	}
	if rootDevice == systemDevice {
		return fmt.Errorf("%s is not a mounted volume: it is a directory on the system disk, which usually means the drive is unplugged and a stale mount point was left behind", root)
	}
	return validateNoEscape(path, resolved, root, deps)
}

func resolveEndpoint(path string, deps Deps) (string, error) {
	resolved, err := deps.FS.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s: %w", path, err)
	}
	return resolved, nil
}

// validateNoEscape refuses an endpoint whose symlinks lead out of the mounted
// volume that was just validated. Stat, Device, and rsync itself all follow
// symlinks, so a destination leaf or parent pointing at the system disk passes
// every device check while rsync writes, and deletes with --delete, wherever
// the link really goes.
func validateNoEscape(path, resolved, root string, deps Deps) error {
	resolvedRoot, err := deps.FS.Resolve(root)
	if err != nil {
		return fmt.Errorf("cannot resolve the mount point %s: %w", root, err)
	}
	if !isVolumeRoot(resolvedRoot) {
		return fmt.Errorf("the mount point %s resolves to %s, which is not a mounted volume under %s: a symlinked mount point would make the mirror write and delete outside the drive it was configured for", root, resolvedRoot, volumesDir)
	}

	if !contains(resolvedRoot, resolved) {
		return fmt.Errorf("%s resolves to %s, which is outside the mounted volume %s: a symlinked endpoint would make the mirror write and delete outside the drive it was configured for", path, resolved, resolvedRoot)
	}
	return nil
}

// volumeRoot returns the /Volumes/<name> mount point a path belongs to.
func volumeRoot(path string) (string, bool) {
	prefix := volumesDir + string(filepath.Separator)
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, prefix)
	name, _, _ := strings.Cut(rest, string(filepath.Separator))
	if name == "" {
		return "", false
	}
	return prefix + name, true
}

// isVolumeRoot reports whether path is itself a /Volumes/<name> mount point.
func isVolumeRoot(path string) bool {
	root, ok := volumeRoot(path)
	return ok && root == path
}

// contains reports whether child is parent itself or lives under it.
func contains(parent, child string) bool {
	return child == parent || strings.HasPrefix(child, parent+string(filepath.Separator))
}
