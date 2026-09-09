package mirror

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandAdaptersUseWorkingDirectoryAndResolveRelativeExecutable(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, "destination with spaces")
	makeDir(t, destination)
	writeFile(t, filepath.Join(destination, "marker"), "from the destination")
	program := filepath.Join(root, "helper")
	if err := os.WriteFile(program, []byte("#!/bin/sh\ncat marker\nprintf 'diagnostic' >&2\nexit \"$1\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, program)
	if err != nil {
		t.Fatal(err)
	}
	for _, exit := range []string{"0", "23"} {
		t.Run("exit "+exit, func(t *testing.T) {
			command := Command{Argv: []string{relative, exit}, Dir: destination}
			wantCode := 0
			if exit == "23" {
				wantCode = 23
			}
			result, err := (osRunner{}).Run(context.Background(), command)
			if err != nil || result.ExitCode != wantCode || result.Stdout != "from the destination" || result.Stderr != "diagnostic" {
				t.Fatalf("captured result = %+v, error = %v", result, err)
			}
			var stdout, stderr bytes.Buffer
			code, err := (osStreamer{stdout: &stdout, stderr: &stderr}).Stream(context.Background(), command)
			if err != nil || code != wantCode || stdout.String() != result.Stdout || stderr.String() != result.Stderr {
				t.Fatalf("streamed code = %d, error = %v, stdout = %q, stderr = %q", code, err, &stdout, &stderr)
			}
		})
	}
	if after, err := os.Getwd(); err != nil || after != cwd {
		t.Fatalf("parent working directory changed: %q, %v", after, err)
	}
}

func TestCommandAdaptersRefuseUnusableCommands(t *testing.T) {
	for _, command := range []Command{
		{},
		{Argv: []string{"goback-nonexistent-command"}},
		{Argv: []string{"pwd"}, Dir: filepath.Join(t.TempDir(), "missing")},
	} {
		if result, err := (osRunner{}).Run(context.Background(), command); err == nil || result.Stdout != "" {
			t.Fatalf("captured command %v = %+v, %v, want failure", command, result, err)
		}
		var out bytes.Buffer
		if code, err := (osStreamer{stdout: &out, stderr: &out}).Stream(context.Background(), command); err == nil || code == 0 || out.Len() != 0 {
			t.Fatalf("streamed command %v = %d, %v, output %q, want failure", command, code, err, &out)
		}
	}
}

func TestCommandDisplayAndHistoryIncludeWorkingDirectory(t *testing.T) {
	command := Command{Argv: []string{"rsync", "/bound/source/", "."}, Dir: "/bound/destination with spaces"}
	result := Result{Command: command, Status: StatusSucceeded}
	if !strings.Contains(result.CommandString(), command.Dir) || !strings.Contains(result.CommandString(), "rsync /bound/source/ .") {
		t.Fatalf("history command = %q, want working directory and argv", result.CommandString())
	}
	var out bytes.Buffer
	(Plan{Command: command}).Render(&out)
	if !strings.Contains(out.String(), command.Dir) {
		t.Fatalf("plan = %q, want working directory", &out)
	}
}
