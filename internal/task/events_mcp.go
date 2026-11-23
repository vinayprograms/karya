package task

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type EventQuery struct {
	Project string `json:"project,omitempty" jsonschema:"optional project name"`
	Filter  string `json:"filter,omitempty" jsonschema:"optional task filter expression for title, tags, assignee or dates"`
}

type EventResult struct {
	Events []TaskInfo `json:"events" jsonschema:"matching events, including one-off and recurring items"`
	Count  int        `json:"count" jsonschema:"number of events"`
}

func (s *MCPServer) events(ctx context.Context, req *mcp.CallToolRequest, args EventQuery) (*mcp.CallToolResult, EventResult, error) {
	tasks, err := ListTasks(s.config, args.Project, true)
	if err != nil {
		return nil, EventResult{}, err
	}
	_, events, _ := Partition(tasks, s.config)
	if args.Filter != "" {
		events = FilterTasks(events, args.Filter)
	}
	slices.SortFunc(events, func(left, right *Task) int {
		return strings.Compare(left.FilePath+"\x00"+left.RawTitle, right.FilePath+"\x00"+right.RawTitle)
	})
	result := EventResult{Events: make([]TaskInfo, 0, len(events)), Count: len(events)}
	for _, event := range events {
		result.Events = append(result.Events, s.taskToInfo(event))
	}
	return nil, result, nil
}

func (s *MCPServer) reschedule(ctx context.Context, req *mcp.CallToolRequest, args ScheduleTaskArgs) (*mcp.CallToolResult, ScheduleTaskResult, error) {
	if KeywordStatus(s.config, args.Keyword) != Events {
		return nil, ScheduleTaskResult{Message: "keyword must belong to events"}, nil
	}
	tasks, err := ListTasks(s.config, args.Project, true)
	if err != nil {
		return nil, ScheduleTaskResult{}, err
	}
	var selected *Task
	for _, event := range tasks {
		if event.Keyword != args.Keyword {
			continue
		}
		matched := args.RawTitle != "" && event.RawTitle == args.RawTitle
		if args.RawTitle == "" {
			matched = args.Title != "" && event.Title == args.Title
		}
		if !matched {
			continue
		}
		if selected != nil {
			return nil, ScheduleTaskResult{Message: "multiple events match; narrow project and raw_title"}, nil
		}
		selected = event
	}
	if selected == nil {
		return nil, ScheduleTaskResult{Message: "event not found"}, nil
	}
	if err := SetTaskDate(selected, args.ScheduledAt, args.DueAt, args.RemoveScheduled, args.RemoveDue); err != nil {
		return nil, ScheduleTaskResult{Message: fmt.Sprintf("update event: %v", err)}, nil
	}
	return nil, ScheduleTaskResult{Success: true, Message: "Event dates updated"}, nil
}

func (s *MCPServer) refresh(ctx context.Context, req *mcp.CallToolRequest, args GetKeywordsArgs) (*mcp.CallToolResult, ScheduleTaskResult, error) {
	if err := Refresh(s.config, time.Now()); err != nil {
		return nil, ScheduleTaskResult{Message: err.Error()}, nil
	}
	return nil, ScheduleTaskResult{Success: true, Message: "Recurring events advanced to today or their next occurrence"}, nil
}
