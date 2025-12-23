package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockTransport implements ClientTransport for testing.
type mockTransport struct {
	responses []*Response
	requests  []*Request
	index     int
	closed    bool
}

func newMockTransport(responses ...*Response) *mockTransport {
	return &mockTransport{responses: responses}
}

func (t *mockTransport) SendRequest(req *Request) (*Response, error) {
	t.requests = append(t.requests, req)

	if t.closed {
		return nil, io.ErrClosedPipe
	}

	if t.index >= len(t.responses) {
		return &Response{
			JSONRPC: JSONRPCVersion,
			Error:   &Error{Code: -1, Message: "no more responses"},
		}, nil
	}

	resp := t.responses[t.index]
	t.index++
	return resp, nil
}

func (t *mockTransport) Close() error {
	t.closed = true
	return nil
}

func TestNewClient(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(transport)

	assert.NotNil(t, client)
	assert.Equal(t, "relicta-mcp-client", client.clientInfo.Name)
	assert.Equal(t, "1.0.0", client.clientInfo.Version)
}

func TestNewClient_WithOptions(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(transport,
		WithClientInfo("test-client", "2.0.0"),
	)

	assert.Equal(t, "test-client", client.clientInfo.Name)
	assert.Equal(t, "2.0.0", client.clientInfo.Version)
}

func TestClient_Initialize(t *testing.T) {
	initResult := InitializeResult{
		ProtocolVersion: MCPVersion,
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{ListChanged: true},
			Resources: &ResourcesCapability{Subscribe: true},
			Prompts:   &PromptsCapability{ListChanged: true},
		},
		ServerInfo: Implementation{
			Name:    "test-server",
			Version: "1.0.0",
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: initResult},
		&Response{JSONRPC: JSONRPCVersion}, // For notification
	)

	client := NewClient(transport)
	err := client.Initialize(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, client.Capabilities())
	assert.NotNil(t, client.Capabilities().Tools)
	assert.NotNil(t, client.ServerInfo())
	assert.Equal(t, "test-server", client.ServerInfo().Name)

	// Verify requests
	require.Len(t, transport.requests, 2)
	assert.Equal(t, "initialize", transport.requests[0].Method)
	assert.Equal(t, "notifications/initialized", transport.requests[1].Method)
}

