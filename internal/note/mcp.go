package note

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/task"
	"github.com/vinayprograms/karya/internal/zet"
)

// MCP Tool Input/Output types

// Project types
type ListProjectsArgs struct{}

type ListProjectsResult struct {
	Projects []ProjectInfo `json:"projects" jsonschema:"list of projects"`
	Count    int           `json:"count" jsonschema:"total number of projects"`
}

type ProjectInfo struct {
	Name      string `json:"name" jsonschema:"project name"`
	Path      string `json:"path" jsonschema:"project directory path"`
	HasNotes  bool   `json:"has_notes" jsonschema:"whether project has notes directory"`
	NoteCount int    `json:"note_count" jsonschema:"number of notes in project"`
	TaskCount int    `json:"task_count" jsonschema:"number of active tasks in project"`
}

type CreateProjectArgs struct {
	Name string `json:"name" jsonschema:"project name to create"`
}

type CreateProjectResult struct {
	Name    string `json:"name" jsonschema:"created project name"`
	Path    string `json:"path" jsonschema:"project directory path"`
	Message string `json:"message" jsonschema:"status message"`
}

// Note types
type ListNotesArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of notes to return (default: all)"`
}

type ListNotesResult struct {
	Notes   []NoteInfo `json:"notes" jsonschema:"list of notes"`
	Count   int        `json:"count" jsonschema:"total number of notes returned"`
	Project string     `json:"project" jsonschema:"project name"`
}

type NoteInfo struct {
	ID    string `json:"id" jsonschema:"note ID (timestamp format)"`
	Title string `json:"title" jsonschema:"note title"`
	Path  string `json:"path" jsonschema:"file path to the note"`
}

type CreateNoteArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	Title   string `json:"title,omitempty" jsonschema:"note title (optional)"`
	Content string `json:"content,omitempty" jsonschema:"initial content for the note (optional, added after title)"`
}

type CreateNoteResult struct {
	ID      string `json:"id" jsonschema:"created note ID"`
	Path    string `json:"path" jsonschema:"file path to the note"`
	Project string `json:"project" jsonschema:"project name"`
	Message string `json:"message" jsonschema:"status message"`
}

type GetNoteArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	NoteID  string `json:"note_id" jsonschema:"note ID or partial ID"`
}

type GetNoteResult struct {
	ID      string `json:"id" jsonschema:"note ID"`
	Title   string `json:"title" jsonschema:"note title"`
	Content string `json:"content" jsonschema:"full content of the note"`
	Path    string `json:"path" jsonschema:"file path to the note"`
	Project string `json:"project" jsonschema:"project name"`
}

type UpdateNoteArgs struct {
	Project    string `json:"project" jsonschema:"project name"`
	NoteID     string `json:"note_id" jsonschema:"note ID"`
	OldContent string `json:"old_content" jsonschema:"exact content block to find (must match character-for-character including all whitespace, newlines, and indentation). Copy this directly from get_note output."`
	NewContent string `json:"new_content" jsonschema:"content block to replace old_content with (the entire old_content block is replaced with this)"`
}

type UpdateNoteResult struct {
	Success bool   `json:"success" jsonschema:"whether the update succeeded"`
	Message string `json:"message" jsonschema:"status message"`
}

type DeleteNoteArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	NoteID  string `json:"note_id" jsonschema:"note ID to delete"`
}

type DeleteNoteResult struct {
	Message string `json:"message" jsonschema:"status message"`
}

type GetLastNoteArgs struct {
	Project string `json:"project" jsonschema:"project name"`
}

type GetLastNoteResult struct {
	ID      string `json:"id" jsonschema:"note ID"`
	Title   string `json:"title" jsonschema:"note title"`
	Content string `json:"content" jsonschema:"full content of the note"`
	Path    string `json:"path" jsonschema:"file path to the note"`
	Project string `json:"project" jsonschema:"project name"`
}

// Search types
type SearchNotesArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	Pattern string `json:"pattern" jsonschema:"search pattern (case-insensitive substring match)"`
}

