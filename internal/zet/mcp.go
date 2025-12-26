package zet

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP Tool Input/Output types

type CreateZettelArgs struct {
	Title string `json:"title" jsonschema:"title for the new zettel (optional, can be empty)"`
}

type CreateZettelResult struct {
	ZettelID string `json:"zettel_id" jsonschema:"the ID of the created zettel"`
	Path     string `json:"path" jsonschema:"the file path of the created zettel"`
	Message  string `json:"message" jsonschema:"status message"`
}

type ListZettelsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of zettels to return (default: all)"`
}

type ListZettelsResult struct {
	Zettels []ZettelInfo `json:"zettels" jsonschema:"list of zettels"`
	Count   int          `json:"count" jsonschema:"total number of zettels returned"`
}

type ZettelInfo struct {
	ID    string `json:"id" jsonschema:"zettel ID (timestamp format)"`
	Title string `json:"title" jsonschema:"zettel title"`
	Path  string `json:"path" jsonschema:"file path to the zettel"`
}

type GetZettelArgs struct {
	ZettelID string `json:"zettel_id" jsonschema:"the ID or partial ID of the zettel to retrieve"`
}

type GetZettelResult struct {
	ID      string `json:"id" jsonschema:"zettel ID"`
	Title   string `json:"title" jsonschema:"zettel title"`
	Content string `json:"content" jsonschema:"full content of the zettel"`
	Path    string `json:"path" jsonschema:"file path to the zettel"`
}

type SearchZettelsArgs struct {
	Pattern string `json:"pattern" jsonschema:"search pattern (case-insensitive substring match)"`
}

type SearchZettelsResult struct {
	Results []SearchResultInfo `json:"results" jsonschema:"list of search results"`
	Count   int                `json:"count" jsonschema:"total number of matches"`
}

type SearchResultInfo struct {
	ZettelID string `json:"zettel_id" jsonschema:"zettel ID"`
	Title    string `json:"title" jsonschema:"zettel title"`
	LineNum  int    `json:"line_num" jsonschema:"line number of the match"`
	Line     string `json:"line" jsonschema:"the matching line"`
}

type SearchTitlesArgs struct {
	Pattern string `json:"pattern" jsonschema:"search pattern for titles (case-insensitive substring match)"`
}

type SearchTitlesResult struct {
	Zettels []ZettelInfo `json:"zettels" jsonschema:"list of zettels with matching titles"`
	Count   int          `json:"count" jsonschema:"total number of matches"`
}

type CountZettelsArgs struct{}

type CountZettelsResult struct {
	Count int `json:"count" jsonschema:"total number of zettels"`
}

type DeleteZettelArgs struct {
	ZettelID string `json:"zettel_id" jsonschema:"the ID of the zettel to delete"`
}

type DeleteZettelResult struct {
	Message string `json:"message" jsonschema:"status message"`
}

type UpdateZettelArgs struct {
	ZettelID string `json:"zettel_id" jsonschema:"the ID of the zettel to update"`
	Content  string `json:"content" jsonschema:"the new content for the zettel (full replacement)"`
}

type UpdateZettelResult struct {
	Message string `json:"message" jsonschema:"status message"`
}

type GetLastZettelArgs struct{}

type GetLastZettelResult struct {
	ID      string `json:"id" jsonschema:"zettel ID"`
	Title   string `json:"title" jsonschema:"zettel title"`
	Content string `json:"content" jsonschema:"full content of the zettel"`
	Path    string `json:"path" jsonschema:"file path to the zettel"`
}

type FindTodosArgs struct{}

type FindTodosResult struct {
	Results []SearchResultInfo `json:"results" jsonschema:"list of todo items found"`
	Count   int                `json:"count" jsonschema:"total number of todos"`
}

// MCPServer wraps the MCP server with zettelkasten operations
type MCPServer struct {
	zetDir string
	server *mcp.Server
}

// NewMCPServer creates a new MCP server for zettelkasten operations
func NewMCPServer(zetDir string) *MCPServer {
	s := &MCPServer{
		zetDir: zetDir,
	}

	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "zet",
		Version: "1.0.0",
	}, &mcp.ServerOptions{
		Instructions: "Manage freeform zettels (notes not tied to any project). For project-specific notes, use the 'note' MCP server instead.",
	})

	s.registerTools()
	return s
}

// Run starts the MCP server on stdio transport
func (s *MCPServer) Run(ctx context.Context) error {
	return s.server.Run(ctx, &mcp.StdioTransport{})
}

