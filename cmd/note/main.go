package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("ERROR: The first argument must be a project name")
	}
	prj := os.Args[1]
	args := os.Args[2:]

	prjDir := os.Getenv("PRJDIR")
	if prjDir == "" {
		log.Fatal("PRJDIR not set")
	}

	if len(args) > 0 && args[0] == "count" {
		zetDir := filepath.Join(prjDir, prj, "notes")
		os.Setenv("ZETDIR", zetDir)
		cmd := exec.Command("zet", args...)
		cmd.Stdout = os.Stdout
		cmd.Run()
		return
	}

	if err := checkPrjDir(prjDir, prj); err != nil {
		log.Fatal(err)
	}
	if err := checkNotesDir(prjDir, prj); err != nil {
		log.Fatal(err)
	}

	zetDir := filepath.Join(prjDir, prj, "notes")
	os.Setenv("ZETDIR", zetDir)
	cmd := exec.Command("zet", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func checkPrjDir(prjDir, prj string) error {
	prjPath := filepath.Join(prjDir, prj)
	if _, err := os.Stat(prjPath); os.IsNotExist(err) {
		fmt.Printf("Project directory - '%s', doesn't exist. Create [Y/n]? ", prjPath)
		var input string
		fmt.Scanln(&input)
		if input == "Y" || input == "" {
			return os.MkdirAll(prjPath, 0755)
		} else {
			return fmt.Errorf("Cannot capture notes! Exiting.")
		}
	}
	return nil
}

func checkNotesDir(prjDir, prj string) error {
	notesPath := filepath.Join(prjDir, prj, "notes")
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		fmt.Printf("Notes directory '%s' doesn't exist. Create (Y/n)? ", notesPath)
		var input string
		fmt.Scanln(&input)
		if input == "Y" || input == "" {
			if err := os.MkdirAll(notesPath, 0755); err != nil {
				return err
			}
			return exec.Command("git", "init").Run()
		} else {
			return fmt.Errorf("Cannot capture notes! Exiting.")
		}
	}
	return nil
}