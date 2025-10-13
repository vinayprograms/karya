package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("ERROR: The first argument must be a project name")
	}
	prj := os.Args[1]
	thot := strings.Join(os.Args[2:], " ")

	prjDir := os.Getenv("PRJDIR")
	if prjDir == "" {
		log.Fatal("PRJDIR not set")
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = pickEditor()
		if editor == "" {
			log.Fatal("No editor available")
		}
		os.Setenv("EDITOR", editor)
	}

	if err := checkAndCreateDirectory(prjDir, prj); err != nil {
		log.Fatal(err)
	}

	if err := captureThot(prjDir, prj, thot); err != nil {
		log.Fatal(err)
	}
}

func pickEditor() string {
	editors := []string{"vim", "emacs", "nano"}
	for _, ed := range editors {
		if _, err := exec.LookPath(ed); err == nil {
			return ed
		}
	}
	fmt.Print("No editor configured. Pick one - (vim, emacs, nano): ")
	var choice string
	fmt.Scanln(&choice)
	if choice == "" {
		choice = "vim"
	}
	return choice
}

func checkAndCreateDirectory(prjDir, prj string) error {
	prjPath := filepath.Join(prjDir, prj)
	if _, err := os.Stat(prjPath); os.IsNotExist(err) {
		fmt.Printf("Project directory doesn't exist. Create (Y/n)? ")
		var input string
		fmt.Scanln(&input)
		if input == "Y" || input == "" {
			if err := os.MkdirAll(prjPath, 0755); err != nil {
				return err
			}
			// git init
			if err := exec.Command("git", "init").Run(); err == nil {
				exec.Command("git", "branch", "-m", "master", "main").Run()
			}
			thotFile := filepath.Join(prjPath, "thots.md")
			file, err := os.Create(thotFile)
			if err != nil {
				return err
			}
			defer file.Close()
			fmt.Fprintf(file, "# Thoughts - %s\n\n", prj)
			// git add and commit
			if err := exec.Command("git", "-C", prjPath, "add", ".").Run(); err == nil {
				exec.Command("git", "-C", prjPath, "commit", "-m", "New project - "+prj).Run()
			}
		} else {
			return fmt.Errorf("Cannot capture your thought! Exiting.")
		}
	}
	return nil
}

func captureThot(prjDir, prj, thot string) error {
	thotFile := filepath.Join(prjDir, prj, "thots.md")
	if thot == "" {
		// Open editor
		editor := os.Getenv("EDITOR")
		cmd := exec.Command(editor, thotFile)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else {
		// Append
		file, err := os.OpenFile(thotFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		fmt.Fprintf(file, "* %s\n", thot)
	}
	// git add and commit
	prjPath := filepath.Join(prjDir, prj)
	if _, err := os.Stat(filepath.Join(prjPath, ".git")); err == nil {
		exec.Command("git", "-C", prjPath, "add", ".").Run()
		exec.Command("git", "-C", prjPath, "commit", "-m", "New thought in "+prj).Run()
	}
	return nil
}