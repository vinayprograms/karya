package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckInbox(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "inbox_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	inboxFile := filepath.Join(tempDir, "inbox.md")
	if err := checkInbox(inboxFile); err != nil {
		t.Fatal(err)
	}

	content, err := ioutil.ReadFile(inboxFile)
	if err != nil {
		t.Fatal(err)
	}
	expected := "# INBOX\n\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestAppendToInbox(t *testing.T) {
	tempDir, err := ioutil.TempDir("", "inbox_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	inboxFile := filepath.Join(tempDir, "inbox.md")
	if err := checkInbox(inboxFile); err != nil {
		t.Fatal(err)
	}

	// Simulate append
	content := "- test item\n"
	file, err := os.OpenFile(inboxFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	file.WriteString(content)
	file.Close()

	finalContent, err := ioutil.ReadFile(inboxFile)
	if err != nil {
		t.Fatal(err)
	}
	expected := "# INBOX\n\n- test item\n"
	if string(finalContent) != expected {
		t.Errorf("Expected %q, got %q", expected, string(finalContent))
	}
}