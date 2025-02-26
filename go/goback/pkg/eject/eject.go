package eject

import (
	"log"
	"os/exec"

	"github.com/spf13/viper"
)

func Eject() {
	dest := viper.GetString("destination")
	if dest == "" {
		log.Fatal("destination not set")
	}

	cmd := exec.Command("diskutil", "unmount", dest)
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	log.Println("Disk ejected")
}