type SearchNotesResult struct {
	Results []SearchResultInfo `json:"results" jsonschema:"list of search results"`
	Count   int                `json:"count" jsonschema:"total number of matches"`
	Project string             `json:"project" jsonschema:"project name"`
}

type SearchResultInfo struct {
	NoteID  string `json:"note_id" jsonschema:"note ID"`
	Title   string `json:"title" jsonschema:"note title"`
	LineNum int    `json:"line_num" jsonschema:"line number of the match"`
	Line    string `json:"line" jsonschema:"the matching line"`
}

type SearchTitlesArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	Pattern string `json:"pattern" jsonschema:"search pattern for titles (case-insensitive substring match)"`
}

type SearchTitlesResult struct {
	Notes   []NoteInfo `json:"notes" jsonschema:"list of notes with matching titles"`
	Count   int        `json:"count" jsonschema:"total number of matches"`
	Project string     `json:"project" jsonschema:"project name"`
}

// Task types
type AddTaskArgs struct {
	Project string `json:"project" jsonschema:"project name"`
	NoteID  string `json:"note_id" jsonschema:"note ID to add task to"`
	Keyword string `json:"keyword" jsonschema:"task keyword (e.g., TODO, TASK). Call get_keywords from todo MCP to see valid keywords."`
	Title   string `json:"title" jsonschema:"task title/description"`
}

type AddTaskResult struct {
	Message       string              `json:"message" jsonschema:"status message"`
	Success       bool                `json:"success" jsonschema:"whether the task was added successfully"`
	ValidKeywords map[string][]string `json:"valid_keywords" jsonschema:"valid keywords grouped by category"`
}

type FindTasksArgs struct {
	Project string `json:"project" jsonschema:"project name"`
}

type FindTasksResult struct {
	Tasks   []TaskInfo `json:"tasks" jsonschema:"list of tasks found"`
	Count   int        `json:"count" jsonschema:"total number of tasks"`
	Project string     `json:"project" jsonschema:"project name"`
}

type TaskInfo struct {
	NoteID  string `json:"note_id" jsonschema:"note ID containing the task"`
	Title   string `json:"title" jsonschema:"note title"`
	LineNum int    `json:"line_num" jsonschema:"line number of the task"`
	Line    string `json:"line" jsonschema:"the task line"`
}

type CountNotesArgs struct {
	Project string `json:"project" jsonschema:"project name"`
}

type CountNotesResult struct {
	Count   int    `json:"count" jsonschema:"total number of notes"`
	Project string `json:"project" jsonschema:"project name"`
}

// TOC types
type GetTOCArgs struct {
	Project string `json:"project" jsonschema:"project name"`
}

type GetTOCResult struct {
	Content string `json:"content" jsonschema:"full content of the table of contents (README.md)"`
	Path    string `json:"path" jsonschema:"file path to the TOC"`
	Project string `json:"project" jsonschema:"project name"`
}

// MCPServer wraps the MCP server with note operations
type MCPServer struct {
	config *config.Config
	server *mcp.Server
}

// NewMCPServer creates a new MCP server for note operations
func NewMCPServer(cfg *config.Config) *MCPServer {
	s := &MCPServer{
		config: cfg,
	}

	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "note",
		Version: "1.0.0",
	}, nil)

	s.registerTools()
	return s
}

// Run starts the MCP server on stdio transport
func (s *MCPServer) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

