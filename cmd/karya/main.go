package main

import (
	"bufio"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/goal"
	"github.com/vinayprograms/karya/internal/task"
	"github.com/vinayprograms/karya/internal/zet"
)

//go:embed static/index.html
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var (
	clients   = make(map[*websocket.Conn]bool)
	clientsMu sync.Mutex
)

var cfg *config.Config

func main() {
	port := flag.Int("port", 80, "port to listen on")
	bind := flag.String("bind", "127.0.0.1", "address to bind to")
	flag.Parse()

	var err error
	cfg, err = config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", *bind, *port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("port %d is already in use: %v", *port, err)
	}
	ln.Close()

	go watchFilesystem()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("GET /api/tasks", handleTasks)
	mux.HandleFunc("PATCH /api/tasks/{id}/status", handleTaskStatusUpdate)
	mux.HandleFunc("PATCH /api/tasks/{id}/schedule", handleTaskScheduleUpdate)
	mux.HandleFunc("GET /api/agenda", handleAgenda)
	mux.HandleFunc("GET /api/notes", handleNotes)
	mux.HandleFunc("GET /api/notes/{id}", handleNoteByID)
	mux.HandleFunc("PUT /api/notes/{id}", handleNoteSave)
	mux.HandleFunc("GET /api/zettels", handleZettels)
	mux.HandleFunc("GET /api/zettels/{id}", handleZettelByID)
	mux.HandleFunc("PUT /api/zettels/{id}", handleZettelSave)
	mux.HandleFunc("GET /api/goals", handleGoals)
	mux.HandleFunc("GET /api/inbox", handleInbox)
	mux.HandleFunc("POST /api/inbox", handleInboxAdd)
	mux.HandleFunc("GET /api/projects", handleProjects)
	mux.HandleFunc("GET /api/keywords", handleKeywords)
	mux.HandleFunc("GET /ws", handleWebSocket)

	fmt.Printf("karya web → http://%s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func jsonResp(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- Tasks ---

type taskJSON struct {
	ID          string     `json:"id"`
	Project     string     `json:"project"`
	Keyword     string     `json:"keyword"`
	Description string     `json:"description"`
	Tags        []string   `json:"tags"`
	Date        string     `json:"date"`
	DueDate     string     `json:"dueDate,omitempty"`
	Assignee    string     `json:"assignee"`
	Priority    int        `json:"priority"`
	Children    []taskJSON `json:"children"`
	ClockActive bool       `json:"clockActive,omitempty"`
}

func taskToJSON(t *task.Task) taskJSON {
	tags := t.Tags
	if tags == nil {
		tags = []string{}
	}

	date := t.ScheduledAt
	if date != "" {
		if s, err := task.ParseSchedule(date); err == nil {
			date = s.Date.Format("2006-01-02")
		}
	}

	dueDate := t.DueAt
	if dueDate != "" {
		if s, err := task.ParseSchedule(dueDate); err == nil {
			dueDate = s.Date.Format("2006-01-02")
		}
	}

	children := []taskJSON{}
	for _, c := range t.Children {
		children = append(children, taskToJSON(c))
	}

	return taskJSON{
		ID:          taskID(t),
		Project:     t.Project,
		Keyword:     t.Keyword,
		Description: t.Title,
		Tags:        tags,
		Date:        date,
		DueDate:     dueDate,
		Assignee:    t.Assignee,
		Priority:    t.Priority(cfg),
		Children:    children,
		ClockActive: task.IsClockActive(t),
	}
}

func taskID(t *task.Task) string {
	if t.ID != "" {
		return t.ID
	}
	return fmt.Sprintf("%s:%d", t.FilePath, t.LineNum)
}

func findTaskByID(id string) *task.Task {
	tasks, err := task.ListTasks(cfg, "", true)
	if err != nil {
		return nil
	}
	for _, t := range tasks {
		if taskID(t) == id {
			return t
		}
	}
	return nil
}

func handleTasks(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	showCompleted := r.URL.Query().Get("completed") == "true"

	tasks, err := task.ListTasks(cfg, project, showCompleted)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	task.SortByPriority(tasks, cfg)

	rootTasks := task.GroupWithChildren(tasks)

	var result []taskJSON
	for _, t := range rootTasks {
		if t.Parent == nil {
			result = append(result, taskToJSON(t))
		}
	}
	if result == nil {
		result = []taskJSON{}
	}
	jsonResp(w, result)
}

func handleTaskStatusUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Keyword string `json:"keyword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Keyword == "" {
		jsonError(w, "keyword required", 400)
		return
	}

	if !task.IsKeywordValid(cfg, body.Keyword) {
		jsonError(w, "invalid keyword", 400)
		return
	}

	t := findTaskByID(id)
	if t == nil {
		jsonError(w, "task not found", 404)
		return
	}

	if err := task.UpdateTaskStatus(t, body.Keyword, cfg); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResp(w, map[string]string{"status": "ok"})
}

func handleTaskScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		ScheduledAt     string `json:"scheduledAt"`
		DueAt           string `json:"dueAt"`
		RemoveScheduled bool   `json:"removeScheduled"`
		RemoveDue       bool   `json:"removeDue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}

	t := findTaskByID(id)
	if t == nil {
		jsonError(w, "task not found", 404)
		return
	}

	if err := task.SetTaskDate(t, body.ScheduledAt, body.DueAt, body.RemoveScheduled, body.RemoveDue); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResp(w, map[string]string{"status": "ok"})
}

