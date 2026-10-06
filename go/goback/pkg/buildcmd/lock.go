package buildcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sglavoie/dev-helpers/go/goback/pkg/destinationlock"
)

// Lock protects the entire profile destination, including the daily source of
// derived backups. Dry runs do not create lock files.
func (r *builder) Lock(ctx context.Context) (context.Context, func(), error) {
	if r.dryRun {
		return ctx, func() {}, nil
	}
	root := filepath.Dir(r.updatedDestDir)
	ctx, release, err := destinationlock.Acquire(ctx, root)
	if err != nil {
		return ctx, nil, err
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err == nil {
		resolvedRoot, err = filepath.Abs(resolvedRoot)
	}
	if err == nil {
		paths := []string{r.updatedDestDir}
		if r.builderType.String() != "daily" {
			paths = append(paths, r.updatedSrc)
		}
		for _, path := range paths {
			resolved, resolveErr := filepath.EvalSymlinks(path)
			if os.IsNotExist(resolveErr) && path == r.updatedDestDir {
				continue // validation rejects dangling links before execution
			}
			if resolveErr != nil {
				err = resolveErr
				break
			}
			resolved, err = filepath.Abs(resolved)
			if err != nil {
				break
			}
			if !destinationlock.Within(resolvedRoot, resolved) {
				err = fmt.Errorf("backup path %s resolves outside locked destination %s", path, resolvedRoot)
				break
			}
		}
	}
	if err != nil {
		release()
		return ctx, nil, err
	}
	return ctx, release, nil
}
