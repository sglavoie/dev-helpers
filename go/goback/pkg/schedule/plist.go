// Package schedule renders launchd jobs without installing or loading them.
package schedule

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
)

// Options fixes the execution environment independently of an interactive shell.
type Options struct {
	Binary, Config, Profile string
	Label, At               string
	Home, ExecPath, LogDir  string
}

var labelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var timePattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Daily renders one job for one profile. It does not inspect mounted drives or
// create the log directory; the caller installs the reviewed plist separately.
func Daily(opts Options) ([]byte, error) {
	if !labelPattern.MatchString(opts.Label) {
		return nil, fmt.Errorf("label must start with a letter or digit and contain only letters, digits, dots, underscores, or dashes")
	}
	if !timePattern.MatchString(opts.At) {
		return nil, fmt.Errorf("--at must be a local time in HH:MM format (00:00 through 23:59)")
	}
	if opts.Profile == "" {
		return nil, fmt.Errorf("a profile is required")
	}
	for _, path := range []struct{ name, value string }{
		{"binary", opts.Binary}, {"config", opts.Config}, {"home", opts.Home}, {"log directory", opts.LogDir},
	} {
		if !filepath.IsAbs(path.value) {
			return nil, fmt.Errorf("%s must be an absolute path", path.name)
		}
	}
	for _, dir := range strings.Split(opts.ExecPath, ":") {
		if !filepath.IsAbs(dir) {
			return nil, fmt.Errorf("--exec-path must contain only absolute directories separated by colons (no empty entries)")
		}
	}
	hour, _ := strconv.Atoi(opts.At[:2])
	minute, _ := strconv.Atoi(opts.At[3:])
	data := struct {
		Options
		Hour, Minute   int
		Stdout, Stderr string
	}{opts, hour, minute, filepath.Join(opts.LogDir, opts.Label+".stdout.log"), filepath.Join(opts.LogDir, opts.Label+".stderr.log")}
	var out bytes.Buffer
	if err := dailyTemplate.Execute(&out, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

var dailyTemplate = template.Must(template.New("daily").Funcs(template.FuncMap{
	"xml": func(value string) (string, error) {
		var out bytes.Buffer
		err := xml.EscapeText(&out, []byte(value))
		return out.String(), err
	},
}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>{{xml .Label}}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{xml .Binary}}</string>
    <string>--config</string><string>{{xml .Config}}</string>
    <string>--profile</string><string>{{xml .Profile}}</string>
    <string>run</string><string>daily</string>
    <string>--non-interactive</string><string>--quiet</string>
  </array>
  <key>WorkingDirectory</key><string>{{xml .Home}}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>HOME</key><string>{{xml .Home}}</string>
    <key>PATH</key><string>{{xml .ExecPath}}</string>
  </dict>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key><integer>{{.Hour}}</integer>
    <key>Minute</key><integer>{{.Minute}}</integer>
  </dict>
  <key>StandardOutPath</key><string>{{xml .Stdout}}</string>
  <key>StandardErrorPath</key><string>{{xml .Stderr}}</string>
  <key>Umask</key><integer>63</integer>
</dict>
</plist>
`))
