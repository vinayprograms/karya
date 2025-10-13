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

	if err := captureGoal(prjDir, prj, editor); err != nil {
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
		choice = "vim" // default
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
			goalFile := filepath.Join(prjPath, "goals.md")
			file, err := os.Create(goalFile)
			if err != nil {
				return err
			}
			defer file.Close()
			fmt.Fprintf(file, "# Goals - %s\n\n", prj)
			// git add and commit
			if err := exec.Command("git", "-C", prjPath, "add", ".").Run(); err == nil {
				exec.Command("git", "-C", prjPath, "commit", "-m", "New project - "+prj).Run()
			}
		} else {
			return fmt.Errorf("Cannot capture your goals! Exiting.")
		}
	}
	return nil
}

func captureGoal(prjDir, prj, editor string) error {
	goalFile := filepath.Join(prjDir, prj, "goals.md")
	cmd := exec.Command(editor, goalFile)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	// git add and commit
	prjPath := filepath.Join(prjDir, prj)
	if _, err := os.Stat(filepath.Join(prjPath, ".git")); err == nil {
		exec.Command("git", "-C", prjPath, "add", ".").Run()
		exec.Command("git", "-C", prjPath, "commit", "-m", "New thought in "+prj).Run()
	}
	return nil
}