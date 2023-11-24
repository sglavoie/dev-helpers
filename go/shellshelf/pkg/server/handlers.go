package server

import (
	"fmt"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/models"
	"html/template"
	"net/http"
)

// IndexHandler creates a new HTTP handler function with access to the given Config.
func IndexHandler(cfg *models.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tmpl := template.Must(template.ParseFiles("web/templates/index.html"))
		err := tmpl.Execute(w, cfg.Commands)
		if err != nil {
			fmt.Println("Error executing template:", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}
}
