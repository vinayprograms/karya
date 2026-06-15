package goal

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP Tool Input/Output types

type ListGoalsArgs struct {
	Horizon string `json:"horizon,omitempty" jsonschema:"optional horizon to filter (monthly, quarterly, yearly, short-term, long-term)"`
}

type ListGoalsResult struct {
	Goals []GoalInfo `json:"goals" jsonschema:"list of goals"`
	Count int        `json:"count" jsonschema:"total number of goals returned"`
}

type GoalInfo struct {
	Title   string `json:"title" jsonschema:"goal title"`
	Horizon string `json:"horizon" jsonschema:"time horizon (monthly, quarterly, yearly, short-term, long-term)"`
	Period  string `json:"period" jsonschema:"time period (e.g., 2026-05, 2026-Q2, 2026, 2025-2028)"`
	Path    string `json:"path" jsonschema:"file path to the goal"`
}

type CreateGoalArgs struct {
	Horizon string `json:"horizon" jsonschema:"time horizon: monthly, quarterly, yearly, short-term, long-term"`
	Period  string `json:"period" jsonschema:"time period (e.g., 2026-05, 2026-Q2, 2026, 2025-2028)"`
	Title   string `json:"title" jsonschema:"goal title"`
}

type CreateGoalResult struct {
	Goal    *GoalInfo `json:"goal,omitempty" jsonschema:"the created goal"`
	Message string    `json:"message" jsonschema:"status message"`
	Success bool      `json:"success" jsonschema:"whether creation succeeded"`
}

type GetGoalContentArgs struct {
	Horizon string `json:"horizon" jsonschema:"time horizon: monthly, quarterly, yearly, short-term, long-term"`
	Period  string `json:"period" jsonschema:"time period (e.g., 2026-05, 2026-Q2, 2026, 2025-2028)"`
	Title   string `json:"title" jsonschema:"goal title"`
}

type GetGoalContentResult struct {
	Goal    *GoalInfo `json:"goal,omitempty" jsonschema:"goal metadata"`
	Content string    `json:"content" jsonschema:"goal file content"`
	Found   bool      `json:"found" jsonschema:"whether the goal was found"`
}

// MCPServer wraps the MCP server with goal operations
type MCPServer struct {
	manager *GoalManager
	server  *mcp.Server
}

// NewMCPServer creates a new MCP server for goal operations
func NewMCPServer(manager *GoalManager) *MCPServer {
	s := &MCPServer{manager: manager}

	s.server = mcp.NewServer(&mcp.Implementation{
		Name:    "goal",
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
	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "list_goals",
		Description: "PREFERRED: List all goals across all horizons, or filtered to a specific horizon. Returns goals grouped by horizon and period. Use this as your primary goals dashboard.",
	}, s.listGoals)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "create_goal",
		Description: "PREFERRED: Create a new goal for a specific horizon and period. Horizon must be one of: monthly, quarterly, yearly, short-term, long-term. Period format: YYYY-MM for monthly, YYYY-QN for quarterly, YYYY for yearly, YYYY-YYYY for short/long-term.",
	}, s.createGoal)

	mcp.AddTool(s.server, &mcp.Tool{
		Name:        "get_goal_content",
		Description: "PREFERRED: Read the full content of a specific goal file. Use list_goals first to discover available goals and their titles.",
	}, s.getGoalContent)
}

func (s *MCPServer) listGoals(ctx context.Context, req *mcp.CallToolRequest, args ListGoalsArgs) (*mcp.CallToolResult, ListGoalsResult, error) {
	horizons := []Horizon{
		HorizonMonthly,
		HorizonQuarterly,
		HorizonYearly,
		HorizonShortTerm,
		HorizonLongTerm,
	}

	if args.Horizon != "" {
		horizons = []Horizon{Horizon(args.Horizon)}
	}

	var goals []GoalInfo
	for _, h := range horizons {
		byPeriod, err := s.manager.ListGoalsByHorizon(h)
		if err != nil {
			continue
		}
		for period, titles := range byPeriod {
			for _, title := range titles {
				goals = append(goals, GoalInfo{
					Title:   title,
					Horizon: string(h),
					Period:  period,
					Path:    s.manager.GetGoalPathForHorizon(h, period, title),
				})
			}
		}
	}

	return nil, ListGoalsResult{Goals: goals, Count: len(goals)}, nil
}

func (s *MCPServer) createGoal(ctx context.Context, req *mcp.CallToolRequest, args CreateGoalArgs) (*mcp.CallToolResult, CreateGoalResult, error) {
	if args.Horizon == "" || args.Period == "" || args.Title == "" {
		return nil, CreateGoalResult{
			Success: false,
			Message: "horizon, period, and title are all required",
		}, nil
	}

	h := Horizon(args.Horizon)
	if err := s.manager.CreateGoal(h, args.Period, args.Title); err != nil {
		return nil, CreateGoalResult{
			Success: false,
			Message: fmt.Sprintf("failed to create goal: %v", err),
		}, nil
	}

	info := GoalInfo{
		Title:   args.Title,
		Horizon: args.Horizon,
		Period:  args.Period,
		Path:    s.manager.GetGoalPathForHorizon(h, args.Period, args.Title),
	}
	return nil, CreateGoalResult{
		Goal:    &info,
		Success: true,
		Message: fmt.Sprintf("Created goal: %s (%s/%s)", args.Title, args.Horizon, args.Period),
	}, nil
}

func (s *MCPServer) getGoalContent(ctx context.Context, req *mcp.CallToolRequest, args GetGoalContentArgs) (*mcp.CallToolResult, GetGoalContentResult, error) {
	if args.Horizon == "" || args.Period == "" || args.Title == "" {
		return nil, GetGoalContentResult{Found: false}, nil
	}

	h := Horizon(args.Horizon)
	path := s.manager.GetGoalPathForHorizon(h, args.Period, args.Title)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, GetGoalContentResult{Found: false}, nil
	}

	info := GoalInfo{
		Title:   args.Title,
		Horizon: args.Horizon,
		Period:  args.Period,
		Path:    path,
	}
	return nil, GetGoalContentResult{
		Goal:    &info,
		Content: string(data),
		Found:   true,
	}, nil
}
