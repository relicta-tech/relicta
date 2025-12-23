package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Client provides a type-safe API for interacting with MCP servers.
// It handles connection management, request/response correlation,
// and provides convenience methods for all MCP operations.
type Client struct {
	transport    ClientTransport
	capabilities *ServerCapabilities
	serverInfo   *Implementation
	requestID    atomic.Int64
	mu           sync.Mutex
	closed       bool

	// Client information
	clientInfo Implementation
}

// ClientTransport handles the communication layer for MCP clients.
type ClientTransport interface {
	// SendRequest sends a request and returns the response.
	SendRequest(req *Request) (*Response, error)
	// Close closes the transport.
	Close() error
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithClientInfo sets the client information sent during initialization.
func WithClientInfo(name, version string) ClientOption {
	return func(c *Client) {
		c.clientInfo = Implementation{
			Name:    name,
			Version: version,
		}
	}
}

// NewClient creates a new MCP client with the given transport.
// The client is not initialized until Initialize is called.
func NewClient(transport ClientTransport, opts ...ClientOption) *Client {
	c := &Client{
		transport: transport,
		clientInfo: Implementation{
			Name:    "relicta-mcp-client",
			Version: "1.0.0",
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Initialize performs the MCP initialization handshake.
// This must be called before using any other client methods.
func (c *Client) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return errors.New("client is closed")
	}

	params := InitializeParams{
		ProtocolVersion: MCPVersion,
		Capabilities: ClientCapabilities{
			Roots:    &RootsCapability{ListChanged: true},
			Sampling: &SamplingCapability{},
		},
		ClientInfo: c.clientInfo,
	}

	var result InitializeResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return fmt.Errorf("initialize failed: %w", err)
	}

	c.capabilities = &result.Capabilities
	c.serverInfo = &result.ServerInfo

	// Send initialized notification
	if err := c.notify("notifications/initialized", nil); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	return nil
}

// ServerInfo returns information about the connected server.
// Returns nil if not initialized.
func (c *Client) ServerInfo() *Implementation {
	return c.serverInfo
}

// Capabilities returns the server's capabilities.
// Returns nil if not initialized.
func (c *Client) Capabilities() *ServerCapabilities {
	return c.capabilities
}

// Tool Operations

// ListTools returns all available tools from the server.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	var result ListToolsResult
	if err := c.call(ctx, "tools/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// CallTool invokes a tool with the given arguments.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallToolResult, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}

	var result CallToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CallToolTyped invokes a tool and unmarshals the result into the provided type.
// This assumes the tool returns JSON in its text content.
func (c *Client) CallToolTyped(ctx context.Context, name string, args map[string]any, result any) error {
	toolResult, err := c.CallTool(ctx, name, args)
	if err != nil {
		return err
	}

	if toolResult.IsError {
		if len(toolResult.Content) > 0 {
			return &ToolError{Message: toolResult.Content[0].Text}
		}
		return &ToolError{Message: "tool returned error with no message"}
	}

	if len(toolResult.Content) == 0 {
		return errors.New("tool returned no content")
	}

	text := toolResult.Content[0].Text
	if err := json.Unmarshal([]byte(text), result); err != nil {
		return fmt.Errorf("failed to unmarshal tool result: %w", err)
	}

	return nil
}

// ToolError represents an error returned by a tool.
type ToolError struct {
	Message string
}

func (e *ToolError) Error() string {
	return e.Message
}

// Resource Operations

// ListResources returns all available resources from the server.
func (c *Client) ListResources(ctx context.Context) ([]Resource, error) {
	var result ListResourcesResult
	if err := c.call(ctx, "resources/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Resources, nil
}

// ReadResource reads a resource by URI.
func (c *Client) ReadResource(ctx context.Context, uri string) (*ReadResourceResult, error) {
	params := ReadResourceParams{URI: uri}

	var result ReadResourceResult
	if err := c.call(ctx, "resources/read", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReadResourceText reads a resource and returns its text content.
func (c *Client) ReadResourceText(ctx context.Context, uri string) (string, error) {
	result, err := c.ReadResource(ctx, uri)
	if err != nil {
		return "", err
	}

	if len(result.Contents) == 0 {
		return "", errors.New("resource returned no content")
	}

	return result.Contents[0].Text, nil
}

// ReadResourceTyped reads a resource and unmarshals it into the provided type.
func (c *Client) ReadResourceTyped(ctx context.Context, uri string, result any) error {
	text, err := c.ReadResourceText(ctx, uri)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(text), result); err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	return nil
}

// Prompt Operations

// ListPrompts returns all available prompts from the server.
func (c *Client) ListPrompts(ctx context.Context) ([]Prompt, error) {
	var result ListPromptsResult
	if err := c.call(ctx, "prompts/list", nil, &result); err != nil {
		return nil, err
	}
	return result.Prompts, nil
}

// GetPrompt retrieves a prompt with the given arguments.
func (c *Client) GetPrompt(ctx context.Context, name string, args map[string]string) (*GetPromptResult, error) {
	params := GetPromptParams{
		Name:      name,
		Arguments: args,
	}

	var result GetPromptResult
	if err := c.call(ctx, "prompts/get", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	return c.transport.Close()
}

// Internal methods

func (c *Client) call(ctx context.Context, method string, params any, result any) error {
	id := c.requestID.Add(1)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
	}

	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		req.Params = paramsData
	}

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return &RPCError{
			Code:    resp.Error.Code,
			Message: resp.Error.Message,
			Data:    resp.Error.Data,
		}
	}

	if result != nil {
		resultData, err := json.Marshal(resp.Result)
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		if err := json.Unmarshal(resultData, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

func (c *Client) notify(method string, params any) error {
	notification := &Request{
		JSONRPC: JSONRPCVersion,
		Method:  method,
	}

	if params != nil {
		paramsData, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("failed to marshal params: %w", err)
		}
		notification.Params = paramsData
	}

	// Notifications don't have responses, but we use SendRequest anyway
	// The transport should handle this appropriately
	_, err := c.transport.SendRequest(notification)
	return err
}

// RPCError represents a JSON-RPC error returned by the server.
type RPCError struct {
	Code    int
	Message string
	Data    any
}

func (e *RPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("RPC error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// StdioClientTransport implements ClientTransport over stdio.
type StdioClientTransport struct {
	reader  *bufio.Reader
	writer  io.Writer
	writeMu sync.Mutex
	closed  bool
}

// NewStdioClientTransport creates a new stdio client transport.
func NewStdioClientTransport(reader io.Reader, writer io.Writer) *StdioClientTransport {
	return &StdioClientTransport{
		reader: bufio.NewReader(reader),
		writer: writer,
	}
}

// SendRequest sends a request and waits for the response.
func (t *StdioClientTransport) SendRequest(req *Request) (*Response, error) {
	t.writeMu.Lock()
	if t.closed {
		t.writeMu.Unlock()
		return nil, io.ErrClosedPipe
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.writeMu.Unlock()
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := t.writer.Write(append(data, '\n')); err != nil {
		t.writeMu.Unlock()
		return nil, fmt.Errorf("failed to write request: %w", err)
	}
	t.writeMu.Unlock()

	// For notifications (no ID), don't wait for response
	if req.ID == nil {
		return &Response{JSONRPC: JSONRPCVersion}, nil
	}

	// Read response
	line, err := t.reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Close closes the transport.
func (t *StdioClientTransport) Close() error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	t.closed = true
	return nil
}

// HTTPClientTransport implements ClientTransport over HTTP.
type HTTPClientTransport struct {
	client  *http.Client
	baseURL string
	closed  bool
	mu      sync.Mutex
}

// NewHTTPClientTransport creates a new HTTP client transport.
func NewHTTPClientTransport(baseURL string) *HTTPClientTransport {
	return &HTTPClientTransport{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
	}
}

// WithHTTPClient sets a custom HTTP client.
func (t *HTTPClientTransport) WithHTTPClient(client *http.Client) *HTTPClientTransport {
	t.client = client
	return t
}

// SendRequest sends a request over HTTP.
func (t *HTTPClientTransport) SendRequest(req *Request) (*Response, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, io.ErrClosedPipe
	}
	t.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", t.baseURL, bufio.NewReader(
		&jsonReader{data: data},
	))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", httpResp.Status)
	}

	var resp Response
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

// Close closes the transport.
func (t *HTTPClientTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return nil
}

// jsonReader is a simple io.Reader for JSON data.
type jsonReader struct {
	data []byte
	pos  int
}

func (r *jsonReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// Convenience functions for common use cases

// Status calls the relicta.status tool and returns the release state.
func (c *Client) Status(ctx context.Context) (*StatusResult, error) {
	var result StatusResult
	if err := c.CallToolTyped(ctx, "relicta.status", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// StatusResult represents the result of the status tool.
type StatusResult struct {
	HasActiveRelease bool   `json:"has_active_release"`
	State            string `json:"state,omitempty"`
	Version          string `json:"version,omitempty"`
	ReleaseID        string `json:"release_id,omitempty"`
}

// Plan calls the relicta.plan tool to analyze commits.
func (c *Client) Plan(ctx context.Context, analyze bool, from string) (*PlanResult, error) {
	args := map[string]any{}
	if analyze {
		args["analyze"] = true
	}
	if from != "" {
		args["from"] = from
	}

	var result PlanResult
	if err := c.CallToolTyped(ctx, "relicta.plan", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PlanResult represents the result of the plan tool.
type PlanResult struct {
	ReleaseID          string `json:"release_id"`
	CurrentVersion     string `json:"current_version"`
	SuggestedBump      string `json:"suggested_bump"`
	CommitCount        int    `json:"commit_count"`
	BreakingChanges    int    `json:"breaking_changes"`
	Features           int    `json:"features"`
	Fixes              int    `json:"fixes"`
	ConventionalRatio  string `json:"conventional_ratio,omitempty"`
	HighRiskCategories int    `json:"high_risk_categories,omitempty"`
}

// Bump calls the relicta.bump tool to calculate and apply a version.
func (c *Client) Bump(ctx context.Context, releaseID string, bumpType string, apply bool) (*BumpResult, error) {
	args := map[string]any{}
	if releaseID != "" {
		args["release_id"] = releaseID
	}
	if bumpType != "" {
		args["type"] = bumpType
	}
	if apply {
		args["apply"] = true
	}

	var result BumpResult
	if err := c.CallToolTyped(ctx, "relicta.bump", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BumpResult represents the result of the bump tool.
type BumpResult struct {
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version"`
	BumpType       string `json:"bump_type"`
	Applied        bool   `json:"applied"`
}

// Notes calls the relicta.notes tool to generate release notes.
func (c *Client) Notes(ctx context.Context, releaseID string, format string) (*NotesResult, error) {
	args := map[string]any{}
	if releaseID != "" {
		args["release_id"] = releaseID
	}
	if format != "" {
		args["format"] = format
	}

	var result NotesResult
	if err := c.CallToolTyped(ctx, "relicta.notes", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// NotesResult represents the result of the notes tool.
type NotesResult struct {
	Notes       string `json:"notes"`
	Format      string `json:"format"`
	WordCount   int    `json:"word_count"`
	SectionList string `json:"section_list,omitempty"`
}

// Evaluate calls the relicta.evaluate tool to assess risk.
func (c *Client) Evaluate(ctx context.Context, releaseID string) (*EvaluateResult, error) {
	args := map[string]any{}
	if releaseID != "" {
		args["release_id"] = releaseID
	}

	var result EvaluateResult
	if err := c.CallToolTyped(ctx, "relicta.evaluate", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// EvaluateResult represents the result of the evaluate tool.
type EvaluateResult struct {
	RiskLevel          string   `json:"risk_level"`
	RiskScore          float64  `json:"risk_score"`
	Factors            []string `json:"factors,omitempty"`
	Recommendation     string   `json:"recommendation,omitempty"`
	RequiresApproval   bool     `json:"requires_approval"`
	HighRiskCategories int      `json:"high_risk_categories,omitempty"`
}

// Approve calls the relicta.approve tool to approve a release.
func (c *Client) Approve(ctx context.Context, releaseID string, yes bool) (*ApproveResult, error) {
	args := map[string]any{}
	if releaseID != "" {
		args["release_id"] = releaseID
	}
	if yes {
		args["yes"] = true
	}

	var result ApproveResult
	if err := c.CallToolTyped(ctx, "relicta.approve", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ApproveResult represents the result of the approve tool.
type ApproveResult struct {
	Approved bool   `json:"approved"`
	Version  string `json:"version"`
	Message  string `json:"message,omitempty"`
}

// Publish calls the relicta.publish tool to publish a release.
func (c *Client) Publish(ctx context.Context, releaseID string, skipPush bool) (*PublishResult, error) {
	args := map[string]any{}
	if releaseID != "" {
		args["release_id"] = releaseID
	}
	if skipPush {
		args["skip_push"] = true
	}

	var result PublishResult
	if err := c.CallToolTyped(ctx, "relicta.publish", args, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PublishResult represents the result of the publish tool.
type PublishResult struct {
	Published bool   `json:"published"`
	Version   string `json:"version"`
	Tag       string `json:"tag"`
	Message   string `json:"message,omitempty"`
}

// State reads the relicta://state resource.
func (c *Client) State(ctx context.Context) (*StateResource, error) {
	var result StateResource
	if err := c.ReadResourceTyped(ctx, "relicta://state", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// StateResource represents the state resource data.
type StateResource struct {
	HasActiveRelease bool   `json:"has_active_release"`
	State            string `json:"state,omitempty"`
	Version          string `json:"version,omitempty"`
	ReleaseID        string `json:"release_id,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// Config reads the relicta://config resource.
func (c *Client) Config(ctx context.Context) (*ConfigResource, error) {
	var result ConfigResource
	if err := c.ReadResourceTyped(ctx, "relicta://config", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ConfigResource represents the config resource data.
type ConfigResource struct {
	Versioning struct {
		Strategy string `json:"strategy"`
		Prefix   string `json:"prefix,omitempty"`
	} `json:"versioning"`
	Plugins []struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"plugins,omitempty"`
}
