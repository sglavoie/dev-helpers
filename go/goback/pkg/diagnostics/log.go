// Package diagnostics retains bounded output from failed snapshot transfers.
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
	entries, err := os.ReadDir(dir)
	if err != nil {
		return file.Name(), err
	}
	var names []string
	for _, entry := range entries {
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "failure-") && strings.HasSuffix(entry.Name(), ".log") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for len(names) > KeepLogs {
		if err := os.Remove(filepath.Join(dir, names[0])); err != nil && !os.IsNotExist(err) {
			return file.Name(), fmt.Errorf("trim old diagnostic log: %w", err)
		}
		names = names[1:]
	}
	return file.Name(), nil
}
