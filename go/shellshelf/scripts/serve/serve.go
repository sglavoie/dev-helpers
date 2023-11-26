package main

import (
	"sync"

	"github.com/sglavoie/dev-helpers/go/shellshelf/pkg/osutils"
)

func main() {
	var wg sync.WaitGroup

	commands := []string{"air serve", "npx tailwindcss -i web/styles.css -o web/static/styles.css --watch"}

	for _, cmdStr := range commands {
		wg.Add(1)
		go func(command string) {
			defer wg.Done()

			osutils.ExecShellCommand(command)
		}(cmdStr)
	}

	wg.Wait()
}