func (s *MCPServer) registerTools() {
	// Create zettel
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_zettel",
		Description: "Create a new zettel with an optional title. Returns the zettel ID and path.",
	}, s.createZettel)

	// List zettels
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_zettels",
		Description: "List all zettels sorted by ID (newest first). Optionally limit the number of results.",
	}, s.listZettels)

	// Get zettel
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_zettel",
		Description: "Get the full content of a zettel by ID. Supports partial ID matching.",
	}, s.getZettel)

	// Search zettels (fulltext)
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_zettels",
		Description: "Search for a pattern across all zettel contents. Case-insensitive substring match.",
	}, s.searchZettels)

	// Search titles
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "search_titles",
		Description: "Search for a pattern in zettel titles only. Case-insensitive substring match.",
	}, s.searchTitles)

	// Count zettels
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "count_zettels",
		Description: "Get the total count of zettels.",
	}, s.countZettels)

	// Delete zettel
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "delete_zettel",
		Description: "Delete a zettel by ID. This action cannot be undone.",
	}, s.deleteZettel)

	// Update zettel
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "update_zettel",
		Description: "Update a zettel's content. Replaces the entire content of the zettel.",
	}, s.updateZettel)

	// Get last zettel
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_last_zettel",
		Description: "Get the most recently modified zettel based on git history.",
	}, s.getLastZettel)

	// Find todos
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "find_todos",
		Description: "Find all TODO, TASK, and other action items across all zettels.",
	}, s.findTodos)
}

func (s *MCPServer) createZettel(ctx context.Context, req *mcp.CallToolRequest, args CreateZettelArgs) (*mcp.CallToolResult, CreateZettelResult, error) {
	zetID := GenerateZettelID()

	if err := CreateZettel(s.zetDir, zetID, args.Title); err != nil {
		return nil, CreateZettelResult{}, fmt.Errorf("failed to create zettel: %w", err)
	}

	// Update README
	if err := UpdateReadme(s.zetDir); err != nil {
		// Non-fatal, continue
	}

	// Git commit
	actualTitle := args.Title
	if actualTitle == "" {
		actualTitle = "Untitled"
	}
	GitCommit(s.zetDir, zetID, actualTitle)

	result := CreateZettelResult{
		ZettelID: zetID,
		Path:     fmt.Sprintf("%s/%s/README.md", s.zetDir, zetID),
		Message:  fmt.Sprintf("Created zettel %s", zetID),
	}

	return nil, result, nil
}

func (s *MCPServer) listZettels(ctx context.Context, req *mcp.CallToolRequest, args ListZettelsArgs) (*mcp.CallToolResult, ListZettelsResult, error) {
	zettels, err := ListZettels(s.zetDir)
	if err != nil {
		return nil, ListZettelsResult{}, fmt.Errorf("failed to list zettels: %w", err)
	}

	limit := len(zettels)
	if args.Limit > 0 && args.Limit < limit {
		limit = args.Limit
	}

	infos := make([]ZettelInfo, limit)
	for i := 0; i < limit; i++ {
		infos[i] = ZettelInfo{
			ID:    zettels[i].ID,
			Title: zettels[i].Title,
			Path:  zettels[i].Path,
		}
	}

	return nil, ListZettelsResult{
		Zettels: infos,
		Count:   len(infos),
	}, nil
}

func (s *MCPServer) getZettel(ctx context.Context, req *mcp.CallToolRequest, args GetZettelArgs) (*mcp.CallToolResult, GetZettelResult, error) {
	zetID := args.ZettelID

	// Handle partial ID matching
	if !IsValidZettelID(zetID) {
		matches, err := FindMatchingZettels(s.zetDir, zetID)
		if err != nil {
			return nil, GetZettelResult{}, fmt.Errorf("failed to find zettel: %w", err)
		}
		if len(matches) == 0 {
			return nil, GetZettelResult{}, fmt.Errorf("no zettel found matching: %s", zetID)
		}
		if len(matches) > 1 {
			var ids []string
			for _, m := range matches {
				ids = append(ids, m.ID)
			}
			return nil, GetZettelResult{}, fmt.Errorf("multiple zettels match '%s': %s", zetID, strings.Join(ids, ", "))
		}
		zetID = matches[0].ID
	}

	title, err := GetZettelTitle(s.zetDir, zetID)
	if err != nil {
		return nil, GetZettelResult{}, fmt.Errorf("failed to get zettel title: %w", err)
	}

	content, err := ReadZettelContent(s.zetDir, zetID)
	if err != nil {
		return nil, GetZettelResult{}, fmt.Errorf("failed to read zettel content: %w", err)
	}

	return nil, GetZettelResult{
		ID:      zetID,
		Title:   title,
		Content: content,
		Path:    fmt.Sprintf("%s/%s/README.md", s.zetDir, zetID),
	}, nil
}

func (s *MCPServer) searchZettels(ctx context.Context, req *mcp.CallToolRequest, args SearchZettelsArgs) (*mcp.CallToolResult, SearchZettelsResult, error) {
	results, err := SearchZettels(s.zetDir, args.Pattern)
	if err != nil {
		return nil, SearchZettelsResult{}, fmt.Errorf("failed to search zettels: %w", err)
	}

	infos := make([]SearchResultInfo, len(results))
	for i, r := range results {
		infos[i] = SearchResultInfo{
			ZettelID: r.ZettelID,
			Title:    r.Title,
			LineNum:  r.LineNum,
			Line:     r.Line,
		}
	}

	return nil, SearchZettelsResult{
		Results: infos,
		Count:   len(infos),
	}, nil
}

