package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) == 1 {
		fmt.Println("List of tasks you are waiting on...")
		cmd := exec.Command("task", "waiting")
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else if len(os.Args) == 2 {
		person := os.Args[1]
		fmt.Printf("Waiting on: %s.\n\n", person)
		cmd := exec.Command("task", "waiting", "owner:"+person)
		cmd.Stdout = os.Stdout
		cmd.Run()
	} else {
		person := os.Args[1]
		taskid := os.Args[2]
		cmd := exec.Command("tasksync", taskid, "modify", "-in", "+waiting", "owner:"+person, "due:+1w", "wait:monday")
		if err := cmd.Run(); err != nil {
			log.Fatal(err)
		}
	}
}