// --- Agenda ---

type agendaItemJSON struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
	Start string `json:"start"`
	End   string `json:"end"`
	Type  string `json:"type"`
}

func handleAgenda(w http.ResponseWriter, r *http.Request) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	now := time.Now()

	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			start = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			end = t
		}
	}

	if start.IsZero() {
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	}
	if end.IsZero() {
		end = time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())
	}

	days, err := task.QueryAgenda(cfg, start, end, true)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	var result []agendaItemJSON
	for _, day := range days {
		dateStr := day.Date.Format("2006-01-02")
		for _, item := range day.Items {
			startTime := ""
			endTime := ""
			if item.HasTime {
				startTime = item.Date.Format("15:04")
			}
			if item.HasEnd {
				endTime = item.EndTime.Format("15:04")
			}

			itemType := "task"
			kw := strings.ToUpper(item.Task.Keyword)
			if kw == "MEETING" || kw == "CALL" {
				itemType = "meeting"
			}
			for _, tag := range item.Task.Tags {
				if tag == "meeting" || tag == "call" {
					itemType = "meeting"
				}
			}

			result = append(result, agendaItemJSON{
				ID:    taskID(item.Task),
				Title: item.Task.Title,
				Date:  dateStr,
				Start: startTime,
				End:   endTime,
				Type:  itemType,
			})
		}
	}
	if result == nil {
		result = []agendaItemJSON{}
	}
	jsonResp(w, result)
}

// --- Notes ---

type noteListItem struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Title    string `json:"title"`
	Modified string `json:"modified"`
	Preview  string `json:"preview"`
}

type noteContent struct {
	ID      string `json:"id"`
	Project string `json:"project"`
	Content string `json:"content"`
}

func handleNotes(w http.ResponseWriter, r *http.Request) {
	projectFilter := r.URL.Query().Get("project")
	projectsDir := cfg.Directories.Projects

	var notes []noteListItem

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectName := entry.Name()
		if projectFilter != "" && projectName != projectFilter {
			continue
		}

		notesDir := filepath.Join(projectsDir, projectName, "notes")
		if _, err := os.Stat(notesDir); err != nil {
			continue
		}

		noteEntries, err := os.ReadDir(notesDir)
		if err != nil {
			continue
		}

		for _, ne := range noteEntries {
			if !ne.IsDir() || !isValidNoteID(ne.Name()) {
				continue
			}
			noteID := ne.Name()
			readmePath := filepath.Join(notesDir, noteID, "README.md")

			title, preview := readNoteMetadata(readmePath)
			modified := ""
			if info, err := os.Stat(readmePath); err == nil {
				modified = info.ModTime().Format("2006-01-02")
			}

			notes = append(notes, noteListItem{
				ID:       projectName + ":" + noteID,
				Project:  projectName,
				Title:    title,
				Modified: modified,
				Preview:  preview,
			})
		}
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].ID > notes[j].ID
	})

	if notes == nil {
		notes = []noteListItem{}
	}
	jsonResp(w, notes)
}

func handleNoteByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, noteID := splitNoteID(id)
	if project == "" || noteID == "" {
		jsonError(w, "invalid note id (expected project/noteID)", 400)
		return
	}

	readmePath := filepath.Join(cfg.Directories.Projects, project, "notes", noteID, "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		jsonError(w, "note not found", 404)
		return
	}

	jsonResp(w, noteContent{
		ID:      id,
		Project: project,
		Content: string(content),
	})
}

func handleNoteSave(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, noteID := splitNoteID(id)
	if project == "" || noteID == "" {
		jsonError(w, "invalid note id", 400)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}

	readmePath := filepath.Join(cfg.Directories.Projects, project, "notes", noteID, "README.md")
	if err := os.WriteFile(readmePath, []byte(body.Content), 0644); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResp(w, map[string]string{"status": "ok"})
}

