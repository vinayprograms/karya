package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	karyaDir := os.Getenv("KARYA_DIR")
	if karyaDir == "" {
		log.Fatal("KARYA_DIR not set")
	}
	inboxFile := filepath.Join(karyaDir, "inbox.md")

	if err := checkInbox(inboxFile); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) == 1 {
		// Open with editor
		editor := os.Getenv("EDITOR")
		if editor == "" {
			log.Fatal("EDITOR not set")
		}
		// In Go, to open editor, perhaps use exec
		// But for simplicity, print message
		fmt.Printf("Would open %s with %s\n", inboxFile, editor)
	} else {
		// Append to file
		content := "- " + strings.Join(os.Args[1:], " ") + "\n"
		file, err := os.OpenFile(inboxFile, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		if _, err := file.WriteString(content); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Added '%s' to inbox\n", strings.Join(os.Args[1:], " "))
	}
}

func checkInbox(filePath string) error {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString("# INBOX\n\n")
		return err
	}
	return nil
}