func (s *MCPServer) registerTools() {
	// Project operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_projects",
		Description: "List all projects with their note and task counts.",
	}, s.listProjects)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_project",
		Description: "Create a new project with notes directory. Use this before creating notes in a new project.",
	}, s.createProject)

	// Note operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_notes",
		Description: "List all notes in a project, sorted by ID (newest first).",
	}, s.listNotes)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_note",
		Description: "Create a new note in a project with optional title and content.",
	}, s.createNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_note",
		Description: "Get the full content of a note by ID. Supports partial ID matching.",
	}, s.getNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "update_note",
		Description: "Update a note by replacing a content block. IMPORTANT: (1) First call get_note to see exact content. (2) Copy the exact lines you want to replace into old_content - must match character-for-character including whitespace and newlines. (3) Provide new_content to replace that block. Fails if old_content not found or matches multiple locations.",
	}, s.updateNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "delete_note",
		Description: "Delete a note by ID. This action cannot be undone.",
	}, s.deleteNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_last_note",
		Description: "Get the most recently modified note in a project based on git history.",
	}, s.getLastNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "count_notes",
		Description: "Get the total count of notes in a project.",
	}, s.countNotes)

	// Search operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_notes",
		Description: "Search for a pattern across all note contents in a project. Case-insensitive substring match.",
	}, s.searchNotes)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_titles",
		Description: "Search for a pattern in note titles only. Case-insensitive substring match.",
	}, s.searchTitles)

	// TOC operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_toc",
		Description: "Get the table of contents (README.md) for a project's notes. The TOC is auto-generated and updated when notes are created/modified.",
	}, s.getTOC)

	// Task operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_task",
		Description: "Add a task line to an existing note. The task is appended at the end of the note. IMPORTANT: The note must exist - use create_note first if needed.",
	}, s.addTask)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "find_tasks",
		Description: "Find all tasks (TODO, TASK, etc.) across all notes in a project.",
	}, s.findTasks)
}

func (s *MCPServer) getNotesDir(project string) string {
	return filepath.Join(s.config.Directories.Projects, project, "notes")
}