// --- Zettels ---

type zettelListItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Modified string   `json:"modified"`
	Tags     []string `json:"tags"`
}

func getZetDir() string {
	if cfg.Directories.Zettelkasten != "" {
		return cfg.Directories.Zettelkasten
	}
	if v := os.Getenv("ZETTELKASTEN"); v != "" {
		return v
	}
	return ""
}

func handleZettels(w http.ResponseWriter, r *http.Request) {
	zetDir := getZetDir()
	if zetDir == "" {
		jsonResp(w, []zettelListItem{})
		return
	}

	zettels, err := zet.ListZettels(zetDir)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	var result []zettelListItem
	for _, z := range zettels {
		modified := ""
		readmePath := filepath.Join(zetDir, z.ID, "README.md")
		if info, err := os.Stat(readmePath); err == nil {
			modified = info.ModTime().Format("2006-01-02")
		}

		tags := extractZettelTags(zetDir, z.ID)

		result = append(result, zettelListItem{
			ID:       z.ID,
			Title:    z.Title,
			Modified: modified,
			Tags:     tags,
		})
	}
	if result == nil {
		result = []zettelListItem{}
	}
	jsonResp(w, result)
}

func handleZettelByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	zetDir := getZetDir()
	if zetDir == "" {
		jsonError(w, "zettelkasten not configured", 500)
		return
	}

	content, err := zet.ReadZettelContent(zetDir, id)
	if err != nil {
		jsonError(w, "zettel not found", 404)
		return
	}

	jsonResp(w, map[string]string{
		"id":      id,
		"content": content,
	})
}

func handleZettelSave(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	zetDir := getZetDir()
	if zetDir == "" {
		jsonError(w, "zettelkasten not configured", 500)
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid body", 400)
		return
	}

	if err := zet.WriteZettelContent(zetDir, id, body.Content); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResp(w, map[string]string{"status": "ok"})
}

// --- Goals ---

func handleGoals(w http.ResponseWriter, r *http.Request) {
	goalsDir := getGoalsDir()
	if goalsDir == "" {
		jsonResp(w, map[string]any{})
		return
	}

	manager := goal.NewGoalManager(goalsDir)
	allGoals, err := manager.ListGoals()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	result := make(map[string][]map[string]string)
	horizons := []string{"monthly", "quarterly", "yearly", "short-term", "long-term"}
	for _, h := range horizons {
		horizon := goal.Horizon(h)
		periods, ok := allGoals[horizon]
		if !ok {
			result[h] = []map[string]string{}
			continue
		}
		var items []map[string]string
		for period, titles := range periods {
			for _, title := range titles {
				items = append(items, map[string]string{
					"id":     fmt.Sprintf("%s/%s/%s", h, period, sanitizeGoalID(title)),
					"title":  title,
					"period": period,
					"status": "in_progress",
				})
			}
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i]["period"] > items[j]["period"]
		})
		if items == nil {
			items = []map[string]string{}
		}
		result[h] = items
	}

	jsonResp(w, result)
}

func getGoalsDir() string {
	base := cfg.Directories.Karya
	if base == "" {
		base = cfg.Directories.Projects
	}
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".karya")
	}
	return filepath.Join(base, ".goals")
}

func sanitizeGoalID(title string) string {
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, title)
	return result
}

// --- Inbox ---

func handleInbox(w http.ResponseWriter, r *http.Request) {
	inboxPath := cfg.GetInboxFilePath()

	file, err := os.Open(inboxPath)
	if err != nil {
		if os.IsNotExist(err) {
			jsonResp(w, []map[string]string{})
			return
		}
		jsonError(w, err.Error(), 500)
		return
	}
	defer file.Close()

	var items []map[string]string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		text := trimmed
		for _, prefix := range []string{"- ", "* ", "+ "} {
			if strings.HasPrefix(text, prefix) {
				text = text[len(prefix):]
				break
			}
		}

		items = append(items, map[string]string{
			"id":   fmt.Sprintf("%d", lineNum),
			"text": text,
		})
	}

	if items == nil {
		items = []map[string]string{}
	}
	jsonResp(w, items)
}

func handleInboxAdd(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
		jsonError(w, "text required", 400)
		return
	}

	inboxPath := cfg.GetInboxFilePath()

	// Ensure directory exists
	dir := filepath.Dir(inboxPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	defer f.Close()

	line := fmt.Sprintf("TODO: %s\n", body.Text)
	if _, err := f.WriteString(line); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResp(w, map[string]string{"id": "new", "text": body.Text})
}

