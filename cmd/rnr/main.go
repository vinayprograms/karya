package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	prjDir := os.Getenv("PRJDIR")
	if prjDir == "" {
		log.Fatal("PRJDIR not set")
	}
	readingList := filepath.Join(prjDir, "reading-list.md")

	prerequisitesCheck(readingList)

	if len(os.Args) == 1 {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			log.Fatal("EDITOR not set")
		}
		cmd := exec.Command(editor, readingList)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	} else {
		readNReview(os.Args[1], readingList)
	}
}

func prerequisitesCheck(readingList string) {
	// Check if file exists, create if not
	if _, err := os.Stat(readingList); os.IsNotExist(err) {
		file, err := os.Create(readingList)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		fmt.Fprintln(file, "# Reading List")
		fmt.Fprintln(file, "")
	}
}

func webpageTitle(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return url
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return url
	}
	re := regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return url
}

func readNReview(link, file string) {
	title := webpageTitle(link)
	line := fmt.Sprintf("- [%s](%s)", title, link)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintln(f, line)
}