func (s *MCPServer) projectExists(project string) bool {
	prjPath := filepath.Join(s.config.Directories.Projects, project)
	if _, err := os.Stat(prjPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func (s *MCPServer) notesExist(project string) bool {
	notesPath := s.getNotesDir(project)
	if _, err := os.Stat(notesPath); os.IsNotExist(err) {
		return false
	}
	return true
}

// Project operations

func (s *MCPServer) listProjects(ctx context.Context, req *mcp.CallToolRequest, args ListProjectsArgs) (*mcp.CallToolResult, ListProjectsResult, error) {
	prjDir := s.config.Directories.Projects

	entries, err := os.ReadDir(prjDir)
	if err != nil {
		return nil, ListProjectsResult{}, fmt.Errorf("failed to read projects directory: %w", err)
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		prjPath := filepath.Join(prjDir, entry.Name())
		notesPath := filepath.Join(prjPath, "notes")

		hasNotes := false
		noteCount := 0
		taskCount := 0

		if _, err := os.Stat(notesPath); err == nil {
			hasNotes = true
			if count, err := zet.CountZettels(notesPath); err == nil {
				noteCount = count
			}
			// Count tasks
			if results, err := zet.FindTodos(notesPath); err == nil {
				taskCount = len(results)
			}
		}

		projects = append(projects, ProjectInfo{
			Name:      entry.Name(),
			Path:      prjPath,
			HasNotes:  hasNotes,
			NoteCount: noteCount,
			TaskCount: taskCount,
		})
	}

	return nil, ListProjectsResult{
		Projects: projects,
		Count:    len(projects),
	}, nil
}

func (s *MCPServer) createProject(ctx context.Context, req *mcp.CallToolRequest, args CreateProjectArgs) (*mcp.CallToolResult, CreateProjectResult, error) {
	if args.Name == "" {
		return nil, CreateProjectResult{}, fmt.Errorf("project name is required")
	}

	prjPath := filepath.Join(s.config.Directories.Projects, args.Name)
	notesPath := filepath.Join(prjPath, "notes")

	// Create project and notes directories
	if err := os.MkdirAll(notesPath, 0755); err != nil {
		return nil, CreateProjectResult{}, fmt.Errorf("failed to create project: %w", err)
	}

	// Initialize git in notes directory
	if err := zet.GitInit(notesPath); err != nil {
		// Non-fatal, continue
	}

	return nil, CreateProjectResult{
		Name:    args.Name,
		Path:    prjPath,
		Message: fmt.Sprintf("Created project '%s' with notes directory", args.Name),
	}, nil
}

// Note operations

func (s *MCPServer) listNotes(ctx context.Context, req *mcp.CallToolRequest, args ListNotesArgs) (*mcp.CallToolResult, ListNotesResult, error) {
	if args.Project == "" {
		return nil, ListNotesResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, ListNotesResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)
	zettels, err := zet.ListZettels(notesDir)
	if err != nil {
		return nil, ListNotesResult{}, fmt.Errorf("failed to list notes: %w", err)
	}

	limit := len(zettels)
	if args.Limit > 0 && args.Limit < limit {
		limit = args.Limit
	}

	notes := make([]NoteInfo, limit)
	for i := 0; i < limit; i++ {
		notes[i] = NoteInfo{
			ID:    zettels[i].ID,
			Title: zettels[i].Title,
			Path:  zettels[i].Path,
		}
	}

	return nil, ListNotesResult{
		Notes:   notes,
		Count:   len(notes),
		Project: args.Project,
	}, nil
}

func (s *MCPServer) createNote(ctx context.Context, req *mcp.CallToolRequest, args CreateNoteArgs) (*mcp.CallToolResult, CreateNoteResult, error) {
	if args.Project == "" {
		return nil, CreateNoteResult{}, fmt.Errorf("project name is required")
	}

	if !s.projectExists(args.Project) {
		return nil, CreateNoteResult{}, fmt.Errorf("project '%s' does not exist. Use create_project first.", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)

	// Create notes directory if it doesn't exist
	if !s.notesExist(args.Project) {
		if err := os.MkdirAll(notesDir, 0755); err != nil {
			return nil, CreateNoteResult{}, fmt.Errorf("failed to create notes directory: %w", err)
		}
		if err := zet.GitInit(notesDir); err != nil {
			// Non-fatal
		}
	}

	noteID := zet.GenerateZettelID()

	if err := zet.CreateZettel(notesDir, noteID, args.Title); err != nil {
		return nil, CreateNoteResult{}, fmt.Errorf("failed to create note: %w", err)
	}

	// If content is provided, append it after the title
	if args.Content != "" {
		existingContent, err := zet.ReadZettelContent(notesDir, noteID)
		if err != nil {
			return nil, CreateNoteResult{}, fmt.Errorf("failed to read note: %w", err)
		}

		newContent := existingContent
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += "\n" + args.Content

		if err := zet.WriteZettelContent(notesDir, noteID, newContent); err != nil {
			return nil, CreateNoteResult{}, fmt.Errorf("failed to write content: %w", err)
		}
	}

	// Update README
	zet.UpdateReadme(notesDir)

	// Git commit
	title := args.Title
	if title == "" {
		title = "Untitled"
	}
	zet.GitCommit(notesDir, noteID, title)

	return nil, CreateNoteResult{
		ID:      noteID,
		Path:    filepath.Join(notesDir, noteID, "README.md"),
		Project: args.Project,
		Message: fmt.Sprintf("Created note %s in project '%s'", noteID, args.Project),
	}, nil
}

func (s *MCPServer) getNote(ctx context.Context, req *mcp.CallToolRequest, args GetNoteArgs) (*mcp.CallToolResult, GetNoteResult, error) {
	if args.Project == "" {
		return nil, GetNoteResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, GetNoteResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)
	noteID := args.NoteID

	// Handle partial ID matching
	if !zet.IsValidZettelID(noteID) {
		matches, err := zet.FindMatchingZettels(notesDir, noteID)
		if err != nil {
			return nil, GetNoteResult{}, fmt.Errorf("failed to find note: %w", err)
		}
		if len(matches) == 0 {
			return nil, GetNoteResult{}, fmt.Errorf("no note found matching: %s", noteID)
		}
		if len(matches) > 1 {
			var ids []string
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
			return nil, GetNoteResult{}, fmt.Errorf("multiple notes match '%s': %s", noteID, strings.Join(ids, ", "))
		}
		noteID = matches[0].ID
	}

	title, err := zet.GetZettelTitle(notesDir, noteID)
	if err != nil {
		return nil, GetNoteResult{}, fmt.Errorf("failed to get note title: %w", err)
	}

	content, err := zet.ReadZettelContent(notesDir, noteID)
	if err != nil {
		return nil, GetNoteResult{}, fmt.Errorf("failed to read note content: %w", err)
	}

	return nil, GetNoteResult{
		ID:      noteID,
		Title:   title,
		Content: content,
		Path:    filepath.Join(notesDir, noteID, "README.md"),
		Project: args.Project,
	}, nil
}

func (s *MCPServer) updateNote(ctx context.Context, req *mcp.CallToolRequest, args UpdateNoteArgs) (*mcp.CallToolResult, UpdateNoteResult, error) {
	if args.Project == "" {
		return nil, UpdateNoteResult{Success: false, Message: "project name is required"}, nil
	}

	if !zet.IsValidZettelID(args.NoteID) {
		return nil, UpdateNoteResult{Success: false, Message: fmt.Sprintf("invalid note ID: %s", args.NoteID)}, nil
	}

	if args.OldContent == "" {
		return nil, UpdateNoteResult{Success: false, Message: "old_content is required - use get_note first to see exact content"}, nil
	}

	notesDir := s.getNotesDir(args.Project)

	// Read current content
	content, err := zet.ReadZettelContent(notesDir, args.NoteID)
	if err != nil {
		return nil, UpdateNoteResult{Success: false, Message: fmt.Sprintf("failed to read note: %v", err)}, nil
	}

	// Check if old_content exists exactly once
	count := strings.Count(content, args.OldContent)
	if count == 0 {
		return nil, UpdateNoteResult{
			Success: false,
			Message: "old_content block not found in note. The entire old_content string must match character-for-character including all whitespace, newlines, and indentation. Use get_note first and copy the exact text you want to replace.",
		}, nil
	}
	if count > 1 {
		return nil, UpdateNoteResult{
			Success: false,
			Message: fmt.Sprintf("old_content block found %d times in the note. Include more surrounding lines in old_content to uniquely identify the section you want to replace.", count),
		}, nil
	}

	// Replace old content with new content
	newContent := strings.Replace(content, args.OldContent, args.NewContent, 1)

	if err := zet.WriteZettelContent(notesDir, args.NoteID, newContent); err != nil {
		return nil, UpdateNoteResult{Success: false, Message: fmt.Sprintf("failed to write note: %v", err)}, nil
	}

	// Update README
	zet.UpdateReadme(notesDir)

	// Git commit
	title, _ := zet.GetZettelTitle(notesDir, args.NoteID)
	if title == "" {
		title = "Untitled"
	}
	zet.GitCommit(notesDir, args.NoteID, title)

	return nil, UpdateNoteResult{
		Success: true,
		Message: fmt.Sprintf("Updated note %s", args.NoteID),
	}, nil
}

func (s *MCPServer) deleteNote(ctx context.Context, req *mcp.CallToolRequest, args DeleteNoteArgs) (*mcp.CallToolResult, DeleteNoteResult, error) {
	if args.Project == "" {
		return nil, DeleteNoteResult{}, fmt.Errorf("project name is required")
	}

	if !zet.IsValidZettelID(args.NoteID) {
		return nil, DeleteNoteResult{}, fmt.Errorf("invalid note ID: %s", args.NoteID)
	}

	notesDir := s.getNotesDir(args.Project)

	title, _ := zet.GetZettelTitle(notesDir, args.NoteID)

	if err := zet.DeleteZettel(notesDir, args.NoteID); err != nil {
		return nil, DeleteNoteResult{}, fmt.Errorf("failed to delete note: %w", err)
	}

	// Git commit deletion
	if title != "" {
		zet.GitDeleteZettel(notesDir, args.NoteID, title)
	}

	return nil, DeleteNoteResult{
		Message: fmt.Sprintf("Deleted note %s", args.NoteID),
	}, nil
}

func (s *MCPServer) getLastNote(ctx context.Context, req *mcp.CallToolRequest, args GetLastNoteArgs) (*mcp.CallToolResult, GetLastNoteResult, error) {
	if args.Project == "" {
		return nil, GetLastNoteResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, GetLastNoteResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)

	noteID, err := zet.GetLastZettelID(notesDir)
	if err != nil {
		return nil, GetLastNoteResult{}, fmt.Errorf("failed to get last note: %w", err)
	}

	title, err := zet.GetZettelTitle(notesDir, noteID)
	if err != nil {
		return nil, GetLastNoteResult{}, fmt.Errorf("failed to get note title: %w", err)
	}

	content, err := zet.ReadZettelContent(notesDir, noteID)
	if err != nil {
		return nil, GetLastNoteResult{}, fmt.Errorf("failed to read note content: %w", err)
	}

	return nil, GetLastNoteResult{
		ID:      noteID,
		Title:   title,
		Content: content,
		Path:    filepath.Join(notesDir, noteID, "README.md"),
		Project: args.Project,
	}, nil
}

func (s *MCPServer) countNotes(ctx context.Context, req *mcp.CallToolRequest, args CountNotesArgs) (*mcp.CallToolResult, CountNotesResult, error) {
	if args.Project == "" {
		return nil, CountNotesResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, CountNotesResult{Count: 0, Project: args.Project}, nil
	}

	notesDir := s.getNotesDir(args.Project)
	count, err := zet.CountZettels(notesDir)
	if err != nil {
		return nil, CountNotesResult{}, fmt.Errorf("failed to count notes: %w", err)
	}

	return nil, CountNotesResult{
		Count:   count,
		Project: args.Project,
	}, nil
}

// TOC operations

func (s *MCPServer) getTOC(ctx context.Context, req *mcp.CallToolRequest, args GetTOCArgs) (*mcp.CallToolResult, GetTOCResult, error) {
	if args.Project == "" {
		return nil, GetTOCResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, GetTOCResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)
	tocPath := filepath.Join(notesDir, "README.md")

	content, err := os.ReadFile(tocPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, GetTOCResult{
				Content: "",
				Path:    tocPath,
				Project: args.Project,
			}, nil
		}
		return nil, GetTOCResult{}, fmt.Errorf("failed to read TOC: %w", err)
	}

	return nil, GetTOCResult{
		Content: string(content),
		Path:    tocPath,
		Project: args.Project,
	}, nil
}

// Search operations

func (s *MCPServer) searchNotes(ctx context.Context, req *mcp.CallToolRequest, args SearchNotesArgs) (*mcp.CallToolResult, SearchNotesResult, error) {
	if args.Project == "" {
		return nil, SearchNotesResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, SearchNotesResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)

	results, err := zet.SearchZettels(notesDir, args.Pattern)
	if err != nil {
		return nil, SearchNotesResult{}, fmt.Errorf("failed to search notes: %w", err)
	}

	infos := make([]SearchResultInfo, len(results))
	for i, r := range results {
		infos[i] = SearchResultInfo{
			NoteID:  r.ZettelID,
			Title:   r.Title,
			LineNum: r.LineNum,
			Line:    r.Line,
		}
	}

	return nil, SearchNotesResult{
		Results: infos,
		Count:   len(infos),
		Project: args.Project,
	}, nil
}

func (s *MCPServer) searchTitles(ctx context.Context, req *mcp.CallToolRequest, args SearchTitlesArgs) (*mcp.CallToolResult, SearchTitlesResult, error) {
	if args.Project == "" {
		return nil, SearchTitlesResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, SearchTitlesResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)

	zettels, err := zet.SearchZettelTitles(notesDir, args.Pattern)
	if err != nil {
		return nil, SearchTitlesResult{}, fmt.Errorf("failed to search titles: %w", err)
	}

	notes := make([]NoteInfo, len(zettels))
	for i, z := range zettels {
		notes[i] = NoteInfo{
			ID:    z.ID,
			Title: z.Title,
			Path:  z.Path,
		}
	}

	return nil, SearchTitlesResult{
		Notes:   notes,
		Count:   len(notes),
		Project: args.Project,
	}, nil
}

// Task operations

func (s *MCPServer) addTask(ctx context.Context, req *mcp.CallToolRequest, args AddTaskArgs) (*mcp.CallToolResult, AddTaskResult, error) {
	validKeywords := task.GetAllKeywords(s.config)

	if args.Project == "" {
		return nil, AddTaskResult{
			Success:       false,
			Message:       "project name is required",
			ValidKeywords: validKeywords,
		}, nil
	}

	if args.NoteID == "" {
		return nil, AddTaskResult{
			Success:       false,
			Message:       "note_id is required",
			ValidKeywords: validKeywords,
		}, nil
	}

	if args.Keyword == "" {
		return nil, AddTaskResult{
			Success:       false,
			Message:       "keyword is required",
			ValidKeywords: validKeywords,
		}, nil
	}

	if args.Title == "" {
		return nil, AddTaskResult{
			Success:       false,
			Message:       "task title is required",
			ValidKeywords: validKeywords,
		}, nil
	}

	// Validate keyword
	if !isValidKeyword(s.config, args.Keyword) {
		return nil, AddTaskResult{
			Success:       false,
			Message:       fmt.Sprintf("invalid keyword '%s'. See valid_keywords for allowed values.", args.Keyword),
			ValidKeywords: validKeywords,
		}, nil
	}

	if !s.notesExist(args.Project) {
		return nil, AddTaskResult{
			Success:       false,
			Message:       fmt.Sprintf("project '%s' has no notes directory", args.Project),
			ValidKeywords: validKeywords,
		}, nil
	}

	notesDir := s.getNotesDir(args.Project)

	if !zet.IsValidZettelID(args.NoteID) {
		return nil, AddTaskResult{
			Success:       false,
			Message:       fmt.Sprintf("invalid note ID: %s", args.NoteID),
			ValidKeywords: validKeywords,
		}, nil
	}

	// Read existing content
	content, err := zet.ReadZettelContent(notesDir, args.NoteID)
	if err != nil {
		return nil, AddTaskResult{
			Success:       false,
			Message:       fmt.Sprintf("failed to read note: %v", err),
			ValidKeywords: validKeywords,
		}, nil
	}

	// Append task line
	taskLine := fmt.Sprintf("%s: %s", args.Keyword, args.Title)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "\n" + taskLine + "\n"

	// Write back
	if err := zet.WriteZettelContent(notesDir, args.NoteID, content); err != nil {
		return nil, AddTaskResult{
			Success:       false,
			Message:       fmt.Sprintf("failed to write note: %v", err),
			ValidKeywords: validKeywords,
		}, nil
	}

	// Git commit
	title, _ := zet.GetZettelTitle(notesDir, args.NoteID)
	if title == "" {
		title = "Untitled"
	}
	zet.GitCommit(notesDir, args.NoteID, title)

	return nil, AddTaskResult{
		Success:       true,
		Message:       fmt.Sprintf("Added task '%s: %s' to note %s", args.Keyword, args.Title, args.NoteID),
		ValidKeywords: validKeywords,
	}, nil
}

func (s *MCPServer) findTasks(ctx context.Context, req *mcp.CallToolRequest, args FindTasksArgs) (*mcp.CallToolResult, FindTasksResult, error) {
	if args.Project == "" {
		return nil, FindTasksResult{}, fmt.Errorf("project name is required")
	}

	if !s.notesExist(args.Project) {
		return nil, FindTasksResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)

	results, err := zet.FindTodos(notesDir)
	if err != nil {
		return nil, FindTasksResult{}, fmt.Errorf("failed to find tasks: %w", err)
	}

	tasks := make([]TaskInfo, len(results))
	for i, r := range results {
		tasks[i] = TaskInfo{
			NoteID:  r.ZettelID,
			Title:   r.Title,
			LineNum: r.LineNum,
			Line:    r.Line,
		}
	}

	return nil, FindTasksResult{
		Tasks:   tasks,
		Count:   len(tasks),
		Project: args.Project,
	}, nil
}

// isValidKeyword checks if a keyword is valid according to config
func isValidKeyword(c *config.Config, keyword string) bool {
	for _, kw := range c.Todo.Active {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.Todo.InProgress {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.Todo.Completed {
		if keyword == kw {
			return true
		}
	}
	for _, kw := range c.Todo.Someday {
		if keyword == kw {
			return true
		}
	}
	return false
}