func (s *MCPServer) searchTitles(ctx context.Context, req *mcp.CallToolRequest, args SearchTitlesArgs) (*mcp.CallToolResult, SearchTitlesResult, error) {
	zettels, err := SearchZettelTitles(s.zetDir, args.Pattern)
	if err != nil {
		return nil, SearchTitlesResult{}, fmt.Errorf("failed to search titles: %w", err)
	}

	infos := make([]ZettelInfo, len(zettels))
	for i, z := range zettels {
		infos[i] = ZettelInfo{
			ID:    z.ID,
			Title: z.Title,
			Path:  z.Path,
		}
	}

	return nil, SearchTitlesResult{
		Zettels: infos,
		Count:   len(infos),
	}, nil
}

func (s *MCPServer) countZettels(ctx context.Context, req *mcp.CallToolRequest, args CountZettelsArgs) (*mcp.CallToolResult, CountZettelsResult, error) {
	count, err := CountZettels(s.zetDir)
	if err != nil {
		return nil, CountZettelsResult{}, fmt.Errorf("failed to count zettels: %w", err)
	}

	return nil, CountZettelsResult{Count: count}, nil
}

func (s *MCPServer) deleteZettel(ctx context.Context, req *mcp.CallToolRequest, args DeleteZettelArgs) (*mcp.CallToolResult, DeleteZettelResult, error) {
	if !IsValidZettelID(args.ZettelID) {
		return nil, DeleteZettelResult{}, fmt.Errorf("invalid zettel ID: %s", args.ZettelID)
	}

	title, _ := GetZettelTitle(s.zetDir, args.ZettelID)

	if err := DeleteZettel(s.zetDir, args.ZettelID); err != nil {
		return nil, DeleteZettelResult{}, fmt.Errorf("failed to delete zettel: %w", err)
	}

	// Git commit the deletion
	if title != "" {
		GitDeleteZettel(s.zetDir, args.ZettelID, title)
	}

	return nil, DeleteZettelResult{
		Message: fmt.Sprintf("Deleted zettel %s", args.ZettelID),
	}, nil
}

func (s *MCPServer) updateZettel(ctx context.Context, req *mcp.CallToolRequest, args UpdateZettelArgs) (*mcp.CallToolResult, UpdateZettelResult, error) {
	if !IsValidZettelID(args.ZettelID) {
		return nil, UpdateZettelResult{}, fmt.Errorf("invalid zettel ID: %s", args.ZettelID)
	}

	if err := WriteZettelContent(s.zetDir, args.ZettelID, args.Content); err != nil {
		return nil, UpdateZettelResult{}, fmt.Errorf("failed to update zettel: %w", err)
	}

	// Update README
	if err := UpdateReadme(s.zetDir); err != nil {
		// Non-fatal
	}

	// Git commit
	title, _ := GetZettelTitle(s.zetDir, args.ZettelID)
	if title == "" {
		title = "Untitled"
	}
	GitCommit(s.zetDir, args.ZettelID, title)

	return nil, UpdateZettelResult{
		Message: fmt.Sprintf("Updated zettel %s", args.ZettelID),
	}, nil
}

func (s *MCPServer) getLastZettel(ctx context.Context, req *mcp.CallToolRequest, args GetLastZettelArgs) (*mcp.CallToolResult, GetLastZettelResult, error) {
	zetID, err := GetLastZettelID(s.zetDir)
	if err != nil {
		return nil, GetLastZettelResult{}, fmt.Errorf("failed to get last zettel: %w", err)
	}

	title, err := GetZettelTitle(s.zetDir, zetID)
	if err != nil {
		return nil, GetLastZettelResult{}, fmt.Errorf("failed to get zettel title: %w", err)
	}

	content, err := ReadZettelContent(s.zetDir, zetID)
	if err != nil {
		return nil, GetLastZettelResult{}, fmt.Errorf("failed to read zettel content: %w", err)
	}

	return nil, GetLastZettelResult{
		ID:      zetID,
		Title:   title,
		Content: content,
		Path:    fmt.Sprintf("%s/%s/README.md", s.zetDir, zetID),
	}, nil
}

func (s *MCPServer) findTodos(ctx context.Context, req *mcp.CallToolRequest, args FindTodosArgs) (*mcp.CallToolResult, FindTodosResult, error) {
	results, err := FindTodos(s.zetDir)
	if err != nil {
		return nil, FindTodosResult{}, fmt.Errorf("failed to find todos: %w", err)
	}

	infos := make([]SearchResultInfo, len(results))
	for i, r := range results {
		infos[i] = SearchResultInfo{
			ZettelID: r.ZettelID,
			Title:    r.Title,
			LineNum:  r.LineNum,
			Line:     r.Line,
		}
	}

	return nil, FindTodosResult{
		Results: infos,
		Count:   len(infos),
	}, nil
}
