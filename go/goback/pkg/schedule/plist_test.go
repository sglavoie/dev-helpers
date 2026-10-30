package schedule

import (
	"bytes"
	"encoding/xml"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Decode the plist as a consumer, including XML escaping and the distinction
// between argument strings, integer times, and dictionaries.
type plistNode struct {
	XMLName  xml.Name
	Text     string      `xml:",chardata"`
	Children []plistNode `xml:",any"`
}

func dictionary(t *testing.T, node plistNode) map[string]plistNode {
	t.Helper()
	if node.XMLName.Local != "dict" || len(node.Children)%2 != 0 {
		t.Fatalf("invalid dictionary: %+v", node)
	}
	out := make(map[string]plistNode)
	for i := 0; i < len(node.Children); i += 2 {
		key := node.Children[i]
		if key.XMLName.Local != "key" {
			t.Fatalf("invalid key: %+v", key)
		}
		out[key.Text] = node.Children[i+1]
	}
	return out
}

func testOptions() Options {
	return Options{
		Binary: "/Users/me/My Tools/goback", Config: "/Users/me/A & B/config.json",
		Profile: "work & personal", Home: "/Users/me", Label: "local.goback.daily",
		At: "09:05", ExecPath: "/Users/me/My Tools:/opt/homebrew/bin:/usr/bin:/bin", LogDir: "/Users/me/Library/Logs/goback",
	}
}

func TestDailyPlistExecutionContract(t *testing.T) {
	opts := testOptions()
	data, err := Daily(opts)
	if err != nil {
		t.Fatal(err)
	}
	var root plistNode
	if err := xml.Unmarshal(data, &root); err != nil {
		t.Fatalf("invalid XML: %v\n%s", err, data)
	}
	job := dictionary(t, root.Children[0])
	if job["Label"].Text != opts.Label || job["WorkingDirectory"].Text != opts.Home {
		t.Fatalf("job identity or cwd: %+v", job)
	}
	var argv []string
	for _, value := range job["ProgramArguments"].Children {
		if value.XMLName.Local != "string" {
			t.Fatalf("argument is not a string: %+v", value)
		}
		argv = append(argv, value.Text)
	}
	want := []string{opts.Binary, "--config", opts.Config, "--profile", opts.Profile, "run", "daily", "--non-interactive", "--quiet"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv=%q, want %q", argv, want)
	}
	env := dictionary(t, job["EnvironmentVariables"])
	if env["HOME"].Text != opts.Home || env["PATH"].Text != opts.ExecPath {
		t.Fatalf("environment: %+v", env)
	}
	when := dictionary(t, job["StartCalendarInterval"])
	if len(when) != 2 || when["Hour"].XMLName.Local != "integer" || when["Hour"].Text != "9" || when["Minute"].Text != "5" {
		t.Fatalf("time: %+v", when)
	}
	if job["StandardOutPath"].Text != filepath.Join(opts.LogDir, opts.Label+".stdout.log") || job["StandardErrorPath"].Text != filepath.Join(opts.LogDir, opts.Label+".stderr.log") || job["Umask"].Text != "63" {
		t.Fatalf("logging or permissions: %+v", job)
	}
	for _, absent := range []string{"KeepAlive", "RunAtLoad", "StartInterval"} {
		if _, exists := job[absent]; exists {
			t.Fatalf("job unexpectedly sets %s", absent)
		}
	}
	if runtime.GOOS == "darwin" {
		command := exec.Command("/usr/bin/plutil", "-lint", "-")
		command.Stdin = bytes.NewReader(data)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("macOS rejects plist: %v\n%s", err, out)
		}
	}
}

func TestDailyRejectsInvalidSchedule(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Options)
		want string
	}{
		{"time", func(o *Options) { o.At = "24:00" }, "HH:MM"},
		{"short time", func(o *Options) { o.At = "9:00" }, "HH:MM"},
		{"minute", func(o *Options) { o.At = "09:60" }, "HH:MM"},
		{"label", func(o *Options) { o.Label = "../bad" }, "label"},
		{"profile", func(o *Options) { o.Profile = "" }, "profile"},
		{"binary", func(o *Options) { o.Binary = "goback" }, "absolute"},
		{"config", func(o *Options) { o.Config = "config.json" }, "absolute"},
		{"logs", func(o *Options) { o.LogDir = "logs" }, "absolute"},
		{"home", func(o *Options) { o.Home = "~" }, "absolute"},
		{"relative PATH", func(o *Options) { o.ExecPath = "/usr/bin:bin" }, "--exec-path"},
		{"empty PATH entry", func(o *Options) { o.ExecPath = "/usr/bin:" }, "--exec-path"},
		{"empty PATH", func(o *Options) { o.ExecPath = "" }, "--exec-path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := testOptions()
			tc.edit(&opts)
			data, err := Daily(opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) || len(data) != 0 {
				t.Fatalf("Daily returned %q, %v", data, err)
			}
		})
	}
}
