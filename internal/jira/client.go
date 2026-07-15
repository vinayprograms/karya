package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Client calls JIRA tools via the Atlassian MCP server.
type Client struct {
	session  *mcp.ClientSession
	cloudID  string
	endpoint string
	token    *TokenStore
}

// NewClient creates a JIRA client for a named connection.
func NewClient(name string) (*Client, error) {
	store := NewTokenStore(name)
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("loading token for %q: %w (run 'todo jira-auth %s')", name, err, name)
	}
	return &Client{
		token: store,
	}, nil
}

// Init connects to the Atlassian MCP server and establishes a session.
func (c *Client) Init(ctx context.Context) error {
	tok, err := c.token.AccessToken()
	if err != nil {
		return err
	}

	// Read the endpoint from stored config
	endpoint := c.token.Endpoint()
	if endpoint == "" {
		return fmt.Errorf("no endpoint stored for this connection")
	}

	transport := &mcp.StreamableClientTransport{
		Endpoint: endpoint,
		HTTPClient: &http.Client{
			Timeout:   60 * time.Second,
			Transport: &bearerTransport{token: tok},
		},
	}

	mcpClient := mcp.NewClient(&mcp.Implementation{
		Name:    "karya-todo",
		Version: "1.0.0",
	}, nil)

	session, err := mcpClient.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connecting to MCP server: %w", err)
	}
	c.session = session

	// Discover cloud ID by calling getAccessibleAtlassianResources
	resources, err := c.callTool(ctx, "getAccessibleAtlassianResources", map[string]any{})
	if err != nil {
		return fmt.Errorf("getting accessible resources: %w", err)
	}

	// Parse cloud ID from response
	c.cloudID = extractCloudID(resources)
	if c.cloudID == "" {
		return fmt.Errorf("could not determine cloud ID from accessible resources")
	}

	return nil
}

// SearchIssues executes a JQL query and returns matching issues.
func (c *Client) SearchIssues(ctx context.Context, jql string) ([]Issue, error) {
	args := map[string]any{
		"cloudId":               c.cloudID,
		"jql":                   jql,
		"fields":                []string{"summary", "description", "status", "assignee", "duedate", "labels", "issuetype", "parent", "comment"},
		"responseContentFormat": "markdown",
		"maxResults":            100,
		"searchResultMode":      "issues",
	}

	result, err := c.callTool(ctx, "searchJiraIssuesUsingJql", args)
	if err != nil {
		return nil, fmt.Errorf("JQL search: %w", err)
	}

	return parseIssuesFromMCP(result)
}

// GetIssue fetches a single issue by key.
func (c *Client) GetIssue(ctx context.Context, key string) (*Issue, error) {
	args := map[string]any{
		"cloudId":               c.cloudID,
		"issueIdOrKey":          key,
		"fields":                []string{"summary", "description", "status", "assignee", "duedate", "labels", "issuetype", "parent", "comment"},
		"responseContentFormat": "markdown",
	}

	result, err := c.callTool(ctx, "getJiraIssue", args)
	if err != nil {
		return nil, fmt.Errorf("get issue %s: %w", key, err)
	}

	return parseSingleIssueFromMCP(result)
}

// GetCurrentUser returns the account ID of the authenticated user.
func (c *Client) GetCurrentUser(ctx context.Context) (string, error) {
	result, err := c.callTool(ctx, "atlassianUserInfo", map[string]any{})
	if err != nil {
		return "", fmt.Errorf("get current user: %w", err)
	}

	var info struct {
		AccountID string `json:"account_id"`
	}
	if err := json.Unmarshal([]byte(result), &info); err != nil {
		return "", err
	}
	return info.AccountID, nil
}

// Close terminates the MCP session.
func (c *Client) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

func (c *Client) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return "", err
	}

	if result.IsError {
		return "", fmt.Errorf("tool %s returned error: %s", name, contentText(result.Content))
	}

	return contentText(result.Content), nil
}

