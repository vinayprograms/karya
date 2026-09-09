package task

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vinayprograms/karya/internal/config"
)

// maxWatchDirs limits the number of directories to watch in unstructured
// mode to avoid exhausting file descriptors. Structured mode has no cap
// since it only watches zettel directories (bounded by actual note count).
const maxWatchDirs = 1000

// WatchDirs returns the directories a live-reloading task view should
// watch. In structured mode it covers the project/notes/zettel tree; in
// unstructured mode it walks the project tree, preferring shallower
// directories up to a fixed cap. The inbox directory is always included.
func WatchDirs(cfg *config.Config, project string) []string {
	var dirs []string
	if cfg.Todo.Structured {
		dirs = structuredWatchDirs(cfg, project)
	} else {
		dirs = unstructuredWatchDirs(cfg, project)
	}
	if inboxPath := cfg.GetInboxFilePath(); inboxPath != "" {
		dirs = append(dirs, filepath.Dir(inboxPath))
	}
	return dirs
}

// structuredWatchDirs covers PRJDIR, PRJDIR/*, PRJDIR/*/notes and
// PRJDIR/*/notes/* for the given project ("" or "*" means all).
func structuredWatchDirs(cfg *config.Config, project string) []string {
	var dirs []string
	prjDir := cfg.Directories.Projects

	addProject := func(projectDir string) {
		notesDir := filepath.Join(projectDir, "notes")
		dirs = append(dirs, projectDir, notesDir)
		entries, err := os.ReadDir(notesDir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, filepath.Join(notesDir, e.Name()))
			}
		}
	}

	if project != "" && project != "*" {
		addProject(filepath.Join(prjDir, project))
		return dirs
	}

	dirs = append(dirs, prjDir)
	entries, err := os.ReadDir(prjDir)
	if err != nil {
		return dirs
	}
	for _, e := range entries {
		if e.IsDir() {
			addProject(filepath.Join(prjDir, e.Name()))
		}
	}
	return dirs
}

// unstructuredWatchDirs walks the project tree and returns up to
// maxWatchDirs directories, shallower ones first.
func unstructuredWatchDirs(cfg *config.Config, project string) []string {
	rootDir := cfg.Directories.Projects
	if project != "" && project != "*" {
		rootDir = filepath.Join(cfg.Directories.Projects, project)
	}

	rootDepth := strings.Count(rootDir, string(filepath.Separator))
	type dirInfo struct {
		path  string
		depth int
	}
	var all []dirInfo

	filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			all = append(all, dirInfo{path, strings.Count(path, string(filepath.Separator)) - rootDepth})
		}
		return nil
	})

	sort.Slice(all, func(i, j int) bool { return all[i].depth < all[j].depth })

	var dirs []string
	for i, d := range all {
		if i >= maxWatchDirs {
			break
		}
		dirs = append(dirs, d.path)
	}
	return dirs
}
