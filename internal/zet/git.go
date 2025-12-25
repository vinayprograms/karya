package zet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func GitCommit(zetDir, zetID, title string) error {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	// Open the repository
	repo, err := git.PlainOpen(zetDir)
	if err != nil {
		return err
	}

	// Get the working tree
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Add files
	zetPath := filepath.Join(zetID, "README.md")
	if _, err := w.Add(zetPath); err != nil {
		return err
	}
	if _, err := w.Add("README.md"); err != nil {
		// Ignore error if README.md doesn't exist
	}

	// Check if there are any changes to commit
	status, err := w.Status()
	if err != nil {
		return err
	}

	if status.IsClean() {
		return nil
	}

	// Commit the changes
	_, err = w.Commit(title, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Karya",
			Email: "karya@local",
		},
	})
	if err != nil {
		return err
	}

	// Check for remotes
	remotes, err := repo.Remotes()
	if err != nil || len(remotes) == 0 {
		return nil
	}

	// Push to remote
	err = repo.Push(&git.PushOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func GitDeleteZettel(zetDir, zetID, title string) error {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil
	}

	// Open the repository
	repo, err := git.PlainOpen(zetDir)
	if err != nil {
		return err
	}

	// Get the working tree
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Remove the zettel directory if it still exists
	zetPath := filepath.Join(zetDir, zetID)
	if _, err := os.Stat(zetPath); err == nil {
		if err := os.RemoveAll(zetPath); err != nil {
			return err
		}
	}

	// Stage the deletion - use Add with glob to stage all changes
	// This works whether files were deleted before or by us
	if _, err := w.Add(zetID); err != nil {
		// Zettel dir already deleted, stage via status
		status, err := w.Status()
		if err != nil {
			return err
		}
		for path := range status {
			w.Add(path)
		}
	}
	w.Add("README.md")

	// Commit the deletion
	commitMsg := fmt.Sprintf("Delete zettel '%s'", title)
	_, err = w.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Karya",
			Email: "karya@local",
		},
	})
	if err != nil {
		return err
	}

	// Check for remotes
	remotes, err := repo.Remotes()
	if err != nil || len(remotes) == 0 {
		return nil
	}

	// Push to remote
	err = repo.Push(&git.PushOptions{})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return err
	}

	return nil
}

func GetLastZettelID(zetDir string) (string, error) {
	gitDir := filepath.Join(zetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return "", fmt.Errorf("not a git repository")
	}

	// Open the repository
	repo, err := git.PlainOpen(zetDir)
	if err != nil {
		return "", err
	}

	// Iterate through commits to find the last one that touched a zettel
	commitIter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return "", err
	}
	defer commitIter.Close()

	var zetID string
	err = commitIter.ForEach(func(commit *object.Commit) error {
		// Get the diff for this commit
		stats, err := commit.Stats()
		if err != nil {
			return nil // Skip this commit, try next
		}

		// Check each changed file
		for _, stat := range stats {
			parts := strings.Split(stat.Name, "/")
			if len(parts) > 0 && IsValidZettelID(parts[0]) {
				zetID = parts[0]
				return fmt.Errorf("found") // Stop iteration
			}
		}
		return nil
	})

	// "found" error is expected - it's how we stop iteration
	if err != nil && err.Error() != "found" {
		return "", err
	}

	if zetID == "" {
		return "", fmt.Errorf("could not determine zettel ID from git history")
	}

	return zetID, nil
}

// GitInit initializes a git repository at the given path
func GitInit(path string) error {
	_, err := git.PlainInit(path, false)
	return err
}

// GitInitAndCommit initializes a git repository and makes an initial commit
func GitInitAndCommit(path, message string) error {
	// Initialize repository
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return err
	}

	// Get the working tree
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Add all files
	if _, err := w.Add("."); err != nil {
		return err
	}

	// Make initial commit
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Karya",
			Email: "karya@local",
		},
	})

	return err
}
