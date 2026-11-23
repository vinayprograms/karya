package task

import (
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEventProtocol(t *testing.T) {
	cfg := fixture(t, "EVENT: launch @2026-11-20+1d\nTODO: work\n")
	server := NewMCPServer(cfg)
	incoming, outgoing := mcp.NewInMemoryTransports()
	connection, err := server.server.Connect(t.Context(), incoming, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "regression", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), outgoing, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	tools, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, tool := range tools.Tools {
		found[tool.Name] = true
		if strings.Contains(fmt.Sprint(tool.InputSchema), "show_routines") {
			t.Fatalf("obsolete routine schema: %s", tool.Name)
		}
	}
	for _, name := range []string{"list_events", "schedule_event", "refresh_events"} {
		if !found[name] {
			t.Fatalf("missing tool %s", name)
		}
	}
	for _, name := range []string{"list_events", "refresh_events", "list_tasks"} {
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: map[string]any{}})
		if err != nil || result.IsError {
			t.Fatalf("%s: %+v, %v", name, result, err)
		}
		if name == "list_events" && !strings.Contains(fmt.Sprint(result.StructuredContent), "launch") {
			t.Fatalf("missing event: %+v", result)
		}
	}
}
