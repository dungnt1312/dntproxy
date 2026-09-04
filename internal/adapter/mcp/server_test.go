package mcp

import (
	"context"
	"strings"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dungnt/dntproxy/internal/domain"
)

func TestMCPServerToolsRoundTrip(t *testing.T) {
	chat := &fakeChat{resp: sseResp("analysis result")}
	store := testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "glm/glm-4.5v"}})
	srv := NewServer(store, chat)

	ctx := context.Background()
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	client := gomcp.NewClient(&gomcp.Implementation{Name: "it", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	names := map[string]bool{}
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	if !names["analyze_image"] || !names["analyze_video"] {
		t.Fatalf("tools = %v, want analyze_image + analyze_video", names)
	}

	res, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "analyze_image",
		Arguments: map[string]any{"image_url": dataPNG, "prompt": "what is this"},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	text := resultText(t, res)
	if !strings.Contains(text, "analysis result") {
		t.Fatalf("text = %q", text)
	}
	if chat.model != "glm/glm-4.5v" {
		t.Fatalf("model = %q, want settings default", chat.model)
	}
}

func TestMCPServerMissingModelIsToolError(t *testing.T) {
	srv := NewServer(testStore(t, domain.Settings{}), &fakeChat{resp: sseResp("x")})

	ctx := context.Background()
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := gomcp.NewClient(&gomcp.Implementation{Name: "it", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      "analyze_image",
		Arguments: map[string]any{"image_url": dataPNG},
	})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	if !res.IsError {
		t.Fatal("missing model must be a tool error result")
	}
	if !strings.Contains(resultText(t, res), "No default vision model") {
		t.Fatalf("text = %q", resultText(t, res))
	}
}

func TestMCPServerMissingImageURLIsToolError(t *testing.T) {
	srv := NewServer(testStore(t, domain.Settings{MCP: domain.MCPSettings{DefaultModel: "m"}}), &fakeChat{resp: sseResp("x")})

	ctx := context.Background()
	serverTransport, clientTransport := gomcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := gomcp.NewClient(&gomcp.Implementation{Name: "it", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	res, err := session.CallTool(ctx, &gomcp.CallToolParams{Name: "analyze_image", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("call tool: %v", err)
	}
	// Missing required image_url is rejected by the SDK's schema validation and
	// returned as an IsError tool result.
	if !res.IsError {
		t.Fatalf("res = %+v, want missing-argument tool error", res)
	}
}

func resultText(t *testing.T, res *gomcp.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*gomcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
