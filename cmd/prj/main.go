package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "list" || arg == "ls" {
			listProjects()
			return
		}
		if arg == "show" {
			// For simplicity, just list once
			listProjects()
			return
		}
	}

	project := ""
	if len(os.Args) > 1 {
		project = os.Args[1]
	}

	if project == "none" || project == "" {
		fmt.Println("No project name provided.")
		return
	}

	// Show tasks and notes
	fmt.Println("\n    TASKS    ")
	cmd := exec.Command("todo", project)
	cmd.Stdout = os.Stdout
	cmd.Run()

	fmt.Println("\n   NOTES   ")
	cmd = exec.Command("note", project, "count")
	output, err := cmd.Output()
	if err == nil {
		fmt.Printf("%s notes\n", strings.TrimSpace(string(output)))
	}
	cmd = exec.Command("note", project, "ls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func listProjects() {
	cmd := exec.Command("todo", "projlist")
	output, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var projects []struct {
		name  string
		tasks int
	}
	maxLen := 0
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		name := strings.Join(parts[:len(parts)-1], " ")
		tasks, _ := strconv.Atoi(parts[len(parts)-1])
		projects = append(projects, struct {
			name  string
			tasks int
		}{name, tasks})
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	fmt.Printf("\n%-${maxLen}s  TASKS  NOTES \n", "PROJECT")
	prjCount, totalTasks, totalNotes := 0, 0, 0
	for _, p := range projects {
		// Get note count
		cmd := exec.Command("note", p.name, "count")
		noteOutput, _ := cmd.Output()
		noteCount, _ := strconv.Atoi(strings.TrimSpace(string(noteOutput)))
		fmt.Printf("%-${maxLen}s  %5d %5d\n", p.name, p.tasks, noteCount)
		prjCount++
		totalTasks += p.tasks
		totalNotes += noteCount
	}
	fmt.Printf("\n%d Projects, %d tasks, %d notes\n\n", prjCount, totalTasks, totalNotes)
}