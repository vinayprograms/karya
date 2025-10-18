package zet

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GitCommit(zetDir, zetID, title string) error {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	zetPath := filepath.Join(zetID, "README.md")
	readmePath := "README.md"

	cmd := exec.Command("git", "-C", zetDir, "add", zetPath, readmePath)
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "-C", zetDir, "commit", "-m", title)
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "-C", zetDir, "remote")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		fmt.Println("No remotes for this repository")
		return nil
	}

	cmd = exec.Command("git", "-C", zetDir, "push")
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func GitDeleteZettel(zetDir, zetID, title string) error {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command("git", "-C", zetDir, "rm", "-rf", zetID)
	if err := cmd.Run(); err != nil {
		cmd = exec.Command("git", "-C", zetDir, "add", "README.md")
		cmd.Run()
	} else {
		cmd = exec.Command("git", "-C", zetDir, "add", "README.md")
		cmd.Run()
	}

	commitMsg := fmt.Sprintf("Delete zettel '%s'", title)
	cmd = exec.Command("git", "-C", zetDir, "commit", "-m", commitMsg)
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("git", "-C", zetDir, "remote")
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		return nil
	}

	cmd = exec.Command("git", "-C", zetDir, "push")
	return cmd.Run()
}

func GetLastZettelID(zetDir string) (string, error) {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", fmt.Errorf("not a git repository")
	}

	cmd := exec.Command("git", "-C", zetDir, "log", "--pretty=format:%h", "-n", "1")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(output))

	cmd = exec.Command("git", "-C", zetDir, "show", "--name-only", "--pretty=", commit)
	output, err = cmd.Output()
	if err != nil {
		return "", err
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(files) == 0 {
		return "", fmt.Errorf("no files in last commit")
	}

	zetID := ""
	for _, file := range files {
		if strings.Contains(file, "/") {
			parts := strings.Split(file, "/")
			if len(parts) > 0 && IsValidZettelID(parts[0]) {
				zetID = parts[0]
				break
			}
		}
	}

	if zetID == "" {
		return "", fmt.Errorf("could not determine zettel ID from last commit")
	}

	return zetID, nil
}
