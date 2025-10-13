package main

import (
	"log"
	"os"
	"os/exec"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: tickle <deadline> <task>")
	}
	deadline := os.Args[1]
	task := strings.Join(os.Args[2:], " ")

	// Call inbox wait:"deadline" "task"
	cmd := exec.Command("inbox", "wait:"+deadline, task)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}