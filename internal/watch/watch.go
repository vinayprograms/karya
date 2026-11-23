// Package watch wraps fsnotify for live-reloading TUIs: it watches
// directories and delivers a debounced Changed message via Bubble Tea.
package watch

import (
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"
)

// Changed is the message delivered when any watched file is written,
// created, or removed.
type Changed struct{}

// Dirs creates a watcher monitoring the given directories. Directories
// that cannot be watched (missing, permissions) are skipped silently, so
// callers can pass optimistic lists.
func Dirs(paths ...string) (*fsnotify.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	Add(w, paths...)
	return w, nil
}

// Add registers more directories on an existing watcher, skipping failures.
// fsnotify tolerates duplicate adds, so callers may re-add freely.
func Add(w *fsnotify.Watcher, paths ...string) {
	if w == nil {
		return
	}
	for _, p := range paths {
		_ = w.Add(p)
	}
}

// Wait returns a command that blocks until a relevant file event occurs,
// then emits Changed. Writes are debounced briefly so bursts of events
// collapse into one message. Watcher errors are logged and watching
// continues.
func Wait(w *fsnotify.Watcher) tea.Cmd {
	return func() tea.Msg {
		if w == nil {
			return nil
		}
		for {
			select {
			case event, ok := <-w.Events:
				if !ok {
					return nil
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
					time.Sleep(100 * time.Millisecond)
					return Changed{}
				}
			case err, ok := <-w.Errors:
				if !ok {
					return nil
				}
				log.Printf("watch: %v", err)
			}
		}
	}
}