func TestClient_Initialize_Error(t *testing.T) {
	transport := newMockTransport(
		&Response{
			JSONRPC: JSONRPCVersion,
			ID:      int64(1),
			Error:   &Error{Code: -32600, Message: "Invalid request"},
		},
	)

	client := NewClient(transport)
	err := client.Initialize(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid request")
}

func TestClient_Initialize_Closed(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(transport)
	client.closed = true

	err := client.Initialize(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestClient_ListTools(t *testing.T) {
	result := ListToolsResult{
		Tools: []Tool{
			{Name: "tool1", Description: "First tool"},
			{Name: "tool2", Description: "Second tool"},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	tools, err := client.ListTools(context.Background())

	require.NoError(t, err)
	assert.Len(t, tools, 2)
	assert.Equal(t, "tool1", tools[0].Name)
	assert.Equal(t, "tool2", tools[1].Name)
}

func TestClient_CallTool(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{"status": "ok"}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	toolResult, err := client.CallTool(context.Background(), "test.tool", map[string]any{
		"arg1": "value1",
	})

	require.NoError(t, err)
	assert.False(t, toolResult.IsError)
	assert.Len(t, toolResult.Content, 1)

	// Verify request params
	require.Len(t, transport.requests, 1)
	var params CallToolParams
	require.NoError(t, json.Unmarshal(transport.requests[0].Params, &params))
	assert.Equal(t, "test.tool", params.Name)
	assert.Equal(t, "value1", params.Arguments["arg1"])
}

func TestClient_CallToolTyped(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{"name": "test", "count": 42}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)

	var output struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	err := client.CallToolTyped(context.Background(), "test.tool", nil, &output)

	require.NoError(t, err)
	assert.Equal(t, "test", output.Name)
	assert.Equal(t, 42, output.Count)
}

func TestClient_CallToolTyped_Error(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: "Something went wrong"}},
		IsError: true,
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)

	var output struct{}
	err := client.CallToolTyped(context.Background(), "test.tool", nil, &output)

	require.Error(t, err)
	var toolErr *ToolError
	assert.ErrorAs(t, err, &toolErr)
	assert.Equal(t, "Something went wrong", toolErr.Message)
}

func TestClient_ListResources(t *testing.T) {
	result := ListResourcesResult{
		Resources: []Resource{
			{URI: "test://resource1", Name: "Resource 1"},
			{URI: "test://resource2", Name: "Resource 2"},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	resources, err := client.ListResources(context.Background())

	require.NoError(t, err)
	assert.Len(t, resources, 2)
	assert.Equal(t, "test://resource1", resources[0].URI)
}

func TestClient_ReadResource(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{
			{URI: "test://resource", Text: "Resource content"},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	content, err := client.ReadResource(context.Background(), "test://resource")

	require.NoError(t, err)
	assert.Len(t, content.Contents, 1)
	assert.Equal(t, "Resource content", content.Contents[0].Text)
}

func TestClient_ReadResourceText(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{
			{URI: "test://resource", Text: "Plain text content"},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	text, err := client.ReadResourceText(context.Background(), "test://resource")

	require.NoError(t, err)
	assert.Equal(t, "Plain text content", text)
}

func TestClient_ReadResourceTyped(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{
			{URI: "test://resource", Text: `{"key": "value", "number": 123}`},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)

	var output struct {
		Key    string `json:"key"`
		Number int    `json:"number"`
	}
	err := client.ReadResourceTyped(context.Background(), "test://resource", &output)

	require.NoError(t, err)
	assert.Equal(t, "value", output.Key)
	assert.Equal(t, 123, output.Number)
}

func TestClient_ListPrompts(t *testing.T) {
	result := ListPromptsResult{
		Prompts: []Prompt{
			{Name: "prompt1", Description: "First prompt"},
			{Name: "prompt2", Description: "Second prompt"},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	prompts, err := client.ListPrompts(context.Background())

	require.NoError(t, err)
	assert.Len(t, prompts, 2)
	assert.Equal(t, "prompt1", prompts[0].Name)
}

func TestClient_GetPrompt(t *testing.T) {
	result := GetPromptResult{
		Description: "Test prompt",
		Messages: []PromptMessage{
			{Role: RoleUser, Content: PromptContent{Type: "text", Text: "Hello"}},
		},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	prompt, err := client.GetPrompt(context.Background(), "test-prompt", map[string]string{
		"arg1": "value1",
	})

	require.NoError(t, err)
	assert.Equal(t, "Test prompt", prompt.Description)
	assert.Len(t, prompt.Messages, 1)
}

func TestClient_Close(t *testing.T) {
	transport := newMockTransport()
	client := NewClient(transport)

	err := client.Close()
	require.NoError(t, err)
	assert.True(t, transport.closed)

	// Second close should be no-op
	err = client.Close()
	require.NoError(t, err)
}

func TestRPCError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *RPCError
		expected string
	}{
		{
			name:     "without data",
			err:      &RPCError{Code: -32600, Message: "Invalid request"},
			expected: "RPC error -32600: Invalid request",
		},
		{
			name:     "with data",
			err:      &RPCError{Code: -32602, Message: "Invalid params", Data: "missing field"},
			expected: "RPC error -32602: Invalid params (data: missing field)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestToolError_Error(t *testing.T) {
	err := &ToolError{Message: "tool failed"}
	assert.Equal(t, "tool failed", err.Error())
}

// Stdio Transport Tests

func TestStdioClientTransport_SendRequest(t *testing.T) {
	// Create a pipe for stdin/stdout simulation
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()

	transport := NewStdioClientTransport(clientReader, clientWriter)

	// Simulate server response in goroutine
	go func() {
		defer serverWriter.Close()

		// Read request
		buf := make([]byte, 1024)
		n, err := serverReader.Read(buf)
		require.NoError(t, err)

		var req Request
		require.NoError(t, json.Unmarshal(buf[:n-1], &req)) // -1 for newline

		// Send response
		resp := Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  map[string]string{"status": "ok"},
		}
		data, _ := json.Marshal(resp)
		serverWriter.Write(append(data, '\n'))
	}()

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      int64(1),
		Method:  "test",
	}

	resp, err := transport.SendRequest(req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// JSON unmarshals numbers as float64
	assert.EqualValues(t, 1, resp.ID)
}

func TestStdioClientTransport_SendNotification(t *testing.T) {
	var buf bytes.Buffer
	transport := NewStdioClientTransport(strings.NewReader(""), &buf)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		Method:  "notification",
		// No ID - it's a notification
	}

	resp, err := transport.SendRequest(req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, buf.Len() > 0)
}

func TestStdioClientTransport_Close(t *testing.T) {
	transport := NewStdioClientTransport(strings.NewReader(""), &bytes.Buffer{})

	err := transport.Close()
	require.NoError(t, err)

	// Verify closed state
	_, err = transport.SendRequest(&Request{JSONRPC: JSONRPCVersion, ID: int64(1)})
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

// HTTP Transport Tests

func TestHTTPClientTransport_SendRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req Request
		json.NewDecoder(r.Body).Decode(&req)

		resp := Response{
			JSONRPC: JSONRPCVersion,
			ID:      req.ID,
			Result:  map[string]string{"status": "ok"},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	transport := NewHTTPClientTransport(server.URL)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      int64(1),
		Method:  "test",
	}

	resp, err := transport.SendRequest(req)

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// JSON unmarshals numbers as float64
	assert.EqualValues(t, 1, resp.ID)
}

func TestHTTPClientTransport_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	transport := NewHTTPClientTransport(server.URL)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      int64(1),
		Method:  "test",
	}

	_, err := transport.SendRequest(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP error")
}

func TestHTTPClientTransport_Close(t *testing.T) {
	transport := NewHTTPClientTransport("http://localhost:8080")

	err := transport.Close()
	require.NoError(t, err)

	// Verify closed state
	_, err = transport.SendRequest(&Request{JSONRPC: JSONRPCVersion, ID: int64(1)})
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestHTTPClientTransport_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 60 * time.Second}
	transport := NewHTTPClientTransport("http://localhost:8080")
	transport.WithHTTPClient(customClient)

	assert.Equal(t, customClient, transport.client)
}

// Convenience Method Tests

func TestClient_Status(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"has_active_release": true,
			"state": "planned",
			"version": "1.2.0",
			"release_id": "rel-123"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	status, err := client.Status(context.Background())

	require.NoError(t, err)
	assert.True(t, status.HasActiveRelease)
	assert.Equal(t, "planned", status.State)
	assert.Equal(t, "1.2.0", status.Version)
}

func TestClient_Plan(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"release_id": "rel-456",
			"current_version": "1.0.0",
			"suggested_bump": "minor",
			"commit_count": 15,
			"features": 3,
			"fixes": 5
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	plan, err := client.Plan(context.Background(), true, "v1.0.0")

	require.NoError(t, err)
	assert.Equal(t, "rel-456", plan.ReleaseID)
	assert.Equal(t, "minor", plan.SuggestedBump)
	assert.Equal(t, 15, plan.CommitCount)

	// Verify request params
	require.Len(t, transport.requests, 1)
	var params CallToolParams
	require.NoError(t, json.Unmarshal(transport.requests[0].Params, &params))
	assert.Equal(t, "relicta.plan", params.Name)
	assert.Equal(t, true, params.Arguments["analyze"])
	assert.Equal(t, "v1.0.0", params.Arguments["from"])
}

func TestClient_Bump(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"current_version": "1.0.0",
			"new_version": "1.1.0",
			"bump_type": "minor",
			"applied": true
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	bump, err := client.Bump(context.Background(), "rel-123", "minor", true)

	require.NoError(t, err)
	assert.Equal(t, "1.1.0", bump.NewVersion)
	assert.Equal(t, "minor", bump.BumpType)
	assert.True(t, bump.Applied)
}

func TestClient_Notes(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"notes": "## Release Notes\n\n### Features\n- New feature",
			"format": "markdown",
			"word_count": 25
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	notes, err := client.Notes(context.Background(), "rel-123", "markdown")

	require.NoError(t, err)
	assert.Contains(t, notes.Notes, "Release Notes")
	assert.Equal(t, "markdown", notes.Format)
}

func TestClient_Evaluate(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"risk_level": "medium",
			"risk_score": 0.45,
			"factors": ["breaking_changes", "large_changeset"],
			"requires_approval": true
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	eval, err := client.Evaluate(context.Background(), "rel-123")

	require.NoError(t, err)
	assert.Equal(t, "medium", eval.RiskLevel)
	assert.Equal(t, 0.45, eval.RiskScore)
	assert.True(t, eval.RequiresApproval)
}

func TestClient_Approve(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"approved": true,
			"version": "1.1.0",
			"message": "Release approved"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	approve, err := client.Approve(context.Background(), "rel-123", true)

	require.NoError(t, err)
	assert.True(t, approve.Approved)
	assert.Equal(t, "1.1.0", approve.Version)
}

func TestClient_Publish(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: `{
			"published": true,
			"version": "1.1.0",
			"tag": "v1.1.0"
		}`}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	publish, err := client.Publish(context.Background(), "rel-123", false)

	require.NoError(t, err)
	assert.True(t, publish.Published)
	assert.Equal(t, "v1.1.0", publish.Tag)
}

func TestClient_State(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{{
			URI: "relicta://state",
			Text: `{
				"has_active_release": true,
				"state": "approved",
				"version": "1.1.0",
				"release_id": "rel-789"
			}`,
		}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	state, err := client.State(context.Background())

	require.NoError(t, err)
	assert.True(t, state.HasActiveRelease)
	assert.Equal(t, "approved", state.State)
}

func TestClient_Config(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{{
			URI: "relicta://config",
			Text: `{
				"versioning": {
					"strategy": "conventional",
					"prefix": "v"
				},
				"plugins": [
					{"name": "github", "enabled": true}
				]
			}`,
		}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	config, err := client.Config(context.Background())

	require.NoError(t, err)
	assert.Equal(t, "conventional", config.Versioning.Strategy)
	assert.Equal(t, "v", config.Versioning.Prefix)
	assert.Len(t, config.Plugins, 1)
	assert.Equal(t, "github", config.Plugins[0].Name)
}

// Edge Cases

func TestClient_CallToolTyped_NoContent(t *testing.T) {
	result := CallToolResult{
		Content: []Content{},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)

	var output struct{}
	err := client.CallToolTyped(context.Background(), "test.tool", nil, &output)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no content")
}

func TestClient_CallToolTyped_InvalidJSON(t *testing.T) {
	result := CallToolResult{
		Content: []Content{{Type: "text", Text: "not valid json"}},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)

	var output struct{}
	err := client.CallToolTyped(context.Background(), "test.tool", nil, &output)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestClient_ReadResourceText_NoContent(t *testing.T) {
	result := ReadResourceResult{
		Contents: []ResourceContent{},
	}

	transport := newMockTransport(
		&Response{JSONRPC: JSONRPCVersion, ID: int64(1), Result: result},
	)

	client := NewClient(transport)
	_, err := client.ReadResourceText(context.Background(), "test://resource")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no content")
}
