// Package diagnostics retains bounded output from failed backup commands.
package diagnostics

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const MaxBytes = 64 * 1024
const KeepLogs = 20

// Tail accepts both process streams concurrently and keeps only their last
// MaxBytes bytes. Writing it never affects the transfer's result.
type Tail struct {
	mu        sync.Mutex
	data      []byte
	truncated bool
}

func (t *Tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	if len(t.data)+n > MaxBytes {
		t.truncated = true
		if n >= MaxBytes {
			t.data = append(t.data[:0], p[n-MaxBytes:]...)
			return n, nil
		}
		t.data = append(t.data[:0], t.data[len(t.data)+n-MaxBytes:]...)
	}
	t.data = append(t.data, p...)
	return n, nil
}

// Save writes a private, self-describing failure log and trims older goback
// logs. Successful runs and dry runs never call Save.
func (t *Tail) Save(profile, kind, command string, exitCode int) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".goback", "logs")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, "failure-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-*.log")
	if err != nil {
		return "", err
	}
	header := fmt.Sprintf("Profile: %q\nBackup type: %s\nExit code: %d\nCommand: %s\n", profile, kind, exitCode, command)
	if len(header) > MaxBytes {
		header = header[:MaxBytes] + "\n[header truncated]\n"
	}
	if t.truncated {
		header += "[earlier output omitted; last 64 KiB follows]\n"
	}
	_, writeErr := file.Write(append([]byte(header+"\n"), t.data...))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(file.Name())
		if writeErr != nil {
			return "", writeErr
		}
		return "", closeErr
	}
	names, err := logPaths(dir)
	if err != nil {
		return file.Name(), err
	}
	for _, name := range names[min(KeepLogs, len(names)):] {
		if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
			return file.Name(), fmt.Errorf("trim old diagnostic log: %w", err)
		}
	}
	return file.Name(), nil
}

// LogPaths returns regular failure logs newest first. An absent log directory
// is an empty listing; listing never creates it or follows log symlinks.
func LogPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return logPaths(filepath.Join(home, ".goback", "logs"))
}

func logPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "failure-") && strings.HasSuffix(entry.Name(), ".log") {
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}
