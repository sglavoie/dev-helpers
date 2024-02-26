package server

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/commands"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/config"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
)

// IndexHandler creates a new HTTP handler function with access to the given Config.
func IndexHandler(w http.ResponseWriter, r *http.Request, cfg models.Config) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "index.html", cfg.Commands)
}

func CommandAddButtonHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-add", nil)
}

func CommandShowActionRowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-action-row", nil)
}

func CommandAddSaveHandler(w http.ResponseWriter, r *http.Request, cfg models.Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	currentMaxID, err := commands.GetMaxID(cfg.Commands)
	if err != nil {
		return
	}
	newMaxID := strconv.Itoa(currentMaxID + 1)
	newCmd := models.Command{
		Id:          newMaxID,
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Command:     r.FormValue("command"),
	}

	fmt.Println("newCmd:", newCmd)
	cfg.Commands[newMaxID] = newCmd

	err = config.SaveEncodedCommands(cfg)
	if err != nil {
		log.Println("Error saving commands:", err)
		return
	}

	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-add", nil)
}

func CommandAddValidateNameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.FormValue("name")
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 {
		tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
		ExecuteTemplate(w, tmpl, "commands-add-name-invalid", name)
		return
	}

	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-add-name-valid", name)
}

func CommandEditHandler(w http.ResponseWriter, r *http.Request, cfg models.Config) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/command/edit/")
	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-row-edit", cfg.Commands[commandID])
}

func CommandGetHandler(w http.ResponseWriter, r *http.Request, cfg models.Config) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/command/get/")
	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-row", cfg.Commands[commandID])
}

func CommandRemoveHandler(w http.ResponseWriter, r *http.Request, cfg models.Config) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/command/remove/")
	cfg.Commands = commands.RemoveById(cfg.Commands, commandID)

	err := config.SaveEncodedCommands(cfg)
	if err != nil {
		log.Println("Error saving commands:", err)
		return
	}
}

func CommandSaveHandler(w http.ResponseWriter, r *http.Request, cfg models.Config) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	commandID := strings.TrimPrefix(r.URL.Path, "/command/save/")
	updatedCmd := models.Command{
		Id:          commandID,
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Command:     r.FormValue("command"),
	}
	cfg.Commands = GetUpdateCommand(cfg.Commands, updatedCmd)

	err := config.SaveEncodedCommands(cfg)
	if err != nil {
		log.Println("Error saving commands:", err)
		return
	}

	cmd := cfg.Commands[commandID]
	cmd.Command, err = commands.Decode(cmd.Command)
	if err != nil {
		log.Println("Error decoding command:", err)
		return
	}

	tmpl := template.Must(template.ParseGlob("web/templates/*.html"))
	ExecuteTemplate(w, tmpl, "commands-row", cmd)
}

// WithConfig loads the configuration from the file as a value and returns a closure for handling HTTP requests.
func WithConfig(handler func(http.ResponseWriter, *http.Request, models.Config)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg, err := config.LoadConfigAsValue()
		if err != nil {
			log.Println("Error loading decoded config:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		cfg.Commands, err = commands.LoadDecoded(cfg.Commands)
		if err != nil {
			log.Println("Error decoding commands:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		handler(w, r, cfg)
	}
}
