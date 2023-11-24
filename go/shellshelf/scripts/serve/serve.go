package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func main() {
	var wg sync.WaitGroup

	commands := []string{"air serve", "npx tailwindcss -i web/styles.css -o web/static/styles.css --watch"}

	for _, cmdStr := range commands {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()

			cmdArgs := strings.Fields(command)

			cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			if err := cmd.Run(); err != nil {
				log.Printf("Error running %s: %v", command, err)
			}
		}(cmdStr)
	}

	wg.Wait()
}
