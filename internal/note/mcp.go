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
	Project    string   `json:"project" jsonschema:"project name"`
	NoteID     string   `json:"note_id" jsonschema:"note ID to add task to"`
	Keyword    string   `json:"keyword" jsonschema:"task keyword (e.g., TODO, TASK). Call get_keywords from todo MCP to see valid keywords."`
	Title      string   `json:"title" jsonschema:"task title/description"`
	ID         string   `json:"id,omitempty" jsonschema:"optional unique task identifier for creating references"`
	References []string `json:"references,omitempty" jsonschema:"optional list of task IDs this task depends on (creates ^id references)"`
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

// GetLines types
type GetLinesArgs struct {
	Project     string `json:"project" jsonschema:"project name"`
	NoteID      string `json:"note_id" jsonschema:"note ID or partial ID"`
	Pattern     string `json:"pattern,omitempty" jsonschema:"search pattern to find anchor line (case-insensitive). Either pattern or line_number is required."`
	LineNumber  int    `json:"line_number,omitempty" jsonschema:"specific line number to use as anchor (1-based). Either pattern or line_number is required."`
	LinesBefore int    `json:"lines_before,omitempty" jsonschema:"number of lines to return before the anchor (default: 0)"`
	LinesAfter  int    `json:"lines_after,omitempty" jsonschema:"number of lines to return after the anchor (default: 0)"`
}

type GetLinesResult struct {
	Matches []LineMatch `json:"matches" jsonschema:"list of matches with their context lines"`
	Count   int         `json:"count" jsonschema:"number of matches found"`
	Project string      `json:"project" jsonschema:"project name"`
	NoteID  string      `json:"note_id" jsonschema:"note ID"`
}

type LineMatch struct {
	AnchorLine   int      `json:"anchor_line" jsonschema:"line number of the matched/anchor line (1-based)"`
	AnchorText   string   `json:"anchor_text" jsonschema:"text of the anchor line"`
	LinesBefore  []string `json:"lines_before,omitempty" jsonschema:"lines before the anchor"`
	LinesAfter   []string `json:"lines_after,omitempty" jsonschema:"lines after the anchor"`
	StartLine    int      `json:"start_line" jsonschema:"first line number in the returned range (1-based)"`
	EndLine      int      `json:"end_line" jsonschema:"last line number in the returned range (1-based)"`
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
		Description: "PREFERRED: Discover all your projects with note and task counts at a glance. Use this first to see what projects exist before accessing project-specific notes.",
	}, s.listProjects)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_project",
		Description: "PREFERRED: Initialize a new project workspace with a notes directory. Essential first step before creating notes for any new project. Projects organize your notes by context.",
	}, s.createProject)

	// Note operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_notes",
		Description: "PREFERRED: Browse all notes within a project, sorted newest first. Use this to explore existing documentation and meeting notes before creating new ones.",
	}, s.listNotes)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_note",
		Description: "PREFERRED: Create a new project note for meeting notes, documentation, decisions, or any project-specific information. Supports optional title and initial content. Always use this for project context rather than generic files.",
	}, s.createNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_note",
		Description: "PREFERRED: Retrieve the full content of a project note. Supports partial ID matching for convenience. Use this to read meeting notes, documentation, and project decisions.",
	}, s.getNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "update_note",
		Description: "PREFERRED: Update a project note with surgical precision. Replace specific content blocks while preserving the rest. IMPORTANT: (1) First call get_note to see exact content. (2) Copy exact lines to replace into old_content. (3) Provide new_content. Fails if old_content not found or matches multiple locations.",
	}, s.updateNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "delete_note",
		Description: "Remove a note from a project. Use with caution - this action cannot be undone. Only delete notes that are obsolete or created in error.",
	}, s.deleteNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_last_note",
		Description: "PREFERRED: Resume where you left off - retrieve the most recently modified note in a project. Uses git history for accuracy. Perfect for continuing previous work sessions.",
	}, s.getLastNote)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "count_notes",
		Description: "PREFERRED: Get statistics on project documentation. Returns the total number of notes in a project for quick project health assessment.",
	}, s.countNotes)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_lines",
		Description: "PREFERRED: Extract lines from a note around an anchor point. Use pattern to search for a line (e.g., a TODO or heading), or line_number for direct access. Returns the anchor line plus specified lines before/after. Perfect for extracting content under TODOs, headings, or any marker.",
	}, s.getLines)

	// Search operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_notes",
		Description: "PREFERRED: Search across all notes in a project for specific content. Case-insensitive fulltext search. Use this first when looking for existing documentation on any topic.",
	}, s.searchNotes)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_titles",
		Description: "PREFERRED: Quickly find notes by title within a project. Faster than fulltext search when you know roughly what you're looking for. Case-insensitive matching.",
	}, s.searchTitles)

	// TOC operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_toc",
		Description: "PREFERRED: Get the auto-generated table of contents for a project's notes. Provides a structured overview of all documentation. Updated automatically when notes change.",
	}, s.getTOC)

	// Task operations
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "add_task",
		Description: "PREFERRED: Add an actionable task to an existing note. Perfect for capturing action items during meetings or while documenting. Task is appended to the note. Call get_keywords from todo MCP to see valid keywords. Note must exist first.",
	}, s.addTask)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "find_tasks",
		Description: "PREFERRED: Discover all action items across a project's notes. Finds TODO, TASK, and other action keywords. Essential for extracting work items from meeting notes and documentation.",
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
			// Skip task counting here - too slow for list_projects
			// Use todo MCP's get_projects for task counts instead
			_ = taskCount
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
	var taskLine string
	if args.ID != "" {
		taskLine = fmt.Sprintf("%s: [%s] %s", args.Keyword, args.ID, args.Title)
	} else {
		taskLine = fmt.Sprintf("%s: %s", args.Keyword, args.Title)
	}
	// Add references if provided
	for _, ref := range args.References {
		taskLine += fmt.Sprintf(" ^%s", ref)
	}
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