// --- Projects & Keywords (helper endpoints) ---

func handleProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := task.SummarizeProjects(cfg)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonResp(w, projects)
}

func handleKeywords(w http.ResponseWriter, r *http.Request) {
	keywords := task.GetAllKeywords(cfg)
	jsonResp(w, keywords)
}

// --- WebSocket ---

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	clientsMu.Lock()
	clients[conn] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, conn)
		clientsMu.Unlock()
		conn.Close()
	}()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func broadcastChange(view string) {
	msg, _ := json.Marshal(map[string]string{"type": "reload", "view": view})
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for conn := range clients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			conn.Close()
			delete(clients, conn)
		}
	}
}

// --- Filesystem watcher ---

func watchFilesystem() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fsnotify: %v", err)
		return
	}
	defer watcher.Close()

	// Watch projects directory
	if cfg.Directories.Projects != "" {
		watchRecursive(watcher, cfg.Directories.Projects)
	}

	// Watch zettelkasten directory
	zetDir := getZetDir()
	if zetDir != "" {
		watchRecursive(watcher, zetDir)
	}

	// Watch goals directory
	goalsDir := getGoalsDir()
	if goalsDir != "" {
		watchRecursive(watcher, goalsDir)
	}

	// Watch inbox file's parent dir
	inboxPath := cfg.GetInboxFilePath()
	if inboxDir := filepath.Dir(inboxPath); inboxDir != "" {
		watcher.Add(inboxDir)
	}

	// Debounce: coalesce rapid changes
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex
	pendingViews := make(map[string]bool)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}

			view := classifyPath(event.Name)
			if view == "" {
				continue
			}

			debounceMu.Lock()
			pendingViews[view] = true
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				debounceMu.Lock()
				views := pendingViews
				pendingViews = make(map[string]bool)
				debounceMu.Unlock()
				for v := range views {
					broadcastChange(v)
				}
			})
			debounceMu.Unlock()

			// Watch new directories
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					watcher.Add(event.Name)
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("fsnotify error: %v", err)
		}
	}
}

func watchRecursive(watcher *fsnotify.Watcher, root string) {
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Skip .git directories
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			watcher.Add(path)
		}
		return nil
	})
}

func classifyPath(path string) string {
	zetDir := getZetDir()
	goalsDir := getGoalsDir()
	inboxPath := cfg.GetInboxFilePath()

	if path == inboxPath {
		return "inbox"
	}
	if zetDir != "" && strings.HasPrefix(path, zetDir) {
		return "zettels"
	}
	if goalsDir != "" && strings.HasPrefix(path, goalsDir) {
		return "goals"
	}
	if cfg.Directories.Projects != "" && strings.HasPrefix(path, cfg.Directories.Projects) {
		if strings.Contains(path, "/notes/") {
			return "notes"
		}
		return "todo"
	}
	return ""
}

// --- Helpers ---

func isValidNoteID(id string) bool {
	if len(id) != 14 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func splitNoteID(combined string) (project, noteID string) {
	parts := strings.SplitN(combined, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func readNoteMetadata(path string) (title, preview string) {
	file, err := os.Open(path)
	if err != nil {
		return "Untitled", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	var previewLines []string

	for scanner.Scan() && lineCount < 10 {
		line := scanner.Text()
		lineCount++

		if lineCount == 1 && strings.HasPrefix(line, "# ") {
			title = strings.TrimPrefix(line, "# ")
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed != "" && len(previewLines) < 2 {
			previewLines = append(previewLines, trimmed)
		}
	}

	if title == "" {
		title = "Untitled"
	}
	preview = strings.Join(previewLines, " ")
	if len(preview) > 120 {
		preview = preview[:120] + "..."
	}
	return
}

func extractZettelTags(zetDir, zetID string) []string {
	content, err := zet.ReadZettelContent(zetDir, zetID)
	if err != nil {
		return []string{}
	}

	var tags []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		// Look for #hashtag patterns (not ## headings)
		words := strings.Fields(line)
		for _, w := range words {
			if strings.HasPrefix(w, "#") && !strings.HasPrefix(w, "##") && len(w) > 1 {
				tag := strings.TrimRight(strings.TrimPrefix(w, "#"), ".,;:!?)")
				if tag != "" && !seen[tag] {
					seen[tag] = true
					tags = append(tags, tag)
				}
			}
		}
	}
	if tags == nil {
		tags = []string{}
	}
	return tags
}
