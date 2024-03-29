package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/clihelpers"
	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/server"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run a server to access the shelf via a browser UI",
	Long:  `Run a server to access the shelf via a browser UI.`,
	Run: func(cmd *cobra.Command, args []string) {
		openBrowser, err := cmd.Flags().GetBool("browser")
		if err != nil {
			clihelpers.FatalExit("Error reading 'browser' flag: %v", err)
		}

		startServer(openBrowser)
	},
}

func browserOpener(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("cmd", "/c", "start", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		return err
	}

	return nil
}

func startServer(openBrowser bool) {
	staticFileHandler := http.FileServer(http.Dir("web/static"))
	http.Handle("/static/", http.StripPrefix("/static/", staticFileHandler))
	http.HandleFunc("/", server.WithConfig(server.IndexHandler))
	http.HandleFunc("/command/add/", server.CommandAddButtonHandler)
	http.HandleFunc("/command/showActionRow/", server.CommandShowActionRowHandler)
	http.HandleFunc("/command/add/validate-name/", server.CommandAddValidateNameHandler)
	http.HandleFunc("/command/add/save/", server.WithConfig(server.CommandAddSaveHandler))
	http.HandleFunc("/command/edit/", server.WithConfig(server.CommandEditHandler))
	http.HandleFunc("/command/get/", server.WithConfig(server.CommandGetHandler))
	http.HandleFunc("/command/remove/", server.WithConfig(server.CommandRemoveHandler))
	http.HandleFunc("/command/save/", server.WithConfig(server.CommandSaveHandler))

	port := ":8080"
	url := "http://localhost" + port
	go func() {
		if openBrowser {
			err := browserOpener(url)
			if err != nil {
				log.Printf("Failed to open browser: %v", err)
			}
		}
	}()

	log.Printf("Starting server at %s\n", url)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().BoolP("browser", "b", false, "Open the default web browser to the server's URL")
}