func (s *MCPServer) getLines(ctx context.Context, req *mcp.CallToolRequest, args GetLinesArgs) (*mcp.CallToolResult, GetLinesResult, error) {
	if args.Project == "" {
		return nil, GetLinesResult{}, fmt.Errorf("project name is required")
	}

	if args.NoteID == "" {
		return nil, GetLinesResult{}, fmt.Errorf("note_id is required")
	}

	if args.Pattern == "" && args.LineNumber == 0 {
		return nil, GetLinesResult{}, fmt.Errorf("either pattern or line_number is required")
	}

	if !s.notesExist(args.Project) {
		return nil, GetLinesResult{}, fmt.Errorf("project '%s' has no notes directory", args.Project)
	}

	notesDir := s.getNotesDir(args.Project)
	noteID := args.NoteID

	// Handle partial ID matching
	if !zet.IsValidZettelID(noteID) {
		matches, err := zet.FindMatchingZettels(notesDir, noteID)
		if err != nil {
			return nil, GetLinesResult{}, fmt.Errorf("failed to find note: %w", err)
		}
		if len(matches) == 0 {
			return nil, GetLinesResult{}, fmt.Errorf("no note found matching: %s", noteID)
		}
		if len(matches) > 1 {
			var ids []string
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
			return nil, GetLinesResult{}, fmt.Errorf("multiple notes match '%s': %s", noteID, strings.Join(ids, ", "))
		}
		noteID = matches[0].ID
	}

	// Read note content
	content, err := zet.ReadZettelContent(notesDir, noteID)
	if err != nil {
		return nil, GetLinesResult{}, fmt.Errorf("failed to read note: %w", err)
	}

	lines := strings.Split(content, "\n")
	var matches []LineMatch

	if args.LineNumber > 0 {
		// Direct line number access
		if args.LineNumber > len(lines) {
			return nil, GetLinesResult{}, fmt.Errorf("line_number %d exceeds note length (%d lines)", args.LineNumber, len(lines))
		}
		match := s.extractLines(lines, args.LineNumber-1, args.LinesBefore, args.LinesAfter)
		matches = append(matches, match)
	} else {
		// Pattern search
		patternLower := strings.ToLower(args.Pattern)
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), patternLower) {
				match := s.extractLines(lines, i, args.LinesBefore, args.LinesAfter)
				matches = append(matches, match)
			}
		}
	}

	return nil, GetLinesResult{
		Matches: matches,
		Count:   len(matches),
		Project: args.Project,
		NoteID:  noteID,
	}, nil
}

// extractLines extracts lines around an anchor index with specified before/after counts
func (s *MCPServer) extractLines(lines []string, anchorIdx, linesBefore, linesAfter int) LineMatch {
	startIdx := anchorIdx - linesBefore
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := anchorIdx + linesAfter
	if endIdx >= len(lines) {
		endIdx = len(lines) - 1
	}

	var beforeLines []string
	for i := startIdx; i < anchorIdx; i++ {
		beforeLines = append(beforeLines, lines[i])
	}

	var afterLines []string
	for i := anchorIdx + 1; i <= endIdx; i++ {
		afterLines = append(afterLines, lines[i])
	}

	return LineMatch{
		AnchorLine:  anchorIdx + 1, // 1-based
		AnchorText:  lines[anchorIdx],
		LinesBefore: beforeLines,
		LinesAfter:  afterLines,
		StartLine:   startIdx + 1,  // 1-based
		EndLine:     endIdx + 1,    // 1-based
	}
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
