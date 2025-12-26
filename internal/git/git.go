package git

import (
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// IsGitRepo checks if the given path is inside a git repository
func IsGitRepo(path string) bool {
	_, err := FindRepoRoot(path)
	return err == nil
}

// FindRepoRoot finds the root of the git repository containing the given path
func FindRepoRoot(path string) (string, error) {
	// Walk up the directory tree looking for .git
	current := path
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}

	for {
		gitDir := filepath.Join(current, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return current, nil
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			return "", os.ErrNotExist
		}
		current = parent
	}
}

// CommitFile stages and commits a single file with the given message.
// If push is true and remotes exist, it will also push to the remote.
// Returns nil if the file is not in a git repo.
func CommitFile(filePath, message string, push bool) error {
	repoRoot, err := FindRepoRoot(filePath)
	if err != nil {
		// Not a git repo, silently return
		return nil
	}

	// Open the repository
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return err
	}

	// Get the working tree
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		return err
	}

	// Stage the file
	if _, err := w.Add(relPath); err != nil {
		return err
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
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Karya",
			Email: "karya@local",
		},
	})
	if err != nil {
		return err
	}

	if !push {
		return nil
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

// CommitFiles stages and commits multiple files with the given message.
// If push is true and remotes exist, it will also push to the remote.
// All files must be in the same repository.
func CommitFiles(filePaths []string, message string, push bool) error {
	if len(filePaths) == 0 {
		return nil
	}

	repoRoot, err := FindRepoRoot(filePaths[0])
	if err != nil {
		// Not a git repo, silently return
		return nil
	}

	// Open the repository
	repo, err := git.PlainOpen(repoRoot)
	if err != nil {
		return err
	}

	// Get the working tree
	w, err := repo.Worktree()
	if err != nil {
		return err
	}

	// Stage all files
	for _, filePath := range filePaths {
		relPath, err := filepath.Rel(repoRoot, filePath)
		if err != nil {
			continue
		}
		if _, err := w.Add(relPath); err != nil {
			// Continue with other files if one fails
			continue
		}
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
	_, err = w.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Karya",
			Email: "karya@local",
		},
	})
	if err != nil {
		return err
	}

	if !push {
		return nil
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

// Init initializes a git repository at the given path
func Init(path string) error {
	_, err := git.PlainInit(path, false)
	return err
}

// InitAndCommit initializes a git repository and makes an initial commit
func InitAndCommit(path, message string) error {
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
