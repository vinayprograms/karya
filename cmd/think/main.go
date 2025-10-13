package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	// tickle friday "$@"
	args := append([]string{"friday"}, os.Args[1:]...)
	cmd := exec.Command("tickle", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}