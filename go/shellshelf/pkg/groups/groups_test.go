package groups

import (
	"encoding/base64"
	"testing"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
)

func encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func setupTestConfig() {
	config.Cfg = &models.Config{
		Commands: models.Commands{
			"1": {Id: "1", Name: "cmd-one", Command: encode("echo one")},
			"2": {Id: "2", Name: "cmd-two", Command: encode("echo two")},
			"3": {Id: "3", Name: "cmd-three", Command: encode("echo three")},
		},
		Groups: make(models.Groups),
	}
}

func TestAddValidation(t *testing.T) {
	t.Run("empty name returns error", func(t *testing.T) {
		setupTestConfig()
		err := Add("", []string{"1"}, true)
		if err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("invalid command ID returns error", func(t *testing.T) {
		setupTestConfig()
		err := Add("bad-group", []string{"1", "99"}, true)
		if err == nil {
			t.Fatal("expected error for invalid command ID")
		}
	})
}

func TestRemoveValidation(t *testing.T) {
	t.Run("non-existent group returns error", func(t *testing.T) {
		setupTestConfig()
		err := Remove("nope")
		if err == nil {
			t.Fatal("expected error for non-existent group")
		}
	})
}

func TestGet(t *testing.T) {
	t.Run("existing group", func(t *testing.T) {
		setupTestConfig()
		config.Cfg.Groups["deploy"] = models.Group{
			Name:        "deploy",
			CommandIDs:  []string{"1", "2"},
			StopOnError: true,
		}
		g, err := Get("deploy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if g.Name != "deploy" {
			t.Errorf("got name %q, want %q", g.Name, "deploy")
		}
		if len(g.CommandIDs) != 2 {
			t.Errorf("got %d command IDs, want 2", len(g.CommandIDs))
		}
		if !g.StopOnError {
			t.Error("expected StopOnError to be true")
		}
	})

	t.Run("non-existent group", func(t *testing.T) {
		setupTestConfig()
		_, err := Get("nope")
		if err == nil {
			t.Fatal("expected error for non-existent group")
		}
	})
}

func TestList(t *testing.T) {
	t.Run("sorted by name", func(t *testing.T) {
		setupTestConfig()
		config.Cfg.Groups["zebra"] = models.Group{Name: "zebra"}
		config.Cfg.Groups["alpha"] = models.Group{Name: "alpha"}
		config.Cfg.Groups["middle"] = models.Group{Name: "middle"}

		groups := List()
		if len(groups) != 3 {
			t.Fatalf("got %d groups, want 3", len(groups))
		}
		if groups[0].Name != "alpha" || groups[1].Name != "middle" || groups[2].Name != "zebra" {
			t.Errorf("groups not sorted: %v", groups)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		setupTestConfig()
		groups := List()
		if len(groups) != 0 {
			t.Errorf("got %d groups, want 0", len(groups))
		}
	})
}

func TestGroupsContainingCommand(t *testing.T) {
	t.Run("command in multiple groups", func(t *testing.T) {
		setupTestConfig()
		config.Cfg.Groups["deploy"] = models.Group{Name: "deploy", CommandIDs: []string{"1", "2"}}
		config.Cfg.Groups["ci"] = models.Group{Name: "ci", CommandIDs: []string{"1", "3"}}
		config.Cfg.Groups["other"] = models.Group{Name: "other", CommandIDs: []string{"3"}}

		names := GroupsContainingCommand("1")
		if len(names) != 2 {
			t.Fatalf("got %d groups, want 2", len(names))
		}
		if names[0] != "ci" || names[1] != "deploy" {
			t.Errorf("got %v, want [ci deploy]", names)
		}
	})

	t.Run("command in no groups", func(t *testing.T) {
		setupTestConfig()
		config.Cfg.Groups["deploy"] = models.Group{Name: "deploy", CommandIDs: []string{"2"}}

		names := GroupsContainingCommand("1")
		if len(names) != 0 {
			t.Errorf("got %d groups, want 0", len(names))
		}
	})

	t.Run("empty groups", func(t *testing.T) {
		setupTestConfig()
		names := GroupsContainingCommand("1")
		if len(names) != 0 {
			t.Errorf("got %d groups, want 0", len(names))
		}
	})
}
