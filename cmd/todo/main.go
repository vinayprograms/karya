package main

import (
	"fmt"
	"log"
	"os"
	"sort"

	"karya/internal/task"
)

func main() {
	config := task.NewConfig()
	if len(os.Args) == 1 {
		// Show pending tasks - for now, just list
		tasks, err := config.ListTasks("", true)
		if err != nil {
			log.Fatal(err)
		}
		printTasks(tasks)
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "ls", "list":
		if len(os.Args) > 2 {
			// List for specific project
			tasks, err := config.ListTasks(os.Args[2], false)
			if err != nil {
				log.Fatal(err)
			}
			printTasks(tasks)
		} else {
			tasks, err := config.ListTasks("", false)
			if err != nil {
				log.Fatal(err)
			}
			printTasks(tasks)
		}
	case "projects":
		summary, err := config.SummarizeProjects()
		if err != nil {
			log.Fatal(err)
		}
		printProjectsTable(summary)
	case "projlist":
		summary, err := config.SummarizeProjects()
		if err != nil {
			log.Fatal(err)
		}
		printProjectsList(summary)
	default:
		// Project name
		tasks, err := config.ListTasks(subcommand, false)
		if err != nil {
			log.Fatal(err)
		}
		printTasks(tasks)
	}
}

func printTasks(tasks []*task.Task) {
	for _, t := range tasks {
		fmt.Printf("%s %s %s %s", t.Project, t.Zettel, t.Keyword, t.Title)
		if t.Tag != "" {
			fmt.Printf(" #%s", t.Tag)
		}
		if t.Date != "" {
			fmt.Printf(" @%s", t.Date)
		}
		if t.Assignee != "" {
			fmt.Printf(" >> %s", t.Assignee)
		}
		fmt.Println()
	}
}

func printProjectsTable(summary map[string]int) {
	fmt.Println("Project Tasks")
	var projects []string
	maxLen := 0
	for p := range summary {
		projects = append(projects, p)
		if len(p) > maxLen {
			maxLen = len(p)
		}
	}
	sort.Strings(projects)
	for _, p := range projects {
		fmt.Printf("%-*s %d\n", maxLen, p, summary[p])
	}
}

func printProjectsList(summary map[string]int) {
	var projects []string
	for p := range summary {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	for _, p := range projects {
		fmt.Printf("%s %d\n", p, summary[p])
	}
}