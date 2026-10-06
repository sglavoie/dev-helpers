package diagnostics

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTailRetainsNewestOutputAndBoundsStorage(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tail := &Tail{}
	tail.Write(bytes.Repeat([]byte("x"), MaxBytes*2))
	tail.Write([]byte("permission denied: important.txt\n"))
	path, err := tail.Save("laptop", "daily", "rsync source destination", 23)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail.data) != MaxBytes || !bytes.HasSuffix(data, []byte("permission denied: important.txt\n")) || !bytes.Contains(data, []byte("earlier output omitted")) {
		t.Fatal("tail lost final diagnostic or exceeded bound")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "unrelated.log"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < KeepLogs+2; i++ {
		if _, err := tail.Save("laptop", "daily", "rsync", 23); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != KeepLogs+1 {
		t.Fatalf("retained %d files", len(entries))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("oldest log was not trimmed")
	}
}

func TestConcurrentOutputStreams(t *testing.T) {
	tail := &Tail{}
	var writers sync.WaitGroup
	for _, text := range []string{"stdout\n", "stderr\n"} {
		writers.Add(1)
		go func(text string) {
			defer writers.Done()
			for i := 0; i < 100; i++ {
				tail.Write([]byte(text))
			}
		}(text)
	}
	writers.Wait()
	if strings.Count(string(tail.data), "stdout") != 100 || strings.Count(string(tail.data), "stderr") != 100 {
		t.Fatal("lost output")
	}
}