func contentText(content []mcp.Content) string {
	var parts []string
	for _, c := range content {
		if tc, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractCloudID(result string) string {
	// The response is typically JSON with cloud IDs
	var resources []struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(result), &resources); err == nil && len(resources) > 0 {
		return resources[0].ID
	}

	// Try as single object
	var resource struct {
		ID      string `json:"id"`
		CloudID string `json:"cloudId"`
	}
	if err := json.Unmarshal([]byte(result), &resource); err == nil {
		if resource.CloudID != "" {
			return resource.CloudID
		}
		return resource.ID
	}

	return ""
}

func parseIssuesFromMCP(result string) ([]Issue, error) {
	// The MCP tool returns structured JSON with issues
	var searchResult struct {
		Issues []mcpIssue `json:"issues"`
	}
	if err := json.Unmarshal([]byte(result), &searchResult); err != nil {
		// Try parsing as plain text / markdown and extract what we can
		return nil, fmt.Errorf("parsing search result: %w (raw: %.200s)", err, result)
	}

	var issues []Issue
	for _, mi := range searchResult.Issues {
		issues = append(issues, mi.toIssue())
	}
	return issues, nil
}

func parseSingleIssueFromMCP(result string) (*Issue, error) {
	var mi mcpIssue
	if err := json.Unmarshal([]byte(result), &mi); err != nil {
		return nil, fmt.Errorf("parsing issue: %w (raw: %.200s)", err, result)
	}
	issue := mi.toIssue()
	return &issue, nil
}

// mcpIssue maps the JSON structure returned by the Atlassian MCP tools.
type mcpIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description string `json:"description"`
		Status      struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Assignee *struct {
			AccountID   string `json:"accountId"`
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		DueDate   string   `json:"duedate"`
		Labels    []string `json:"labels"`
		IssueType struct {
			Name    string `json:"name"`
			Subtask bool   `json:"subtask"`
		} `json:"issuetype"`
		Parent *struct {
			Key string `json:"key"`
		} `json:"parent"`
		Comment struct {
			Comments []struct {
				Author struct {
					AccountID   string `json:"accountId"`
					DisplayName string `json:"displayName"`
				} `json:"author"`
				Body    string `json:"body"`
				Created string `json:"created"`
			} `json:"comments"`
		} `json:"comment"`
	} `json:"fields"`
}

func (mi *mcpIssue) toIssue() Issue {
	i := Issue{
		Key: mi.Key,
		Fields: IssueFields{
			Summary:     mi.Fields.Summary,
			Description: SanitizeContent(mi.Fields.Description),
			DueDate:     mi.Fields.DueDate,
			Labels:      mi.Fields.Labels,
			IssueType:   IssueType{Name: mi.Fields.IssueType.Name, Subtask: mi.Fields.IssueType.Subtask},
			Status: Status{
				Name:           mi.Fields.Status.Name,
				StatusCategory: StatusCategory{Key: mi.Fields.Status.StatusCategory.Key},
			},
		},
	}

	if mi.Fields.Assignee != nil {
		i.Fields.Assignee = &User{
			AccountID:   mi.Fields.Assignee.AccountID,
			DisplayName: mi.Fields.Assignee.DisplayName,
		}
	}

	if mi.Fields.Parent != nil {
		i.Fields.Parent = &ParentRef{Key: mi.Fields.Parent.Key}
	}

	for _, c := range mi.Fields.Comment.Comments {
		created, _ := time.Parse("2006-01-02T15:04:05.000-0700", c.Created)
		i.Fields.Comment.Comments = append(i.Fields.Comment.Comments, Comment{
			Author:  User{AccountID: c.Author.AccountID, DisplayName: c.Author.DisplayName},
			Body:    SanitizeContent(c.Body),
			Created: created,
		})
	}

	return i
}

// bearerTransport adds Authorization header to all requests.
type bearerTransport struct {
	token string
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(req)
}

// SanitizeContent strips Atlassian-specific custom HTML elements from markdown content,
// replacing them with their plain-text inner content.
func SanitizeContent(s string) string {
	if s == "" {
		return s
	}

	// Replace <custom data-type="smartlink" ...>URL</custom> with just the URL
	// Replace <custom data-type="mention" ...>@Name</custom> with just @Name
	// Generic: strip all <custom ...>inner</custom> → inner (handles nested HTML)
	re := regexp.MustCompile(`(?s)<custom[^>]*>(.*?)</custom>`)
	s = re.ReplaceAllString(s, "$1")

	// Strip any remaining HTML-like tags (e.g., <br>, <p>, etc.)
	htmlRe := regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	s = htmlRe.ReplaceAllString(s, "")

	// Collapse unicode zero-width/invisible characters that Atlassian inserts
	s = strings.ReplaceAll(s, "‌", "")
	s = strings.ReplaceAll(s, "​", "")

	// Collapse multiple blank lines into one
	blankLines := regexp.MustCompile(`\n{3,}`)
	s = blankLines.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}
