package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	sshconfig "github.com/kevinburke/ssh_config"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// SSHAuthForRepo reads the repo's remote URL, extracts the hostname,
// and resolves the correct SSH private key via ~/.ssh/config.
// Falls back to scanning ~/.ssh/ for any private key if no matching Host entry is found.
func SSHAuthForRepo(repo *git.Repository) (*ssh.PublicKeys, error) {
	user := "git"
	keyPath := ""

	// Get remote URL to extract hostname
	remotes, err := repo.Remotes()
	if err != nil || len(remotes) == 0 {
		return nil, fmt.Errorf("no remotes found")
	}

	// Use first remote (typically "origin")
	remoteURL := remotes[0].Config().URLs[0]
	host := extractSSHHost(remoteURL)

	// Parse ~/.ssh/config to find IdentityFile for this host
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home dir: %w", err)
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	if f, err := os.Open(sshConfigPath); err == nil {
		if cfg, err := sshconfig.Decode(f); err == nil {
			if identityFile, err := cfg.Get(host, "IdentityFile"); err == nil && identityFile != "" {
				keyPath = expandPath(identityFile, home)
			}
			if u, err := cfg.Get(host, "User"); err == nil && u != "" {
				user = u
			}
		}
		f.Close()
	}

	// Fallback: scan ~/.ssh/ for any private key if no match in ssh config.
	// Try common names first, then fall back to directory scan.
	if keyPath == "" {
		sshDir := filepath.Join(home, ".ssh")
		// Common key type names in order of preference
		for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa", "id_dsa"} {
			candidate := filepath.Join(sshDir, name)
			if _, err := os.Stat(candidate); err == nil {
				keyPath = candidate
				break
			}
		}
		// Last resort: scan ~/.ssh/ for any file that looks like a private key
		if keyPath == "" {
			entries, _ := os.ReadDir(sshDir)
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				// Skip .pub files, config, known_hosts, authorized_keys, agent files
				name := entry.Name()
				if strings.HasSuffix(name, ".pub") || strings.HasPrefix(name, "known_") ||
					name == "config" || strings.Contains(name, "authorized") {
					continue
				}
				// Check if file looks like a private key
				path := filepath.Join(sshDir, name)
				if data, err := os.ReadFile(path); err == nil {
					content := string(data)
					if strings.Contains(content, "PRIVATE KEY") {
						keyPath = path
						break
					}
				}
			}
		}
	}

	if keyPath == "" {
		return nil, fmt.Errorf("no SSH key found for host %s", host)
	}

	auth, err := ssh.NewPublicKeysFromFile(user, keyPath, "")
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key %s: %w", keyPath, err)
	}

	return auth, nil
}

// extractSSHHost parses an SSH remote URL like "git@github.com:org/repo.git"
// and returns the hostname ("github.com"). Returns empty for non-SSH URLs.
func extractSSHHost(url string) string {
	// SSH format: git@github.com:org/repo.git
	if strings.Contains(url, "@") && strings.Contains(url, ":") {
		atIdx := strings.Index(url, "@")
		colonIdx := strings.Index(url[atIdx:], ":")
		if atIdx != -1 && colonIdx != -1 {
			return url[atIdx+1 : atIdx+colonIdx]
		}
	}
	// SSH format: ssh://git@github.com:22/org/repo.git
	if strings.HasPrefix(url, "ssh://") {
		rest := strings.TrimPrefix(url, "ssh://")
		if atIdx := strings.Index(rest, "@"); atIdx != -1 {
			rest = rest[atIdx+1:]
			if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
				return rest[:colonIdx]
			}
			if slashIdx := strings.Index(rest, "/"); slashIdx != -1 {
				return rest[:slashIdx]
			}
		}
	}
	return ""
}

// expandPath expands ~ and $HOME in a path, relative to home.
func expandPath(p string, home string) string {
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	if strings.HasPrefix(p, "$HOME/") {
		return filepath.Join(home, p[5:])
	}
	if !filepath.IsAbs(p) {
		return filepath.Join(home, ".ssh", p)
	}
	return p
}

// GetSignature returns a git signature using the user's configured git identity.
// It checks local repo config first, then global config, falling back to defaults.
func GetSignature(repo *git.Repository) *object.Signature {
	name := "Karya"
	email := "karya@local"

	// Try local repo config first
	if repo != nil {
		if cfg, err := repo.Config(); err == nil {
			if cfg.User.Name != "" {
				name = cfg.User.Name
			}
			if cfg.User.Email != "" {
				email = cfg.User.Email
			}
			// If we got both from local config, we're done
			if cfg.User.Name != "" && cfg.User.Email != "" {
				return &object.Signature{
					Name:  name,
					Email: email,
					When:  time.Now(),
				}
			}
		}
	}

	// Try global config
	if globalCfg, err := config.LoadConfig(config.GlobalScope); err == nil {
		if globalCfg.User.Name != "" && name == "Karya" {
			name = globalCfg.User.Name
		}
		if globalCfg.User.Email != "" && email == "karya@local" {
			email = globalCfg.User.Email
		}
	}

	return &object.Signature{
		Name:  name,
		Email: email,
		When:  time.Now(),
	}
}

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
		Author: GetSignature(repo),
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
	auth, authErr := SSHAuthForRepo(repo)
	if authErr != nil {
		return nil // silently skip push if auth can't be resolved
	}
	err = repo.Push(&git.PushOptions{Auth: auth})
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
		Author: GetSignature(repo),
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
	auth, authErr := SSHAuthForRepo(repo)
	if authErr != nil {
		return nil // silently skip push if auth can't be resolved
	}
	err = repo.Push(&git.PushOptions{Auth: auth})
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
		Author: GetSignature(repo),
	})

	return err
}
