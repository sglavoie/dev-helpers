package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestGeneratedDefaultsMatchQuickStart(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	path := filepath.Join(t.TempDir(), "config.json")
	viper.SetConfigFile(path)
	mustCreateDefaultFile()
	generated := viper.New()
	generated.SetConfigFile(path)
	if err := generated.ReadInConfig(); err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, block, found := strings.Cut(string(readme), "```json\n")
	if !found {
		t.Fatal("quick-start JSON missing")
	}
	block, _, _ = strings.Cut(block, "```")
	example := viper.New()
	example.SetConfigType("json")
	if err := example.ReadConfig(bytes.NewBufferString(block)); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"confirmExec", "ejectOnExit", "showProgress", "editor", "profiles.default.rsync"} {
		if !reflect.DeepEqual(generated.Get(key), example.Get(key)) {
			t.Fatalf("%s: generated=%#v, README=%#v", key, generated.Get(key), example.Get(key))
		}
	}
}
