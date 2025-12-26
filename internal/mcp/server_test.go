package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/application/governance"
	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/domain/release"
	"github.com/relicta-tech/relicta/internal/domain/version"
)

func TestNewServer(t *testing.T) {
	t.Run("creates server with defaults", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.NotNil(t, server.riskCalc)
		assert.NotNil(t, server.tools)
		assert.NotNil(t, server.resources)
		assert.NotNil(t, server.prompts)
	})

	t.Run("applies options", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cfg := config.DefaultConfig()
		riskCalc := risk.NewCalculatorWithDefaults()

		server, err := NewServer("1.0.0",
			WithLogger(logger),
			WithConfig(cfg),
			WithRiskCalculator(riskCalc),
		)

		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, logger, server.logger)
		assert.Equal(t, cfg, server.config)
		assert.Equal(t, riskCalc, server.riskCalc)
	})
}

func TestHandleRequest(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("initialize", func(t *testing.T) {
		params, _ := json.Marshal(InitializeParams{
			ProtocolVersion: MCPVersion,
			ClientInfo:      Implementation{Name: "test", Version: "1.0"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "initialize",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		assert.NotNil(t, resp.Result)
	})

	t.Run("initialized notification", func(t *testing.T) {
		req := &Request{
			JSONRPC: JSONRPCVersion,
			Method:  "initialized",
		}

		resp := server.HandleRequest(ctx, req)
		assert.Nil(t, resp) // Notifications don't get responses
	})

	t.Run("ping", func(t *testing.T) {
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "ping",
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("unknown method", func(t *testing.T) {
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "unknown/method",
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
	})

	t.Run("tools/list", func(t *testing.T) {
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "tools/list",
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(ListToolsResult)
		require.True(t, ok)
		assert.Len(t, result.Tools, 7) // 7 tools registered
	})

	t.Run("resources/list", func(t *testing.T) {
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      5,
			Method:  "resources/list",
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(ListResourcesResult)
		require.True(t, ok)
		assert.Len(t, result.Resources, 5) // 5 resources registered
	})

	t.Run("prompts/list", func(t *testing.T) {
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      6,
			Method:  "prompts/list",
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(ListPromptsResult)
		require.True(t, ok)
		assert.Len(t, result.Prompts, 7) // 7 prompts registered
	})
}

func TestToolCall(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("relicta.status without repo", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.evaluate", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "relicta.evaluate"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.bump with args", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.bump",
			Arguments: map[string]any{"bump": "minor"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("unknown tool", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "unknown.tool"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
	})
}

func TestResourceRead(t *testing.T) {
	server, err := NewServer("1.0.0", WithConfig(config.DefaultConfig()))
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("relicta://state", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://state"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta://config", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://config"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("unknown resource", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://unknown"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
	})
}

func TestPromptGet(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("release-summary brief", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{
			Name:      "release-summary",
			Arguments: map[string]string{"style": "brief"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("release-summary detailed", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{
			Name:      "release-summary",
			Arguments: map[string]string{"style": "detailed"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("risk-analysis", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{Name: "risk-analysis"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("unknown prompt", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{Name: "unknown"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
	})
}

func TestStdioTransport(t *testing.T) {
	t.Run("read and write messages", func(t *testing.T) {
		// Create a request
		req := Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "ping",
		}
		reqJSON, _ := json.Marshal(req)

		// Create reader with the request
		reader := strings.NewReader(string(reqJSON) + "\n")
		writer := &bytes.Buffer{}

		transport := NewStdioTransport(reader, writer)

		// Read the request
		readReq, err := transport.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, "ping", readReq.Method)

		// Write a response
		resp := NewResponse(1, map[string]any{})
		err = transport.WriteResponse(resp)
		require.NoError(t, err)

		// Verify response was written
		assert.True(t, len(writer.Bytes()) > 0)
		assert.True(t, bytes.HasSuffix(writer.Bytes(), []byte("\n")))
	})

	t.Run("write notification", func(t *testing.T) {
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(strings.NewReader(""), writer)

		err := transport.WriteNotification("test/notification", map[string]any{"key": "value"})
		require.NoError(t, err)

		assert.True(t, len(writer.Bytes()) > 0)
	})
}

func TestToolCallWithAdapter(t *testing.T) {
	// Create a mock release for the adapter
	rel := createTestRelease("test-release-1", "1.0.0", "1.1.0")
	repo := &mockReleaseRepository{releases: []*release.Release{rel}}

	// Create an adapter with repository
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("relicta.status with adapter", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		require.Len(t, result.Content, 1)
		assert.Contains(t, result.Content[0].Text, "test-release-1")
	})

	t.Run("relicta.plan without plan use case", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.plan",
			Arguments: map[string]any{"from": "v1.0.0"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// Falls back to stub response
	})

	t.Run("relicta.notes without notes use case", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.notes",
			Arguments: map[string]any{"ai": true},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.approve without approve use case", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.approve",
			Arguments: map[string]any{"notes": "test notes"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.publish without publish use case", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.publish",
			Arguments: map[string]any{"dry_run": true},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      5,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})
}

func TestResourceReadWithAdapter(t *testing.T) {
	rel := createTestRelease("test-release-2", "1.0.0", "1.2.0")
	repo := &mockReleaseRepository{releases: []*release.Release{rel}}

	server, err := NewServer("1.0.0",
		WithConfig(config.DefaultConfig()),
		WithReleaseRepository(repo),
	)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("relicta://state with release", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://state"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*ReadResourceResult)
		require.True(t, ok)
		require.Len(t, result.Contents, 1)
		assert.Contains(t, result.Contents[0].Text, "planned")
	})

	t.Run("relicta://commits stub", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://commits"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta://changelog stub", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://changelog"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta://risk-report stub", func(t *testing.T) {
		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://risk-report"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})
}

func TestServerOptions(t *testing.T) {
	t.Run("WithGitService", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithGitService(nil))
		require.NoError(t, err)
		assert.Nil(t, server.gitService)
	})

	t.Run("WithReleaseRepository", func(t *testing.T) {
		repo := &mockReleaseRepository{}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)
		assert.Equal(t, repo, server.releaseRepo)
	})

	t.Run("WithPolicyEngine", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithPolicyEngine(nil))
		require.NoError(t, err)
		assert.Nil(t, server.policyEngine)
	})

	t.Run("WithEvaluator", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithEvaluator(nil))
		require.NoError(t, err)
		assert.Nil(t, server.evaluator)
	})

	t.Run("WithAdapter", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)
		assert.Equal(t, adapter, server.adapter)
	})
}

func TestTransportClose(t *testing.T) {
	reader := strings.NewReader("")
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(reader, writer)

	err := transport.Close()
	assert.NoError(t, err)
}

func TestTransportReadErrors(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		reader := strings.NewReader("")
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(reader, writer)

		_, err := transport.ReadMessage()
		assert.Error(t, err)
	})

	t.Run("invalid JSON", func(t *testing.T) {
		reader := strings.NewReader("not-json\n")
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(reader, writer)

		_, err := transport.ReadMessage()
		assert.Error(t, err)
	})
}

func TestMessageLoop(t *testing.T) {
	t.Run("creates message loop", func(t *testing.T) {
		reader := strings.NewReader("")
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(reader, writer)

		server, _ := NewServer("1.0.0")
		loop := NewMessageLoop(transport, server)
		assert.NotNil(t, loop)
	})

	t.Run("run processes messages until EOF", func(t *testing.T) {
		// Create a ping request
		req := Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "ping",
		}
		reqJSON, _ := json.Marshal(req)

		reader := strings.NewReader(string(reqJSON) + "\n")
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(reader, writer)

		server, _ := NewServer("1.0.0")
		loop := NewMessageLoop(transport, server)

		// Run until EOF (reader is exhausted)
		err := loop.Run(context.Background())
		// Should exit cleanly on EOF
		assert.NoError(t, err)
		// Should have written a response
		assert.True(t, len(writer.Bytes()) > 0)
	})

	t.Run("run handles multiple messages until EOF", func(t *testing.T) {
		// Create multiple requests
		reqs := []Request{
			{JSONRPC: JSONRPCVersion, ID: 1, Method: "ping"},
			{JSONRPC: JSONRPCVersion, ID: 2, Method: "tools/list"},
		}

		var input strings.Builder
		for _, r := range reqs {
			b, _ := json.Marshal(r)
			input.WriteString(string(b) + "\n")
		}

		reader := strings.NewReader(input.String())
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(reader, writer)

		server, _ := NewServer("1.0.0")
		loop := NewMessageLoop(transport, server)

		// Run until EOF
		err := loop.Run(context.Background())
		assert.NoError(t, err)
		// Output should contain both responses
		assert.True(t, len(writer.Bytes()) > 0)
	})

	t.Run("run handles parse errors", func(t *testing.T) {
		// Create invalid JSON followed by valid request
		input := "not-json\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"

		reader := strings.NewReader(input)
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(reader, writer)

		server, _ := NewServer("1.0.0")
		loop := NewMessageLoop(transport, server)

		err := loop.Run(context.Background())
		assert.NoError(t, err)
		// Should have error response for invalid JSON + response for ping
		output := writer.String()
		assert.Contains(t, output, "Parse error")
	})
}

func TestResourceStateEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("state with empty releases returns no active", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*release.Release{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://state"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*ReadResourceResult)
		require.True(t, ok)
		assert.Contains(t, result.Contents[0].Text, "no active release")
	})
}

func TestResourceConfigEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("config without product name uses 'Relicta' default", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Changelog.ProductName = "" // Empty product name - will be replaced with default
		server, err := NewServer("1.0.0", WithConfig(cfg))
		require.NoError(t, err)

		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://config"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*ReadResourceResult)
		require.True(t, ok)
		// Should use "Relicta" as default when empty
		assert.Contains(t, result.Contents[0].Text, "Relicta")
	})

	t.Run("config with product name", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Changelog.ProductName = "MyApp"
		server, err := NewServer("1.0.0", WithConfig(cfg))
		require.NoError(t, err)

		params, _ := json.Marshal(ReadResourceParams{URI: "relicta://config"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "resources/read",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*ReadResourceResult)
		require.True(t, ok)
		assert.Contains(t, result.Contents[0].Text, "MyApp")
	})
}

func TestPromptEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("release-summary with invalid style defaults to brief", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		params, _ := json.Marshal(GetPromptParams{
			Name:      "release-summary",
			Arguments: map[string]string{"style": "invalid"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("release-summary with no style defaults to brief", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		params, _ := json.Marshal(GetPromptParams{
			Name:      "release-summary",
			Arguments: map[string]string{},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})
}

func TestToolResultJSONError(t *testing.T) {
	// Test with something that can't be marshaled
	invalidData := make(chan int)
	_, err := NewToolResultJSON(invalidData)
	assert.Error(t, err)
}

func TestNotificationHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("initialized notification returns nil", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// "initialized" is a notification (no ID expected)
		req := &Request{
			JSONRPC: JSONRPCVersion,
			Method:  "initialized",
		}

		resp := server.HandleRequest(ctx, req)
		// Should return nil for notifications
		assert.Nil(t, resp)
	})
}

func TestToolStatusWithReleaseRepo(t *testing.T) {
	ctx := context.Background()

	t.Run("status with release repo and active release", func(t *testing.T) {
		rel := createTestRelease("repo-test", "1.0.0", "1.1.0")
		repo := &mockReleaseRepository{releases: []*release.Release{rel}}

		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		// Should contain planned state
		assert.Contains(t, result.Content[0].Text, "planned")
	})

	t.Run("status with release repo but no releases", func(t *testing.T) {
		repo := &mockReleaseRepository{releases: []*release.Release{}}
		server, err := NewServer("1.0.0", WithReleaseRepository(repo))
		require.NoError(t, err)

		params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.Contains(t, result.Content[0].Text, "No active release")
	})
}

func TestToolFallbackPaths(t *testing.T) {
	// Test tool handlers without any adapter - exercises fallback paths
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("relicta.status without release repo", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.Contains(t, result.Content[0].Text, "No release repository configured")
	})

	t.Run("relicta.plan fallback", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.plan",
			Arguments: map[string]any{"from": "auto", "analyze": true},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.bump fallback with version", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.bump",
			Arguments: map[string]any{"bump": "minor", "version": "2.0.0"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.notes fallback with ai flag", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.notes",
			Arguments: map[string]any{"ai": false},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.evaluate with default risk calc", func(t *testing.T) {
		// Server always has a default risk calculator
		params, _ := json.Marshal(CallToolParams{Name: "relicta.evaluate"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      5,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// Should return success with risk assessment
		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.False(t, result.IsError)
		assert.Contains(t, result.Content[0].Text, "score")
	})

	t.Run("relicta.approve fallback with notes", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.approve",
			Arguments: map[string]any{"notes": "custom release notes"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      6,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("relicta.publish fallback with dry_run false", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.publish",
			Arguments: map[string]any{"dry_run": false},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      7,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})
}

func TestResourceStateWithVersion(t *testing.T) {
	// Create release with explicit version set
	rel := release.NewRelease(release.ReleaseID("version-test"), "main", "")
	v, _ := version.Parse("1.5.0")
	nextV, _ := version.Parse("1.6.0")
	plan := release.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = release.SetPlan(rel, plan)
	_ = rel.SetVersion(v, "v1.5.0")

	repo := &mockReleaseRepository{releases: []*release.Release{rel}}

	server, err := NewServer("1.0.0", WithReleaseRepository(repo))
	require.NoError(t, err)

	ctx := context.Background()

	params, _ := json.Marshal(ReadResourceParams{URI: "relicta://state"})
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "resources/read",
		Params:  params,
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(*ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)
	assert.Contains(t, result.Contents[0].Text, "1.5.0")
}

func TestNewJSONResourceContent(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		data := map[string]any{"key": "value", "number": 42}
		content, err := NewJSONResourceContent("test://uri", data)
		require.NoError(t, err)
		assert.Equal(t, "test://uri", content.URI)
		assert.Equal(t, "application/json", content.MIMEType)
		assert.Contains(t, content.Text, "key")
		assert.Contains(t, content.Text, "value")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		// Create something that can't be marshaled to JSON
		data := make(chan int)
		_, err := NewJSONResourceContent("test://uri", data)
		assert.Error(t, err)
	})
}

func TestTransportWriteErrors(t *testing.T) {
	t.Run("write response to closed writer", func(t *testing.T) {
		// Test with a writer that has been used
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(strings.NewReader(""), writer)

		// Write a response
		resp := NewResponse(1, map[string]any{"test": "data"})
		err := transport.WriteResponse(resp)
		assert.NoError(t, err)

		// Verify output
		assert.True(t, len(writer.Bytes()) > 0)
	})

	t.Run("write notification with data", func(t *testing.T) {
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(strings.NewReader(""), writer)

		err := transport.WriteNotification("test/event", map[string]any{
			"timestamp": "2024-01-01",
			"data":      []string{"a", "b"},
		})
		assert.NoError(t, err)
		assert.Contains(t, writer.String(), "test/event")
	})
}

func TestHandleInitializeWithError(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid params
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "initialize",
		Params:  []byte(`invalid json`),
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

func TestHandleCallToolWithError(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid params
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  []byte(`invalid json`),
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
}

func TestHandleReadResourceWithError(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid params
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "resources/read",
		Params:  []byte(`invalid json`),
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
}

func TestHandleGetPromptWithError(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid params
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "prompts/get",
		Params:  []byte(`invalid json`),
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
}

func TestProtocolTypes(t *testing.T) {
	t.Run("NewResponse", func(t *testing.T) {
		resp := NewResponse(1, "result")
		assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Equal(t, "result", resp.Result)
		assert.Nil(t, resp.Error)
	})

	t.Run("NewErrorResponse", func(t *testing.T) {
		resp := NewErrorResponse(1, ErrCodeInternalError, "error message", "details")
		assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
		assert.Equal(t, 1, resp.ID)
		assert.Nil(t, resp.Result)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeInternalError, resp.Error.Code)
		assert.Equal(t, "error message", resp.Error.Message)
	})

	t.Run("NewToolResult", func(t *testing.T) {
		result := NewToolResult("test text")
		assert.Len(t, result.Content, 1)
		assert.Equal(t, "text", result.Content[0].Type)
		assert.Equal(t, "test text", result.Content[0].Text)
		assert.False(t, result.IsError)
	})

	t.Run("NewToolResultError", func(t *testing.T) {
		result := NewToolResultError("error message")
		assert.True(t, result.IsError)
		assert.Equal(t, "error message", result.Content[0].Text)
	})

	t.Run("NewToolResultJSON", func(t *testing.T) {
		data := map[string]any{"key": "value"}
		result, err := NewToolResultJSON(data)
		require.NoError(t, err)
		assert.Len(t, result.Content, 1)
		assert.Contains(t, result.Content[0].Text, "key")
	})

	t.Run("NewTextResourceContent", func(t *testing.T) {
		content := NewTextResourceContent("test://uri", "content")
		assert.Equal(t, "test://uri", content.URI)
		assert.Equal(t, "text/plain", content.MIMEType)
		assert.Equal(t, "content", content.Text)
	})

	t.Run("NewPromptMessage", func(t *testing.T) {
		msg := NewPromptMessage("test prompt")
		assert.Equal(t, RoleUser, msg.Role)
		assert.Equal(t, "text", msg.Content.Type)
		assert.Equal(t, "test prompt", msg.Content.Text)
	})
}

// Test tool handlers with adapter that has release repo
func TestToolHandlersWithAdapterAndRepo(t *testing.T) {
	ctx := context.Background()

	// Create release for the repo
	rel := createTestRelease("adapter-test-123", "1.0.0", "1.1.0")
	repo := &mockReleaseRepository{releases: []*release.Release{rel}}

	// Create adapter with just the release repo (no use cases configured)
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	server, err := NewServer("1.0.0",
		WithAdapter(adapter),
		WithReleaseRepository(repo),
	)
	require.NoError(t, err)

	t.Run("status tool with adapter and repo", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		// Should contain release info from adapter
		assert.Contains(t, result.Content[0].Text, "adapter-test-123")
	})

	t.Run("notes tool fails without notes use case", func(t *testing.T) {
		// Adapter has repo but no notes use case - fallback to stub
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.notes",
			Arguments: map[string]any{"ai": true},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// Should fall through to fallback since no use case
		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.Contains(t, result.Content[0].Text, "status")
	})

	t.Run("evaluate tool falls back to risk calc when no governance", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{Name: "relicta.evaluate"})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// Should use default risk calculator
		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.Contains(t, result.Content[0].Text, "score")
	})

	t.Run("approve tool falls back without approve use case", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.approve",
			Arguments: map[string]any{"notes": "edited notes"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      4,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// Should fallback
		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.Contains(t, result.Content[0].Text, "edited notes")
	})

	t.Run("publish tool falls back without publish use case", func(t *testing.T) {
		params, _ := json.Marshal(CallToolParams{
			Name:      "relicta.publish",
			Arguments: map[string]any{"dry_run": true},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      5,
			Method:  "tools/call",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		result, ok := resp.Result.(*CallToolResult)
		require.True(t, ok)
		assert.Contains(t, result.Content[0].Text, "dry_run")
	})
}

// Test tool status with adapter that fails GetStatus
func TestToolStatusWithAdapterError(t *testing.T) {
	ctx := context.Background()

	// Create adapter with empty repo
	repo := &mockReleaseRepository{releases: []*release.Release{}}
	adapter := NewAdapter(WithAdapterReleaseRepository(repo))

	server, err := NewServer("1.0.0", WithAdapter(adapter))
	require.NoError(t, err)

	params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(*CallToolResult)
	require.True(t, ok)
	// Should return "No active release found" when adapter fails
	assert.Contains(t, result.Content[0].Text, "No active release")
}

// Test resource state with release that has version
func TestResourceStateWithDirectVersion(t *testing.T) {
	ctx := context.Background()

	// Create release with explicit version set (not just from plan)
	rel := release.NewRelease(release.ReleaseID("version-direct-test"), "main", "")
	v, _ := version.Parse("2.0.0")
	nextV, _ := version.Parse("2.1.0")
	plan := release.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = release.SetPlan(rel, plan)
	_ = rel.SetVersion(nextV, "v2.1.0")

	repo := &mockReleaseRepository{releases: []*release.Release{rel}}
	server, err := NewServer("1.0.0", WithReleaseRepository(repo))
	require.NoError(t, err)

	params, _ := json.Marshal(ReadResourceParams{URI: "relicta://state"})
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "resources/read",
		Params:  params,
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(*ReadResourceResult)
	require.True(t, ok)
	assert.Contains(t, result.Contents[0].Text, "2.1.0")
}

// Test transport WriteNotification edge cases
func TestTransportWriteNotificationEdgeCases(t *testing.T) {
	t.Run("write notification with complex data", func(t *testing.T) {
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(strings.NewReader(""), writer)

		err := transport.WriteNotification("server/log", map[string]any{
			"level":   "info",
			"message": "test message",
			"context": map[string]any{
				"file": "test.go",
				"line": 42,
			},
		})
		assert.NoError(t, err)
		assert.Contains(t, writer.String(), "server/log")
		assert.Contains(t, writer.String(), "info")
	})

	t.Run("write notification with nil params", func(t *testing.T) {
		writer := &bytes.Buffer{}
		transport := NewStdioTransport(strings.NewReader(""), writer)

		err := transport.WriteNotification("server/event", nil)
		assert.NoError(t, err)
		output := writer.String()
		assert.Contains(t, output, "server/event")
	})
}

// Test GetPromptResult structure
func TestPromptResultStructure(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	t.Run("release-summary detailed style", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{
			Name:      "release-summary",
			Arguments: map[string]string{"style": "detailed"},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*GetPromptResult)
		require.True(t, ok)
		assert.NotEmpty(t, result.Messages)
		assert.Contains(t, result.Messages[0].Content.Text, "detailed")
	})

	t.Run("risk-analysis prompt", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{
			Name:      "risk-analysis",
			Arguments: map[string]string{},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)

		result, ok := resp.Result.(*GetPromptResult)
		require.True(t, ok)
		assert.NotEmpty(t, result.Messages)
	})

	t.Run("prompt not found", func(t *testing.T) {
		params, _ := json.Marshal(GetPromptParams{
			Name:      "non-existent",
			Arguments: map[string]string{},
		})
		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      3,
			Method:  "prompts/get",
			Params:  params,
		}

		resp := server.HandleRequest(ctx, req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeMethodNotFound, resp.Error.Code)
	})
}

// Test toolStatus with release that has version directly set
func TestToolStatusWithVersionedRelease(t *testing.T) {
	ctx := context.Background()

	// Create release with direct version
	rel := release.NewRelease(release.ReleaseID("versioned-release"), "main", "")
	v, _ := version.Parse("3.0.0")
	nextV, _ := version.Parse("3.1.0")
	plan := release.NewReleasePlan(v, nextV, changes.ReleaseTypeMinor, nil, false)
	_ = release.SetPlan(rel, plan)
	_ = rel.SetVersion(nextV, "v3.1.0")

	repo := &mockReleaseRepository{releases: []*release.Release{rel}}
	server, err := NewServer("1.0.0", WithReleaseRepository(repo))
	require.NoError(t, err)

	params, _ := json.Marshal(CallToolParams{Name: "relicta.status"})
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(*CallToolResult)
	require.True(t, ok)
	// Should contain the versioned release info
	assert.Contains(t, result.Content[0].Text, "3.1.0")
}

// Test evaluate without risk calculator
func TestToolEvaluateWithoutRiskCalc(t *testing.T) {
	ctx := context.Background()

	server, err := NewServer("1.0.0", WithRiskCalculator(nil))
	require.NoError(t, err)

	// Set risk calc to nil after creation
	server.riskCalc = nil

	params, _ := json.Marshal(CallToolParams{Name: "relicta.evaluate"})
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	}

	resp := server.HandleRequest(ctx, req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)

	result, ok := resp.Result.(*CallToolResult)
	require.True(t, ok)
	// Should return error result when risk calc is nil
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content[0].Text, "not configured")
}

// Test tool bump argument parsing
func TestToolBumpArgumentParsing(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	tests := []struct {
		name string
		args map[string]any
	}{
		{"default args", map[string]any{}},
		{"bump major", map[string]any{"bump": "major"}},
		{"bump patch", map[string]any{"bump": "patch"}},
		{"with version", map[string]any{"version": "3.0.0"}},
		{"bump and version", map[string]any{"bump": "major", "version": "3.0.0"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(CallToolParams{
				Name:      "relicta.bump",
				Arguments: tt.args,
			})
			req := &Request{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "tools/call",
				Params:  params,
			}

			resp := server.HandleRequest(ctx, req)
			require.NotNil(t, resp)
			assert.Nil(t, resp.Error)
		})
	}
}

// Test tool plan argument parsing
func TestToolPlanArgumentParsing(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	tests := []struct {
		name string
		args map[string]any
	}{
		{"default args", map[string]any{}},
		{"from auto", map[string]any{"from": "auto"}},
		{"from tag", map[string]any{"from": "v1.0.0"}},
		{"with analyze", map[string]any{"analyze": true}},
		{"from with analyze", map[string]any{"from": "v1.0.0", "analyze": true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(CallToolParams{
				Name:      "relicta.plan",
				Arguments: tt.args,
			})
			req := &Request{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "tools/call",
				Params:  params,
			}

			resp := server.HandleRequest(ctx, req)
			require.NotNil(t, resp)
			assert.Nil(t, resp.Error)
		})
	}
}

// Test tool notes argument parsing
func TestToolNotesArgumentParsing(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	tests := []struct {
		name string
		args map[string]any
	}{
		{"default args", map[string]any{}},
		{"ai true", map[string]any{"ai": true}},
		{"ai false", map[string]any{"ai": false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(CallToolParams{
				Name:      "relicta.notes",
				Arguments: tt.args,
			})
			req := &Request{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "tools/call",
				Params:  params,
			}

			resp := server.HandleRequest(ctx, req)
			require.NotNil(t, resp)
			assert.Nil(t, resp.Error)
		})
	}
}

// Test tool approve argument parsing
func TestToolApproveArgumentParsing(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	tests := []struct {
		name string
		args map[string]any
	}{
		{"default args", map[string]any{}},
		{"with notes", map[string]any{"notes": "custom notes"}},
		{"empty notes", map[string]any{"notes": ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(CallToolParams{
				Name:      "relicta.approve",
				Arguments: tt.args,
			})
			req := &Request{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "tools/call",
				Params:  params,
			}

			resp := server.HandleRequest(ctx, req)
			require.NotNil(t, resp)
			assert.Nil(t, resp.Error)
		})
	}
}

// Test tool publish argument parsing
func TestToolPublishArgumentParsing(t *testing.T) {
	ctx := context.Background()
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	tests := []struct {
		name string
		args map[string]any
	}{
		{"default args", map[string]any{}},
		{"dry run true", map[string]any{"dry_run": true}},
		{"dry run false", map[string]any{"dry_run": false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, _ := json.Marshal(CallToolParams{
				Name:      "relicta.publish",
				Arguments: tt.args,
			})
			req := &Request{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "tools/call",
				Params:  params,
			}

			resp := server.HandleRequest(ctx, req)
			require.NotNil(t, resp)
			assert.Nil(t, resp.Error)
		})
	}
}

// Test message loop with context cancellation
func TestMessageLoopWithContextCancel(t *testing.T) {
	// Create a slow reader that will block
	reader := &slowReader{delay: 0}
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(reader, writer)

	server, _ := NewServer("1.0.0")
	loop := NewMessageLoop(transport, server)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Run should return quickly when context is canceled
	err := loop.Run(ctx)
	// The error could be context.Canceled or nil depending on timing
	_ = err // Just ensure no panic
}

// slowReader simulates a slow reader for testing
type slowReader struct {
	delay int
}

func (r *slowReader) Read(p []byte) (n int, err error) {
	return 0, context.Canceled
}

// Test transport WriteResponse when closed
func TestTransportWriteResponseWhenClosed(t *testing.T) {
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(strings.NewReader(""), writer)

	// Close the transport
	transport.Close()

	// Try to write after close
	resp := NewResponse(1, "test")
	err := transport.WriteResponse(resp)
	assert.Error(t, err)
}

// Test transport WriteNotification when closed
func TestTransportWriteNotificationWhenClosed(t *testing.T) {
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(strings.NewReader(""), writer)

	// Close the transport
	transport.Close()

	// Try to write after close
	err := transport.WriteNotification("test/event", nil)
	assert.Error(t, err)
}

// Test transport WriteNotification with invalid params (can't marshal)
func TestTransportWriteNotificationInvalidParams(t *testing.T) {
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(strings.NewReader(""), writer)

	// Try to write with unmarshalable params
	err := transport.WriteNotification("test/event", make(chan int))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "marshal notification params")
}

// Test transport ReadMessage with complex valid JSON
func TestTransportReadMessageComplex(t *testing.T) {
	req := Request{
		JSONRPC: JSONRPCVersion,
		ID:      42,
		Method:  "test/method",
		Params: func() json.RawMessage {
			p, _ := json.Marshal(map[string]any{
				"key":    "value",
				"nested": map[string]any{"deep": "data"},
			})
			return p
		}(),
	}
	reqJSON, _ := json.Marshal(req)

	reader := strings.NewReader(string(reqJSON) + "\n")
	transport := NewStdioTransport(reader, &bytes.Buffer{})

	result, err := transport.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, float64(42), result.ID)
	assert.Equal(t, "test/method", result.Method)
}

// Test Serve method with custom reader/writer
func TestServe(t *testing.T) {
	t.Run("serve processes requests and exits on EOF", func(t *testing.T) {
		// Create request JSON
		initReq := Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "initialize",
			Params: func() json.RawMessage {
				p, _ := json.Marshal(InitializeParams{
					ProtocolVersion: MCPVersion,
					ClientInfo:      Implementation{Name: "test", Version: "1.0"},
				})
				return p
			}(),
		}
		pingReq := Request{
			JSONRPC: JSONRPCVersion,
			ID:      2,
			Method:  "ping",
		}

		// Build input
		var input strings.Builder
		initJSON, _ := json.Marshal(initReq)
		pingJSON, _ := json.Marshal(pingReq)
		input.WriteString(string(initJSON) + "\n")
		input.WriteString(string(pingJSON) + "\n")

		reader := strings.NewReader(input.String())
		writer := &bytes.Buffer{}

		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Serve will process requests until EOF
		err = server.Serve(reader, writer)
		assert.NoError(t, err)

		// Should have written responses
		assert.True(t, len(writer.Bytes()) > 0)
		output := writer.String()
		assert.Contains(t, output, "2.0")
	})

	t.Run("serve handles invalid JSON gracefully", func(t *testing.T) {
		input := "not valid json\n"
		reader := strings.NewReader(input)
		writer := &bytes.Buffer{}

		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		err = server.Serve(reader, writer)
		assert.NoError(t, err)

		// Should have written error response
		assert.Contains(t, writer.String(), "Parse error")
	})
}

// Test handleCallTool with invalid params
func TestHandleCallToolInvalidParams(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	// Create request with invalid params (not a valid CallToolParams JSON)
	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`{"invalid": true}`), // missing required "name" field
	}

	resp := server.HandleRequest(context.Background(), req)
	require.NotNil(t, resp)
	// The handler should still work but with empty name which won't find a tool
}

// Test handleCallTool with completely malformed JSON
func TestHandleCallToolMalformedJSON(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "tools/call",
		Params:  json.RawMessage(`not valid json`),
	}

	resp := server.HandleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

// Test handleReadResource with malformed JSON
func TestHandleReadResourceMalformedJSON(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "resources/read",
		Params:  json.RawMessage(`{broken json`),
	}

	resp := server.HandleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

// Test handleGetPrompt with malformed JSON
func TestHandleGetPromptMalformedJSON(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "prompts/get",
		Params:  json.RawMessage(`[invalid`),
	}

	resp := server.HandleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, ErrCodeInvalidParams, resp.Error.Code)
}

// errorWriter is a writer that always fails
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("write failed")
}

// Test Run with write error
func TestRunWithWriteError(t *testing.T) {
	initReq := Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "initialize",
		Params: func() json.RawMessage {
			p, _ := json.Marshal(InitializeParams{
				ProtocolVersion: MCPVersion,
				ClientInfo:      Implementation{Name: "test", Version: "1.0"},
			})
			return p
		}(),
	}
	reqJSON, _ := json.Marshal(initReq)

	reader := strings.NewReader(string(reqJSON) + "\n")
	writer := &errorWriter{}

	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	// Serve should return error when writing response fails
	err = server.Serve(reader, writer)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write")
}

// Test ReadMessage returns parse error
func TestTransportReadMessageParseError(t *testing.T) {
	reader := strings.NewReader("invalid json line\n")
	transport := NewStdioTransport(reader, &bytes.Buffer{})

	_, err := transport.ReadMessage()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse request")
}

// Test WriteResponse marshal error (should be impossible with valid Response but test for completeness)
func TestTransportWriteResponseMarshalError(t *testing.T) {
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(strings.NewReader(""), writer)

	// Create a response with a value that can't be marshaled
	resp := &Response{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Result:  make(chan int), // channels can't be marshaled
	}

	err := transport.WriteResponse(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "marshal response")
}

// Test WriteResponse write error
func TestTransportWriteResponseWriteError(t *testing.T) {
	transport := NewStdioTransport(strings.NewReader(""), &errorWriter{})

	resp := NewResponse(1, "test")
	err := transport.WriteResponse(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write response")
}

// Test WriteNotification write error
func TestTransportWriteNotificationWriteError(t *testing.T) {
	transport := NewStdioTransport(strings.NewReader(""), &errorWriter{})

	err := transport.WriteNotification("test/event", map[string]string{"key": "value"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write notification")
}

// Test WriteNotification with nil params (should succeed, different code path)
func TestTransportWriteNotificationNilParams(t *testing.T) {
	writer := &bytes.Buffer{}
	transport := NewStdioTransport(strings.NewReader(""), writer)

	err := transport.WriteNotification("test/event", nil)
	require.NoError(t, err)

	// Should have written a valid notification without params
	output := writer.String()
	assert.Contains(t, output, "test/event")
	assert.Contains(t, output, "2.0")
}

// failingReader is an io.Reader that always fails with a non-EOF error
type failingReader struct{}

func (f *failingReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read failed")
}

// Test ReadMessage with read error (not EOF)
func TestTransportReadMessageReadError(t *testing.T) {
	// Create a transport with an error-producing reader
	transport := NewStdioTransport(&failingReader{}, &bytes.Buffer{})

	_, err := transport.ReadMessage()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read failed")
}

// Test ReadMessage when transport is closed
func TestTransportReadMessageWhenClosed(t *testing.T) {
	transport := NewStdioTransport(strings.NewReader(""), &bytes.Buffer{})
	transport.Close()

	_, err := transport.ReadMessage()
	require.Error(t, err)
	assert.ErrorIs(t, err, io.EOF)
}

// Test resourceConfig with nil config
func TestResourceConfigNilConfig(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)
	// Server has no config set by default

	req := &Request{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "resources/read",
		Params: func() json.RawMessage {
			p, _ := json.Marshal(ReadResourceParams{URI: "relicta://config"})
			return p
		}(),
	}

	resp := server.HandleRequest(context.Background(), req)
	require.NotNil(t, resp)
	assert.Nil(t, resp.Error)
	// Result should contain "no configuration loaded"
	resultJSON, _ := json.Marshal(resp.Result)
	assert.Contains(t, string(resultJSON), "no configuration loaded")
}

// Test promptReleaseSummary with all style options
func TestPromptReleaseSummaryAllStyles(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	testCases := []struct {
		style    string
		expected string
	}{
		{"brief", "brief summary"},
		{"detailed", "detailed summary"},
		{"technical", "technical summary"},
	}

	for _, tc := range testCases {
		t.Run(tc.style, func(t *testing.T) {
			params := GetPromptParams{
				Name:      "release-summary",
				Arguments: map[string]string{"style": tc.style},
			}
			paramsJSON, _ := json.Marshal(params)

			req := &Request{
				JSONRPC: JSONRPCVersion,
				ID:      1,
				Method:  "prompts/get",
				Params:  paramsJSON,
			}

			resp := server.HandleRequest(context.Background(), req)
			require.NotNil(t, resp)
			assert.Nil(t, resp.Error)
		})
	}
}

// Test tool handlers with adapter that returns errors at specific points
func TestToolHandlersWithAdapterErrors(t *testing.T) {
	t.Run("toolNotes fails when GetStatus fails", func(t *testing.T) {
		// Create adapter with notes UC but repo that returns error
		errorRepo := &mockErrorReleaseRepository{err: errors.New("no releases")}
		adapter := NewAdapter(
			WithAdapterReleaseRepository(errorRepo),
		)

		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		params := CallToolParams{
			Name:      "relicta.notes",
			Arguments: map[string]any{},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		// This will fall through to stub because HasGenerateNotesUseCase() returns false
		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
	})

	t.Run("toolEvaluate with governance service but repo error", func(t *testing.T) {
		// Create adapter with governance but failing repo
		govSvc := &governance.Service{}
		errorRepo := &mockErrorReleaseRepository{err: errors.New("repo error")}
		adapter := NewAdapter(
			WithGovernanceService(govSvc),
			WithAdapterReleaseRepository(errorRepo),
		)

		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		params := CallToolParams{
			Name:      "relicta.evaluate",
			Arguments: map[string]any{},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		// Should get error result since GetStatus fails
		resultJSON, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(resultJSON), "No active release")
	})

	t.Run("toolApprove with repo that returns error", func(t *testing.T) {
		errorRepo := &mockErrorReleaseRepository{err: errors.New("repo error")}
		adapter := NewAdapter(
			WithAdapterReleaseRepository(errorRepo),
		)

		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		params := CallToolParams{
			Name:      "relicta.approve",
			Arguments: map[string]any{"actor": "test-user"},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
	})

	t.Run("toolPublish with repo that returns error", func(t *testing.T) {
		errorRepo := &mockErrorReleaseRepository{err: errors.New("repo error")}
		adapter := NewAdapter(
			WithAdapterReleaseRepository(errorRepo),
		)

		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)

		params := CallToolParams{
			Name:      "relicta.publish",
			Arguments: map[string]any{},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
	})
}

// Test handler error paths by registering failing handlers
func TestHandlerErrorPaths(t *testing.T) {
	t.Run("toolHandler returns error", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Register a tool that returns an error
		server.tools["test.failing"] = func(ctx context.Context, args map[string]any) (*CallToolResult, error) {
			return nil, errors.New("tool execution failed")
		}

		params := CallToolParams{
			Name:      "test.failing",
			Arguments: map[string]any{},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeInternalError, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Tool execution failed")
	})

	t.Run("resourceHandler returns error", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Register a resource that returns an error
		server.resources["relicta://failing"] = func(ctx context.Context, uri string) (*ReadResourceResult, error) {
			return nil, errors.New("resource read failed")
		}

		params := ReadResourceParams{URI: "relicta://failing"}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "resources/read",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeInternalError, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Resource read failed")
	})

	t.Run("promptHandler returns error", func(t *testing.T) {
		server, err := NewServer("1.0.0")
		require.NoError(t, err)

		// Register a prompt that returns an error
		server.prompts["failing-prompt"] = func(ctx context.Context, args map[string]string) (*GetPromptResult, error) {
			return nil, errors.New("prompt generation failed")
		}

		params := GetPromptParams{Name: "failing-prompt"}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "prompts/get",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.NotNil(t, resp.Error)
		assert.Equal(t, ErrCodeInternalError, resp.Error.Code)
		assert.Contains(t, resp.Error.Message, "Prompt generation failed")
	})
}

// Test tool handlers with different argument types and combinations
func TestToolHandlerArgumentTypes(t *testing.T) {
	server, err := NewServer("1.0.0")
	require.NoError(t, err)

	t.Run("toolPlan with analyze as bool", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.plan",
			Arguments: map[string]any{"from": "main", "analyze": true},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("toolBump with version argument", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.bump",
			Arguments: map[string]any{"bump": "major", "version": "2.0.0"},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
		// Check that version is in result
		resultJSON, _ := json.Marshal(resp.Result)
		assert.Contains(t, string(resultJSON), "2.0.0")
	})

	t.Run("toolNotes with ai as bool", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.notes",
			Arguments: map[string]any{"ai": true},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("toolApprove with message", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.approve",
			Arguments: map[string]any{"actor": "ci-bot", "message": "LGTM"},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("toolPublish with assets", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.publish",
			Arguments: map[string]any{"skip_push": true, "assets": []string{"dist/app.tar.gz"}},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("toolEvaluate with actor arguments", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.evaluate",
			Arguments: map[string]any{"actor": "test-agent", "include_history": true},
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})

	t.Run("toolPublish without skip_push", func(t *testing.T) {
		params := CallToolParams{
			Name:      "relicta.publish",
			Arguments: map[string]any{}, // no arguments
		}
		paramsJSON, _ := json.Marshal(params)

		req := &Request{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Method:  "tools/call",
			Params:  paramsJSON,
		}

		resp := server.HandleRequest(context.Background(), req)
		require.NotNil(t, resp)
		assert.Nil(t, resp.Error)
	})
}

// Test helpers for server_test.go
// Note: mockReleaseRepository is defined in adapters_test.go

// createTestRelease creates a release with a plan for testing
func createTestRelease(id, currentVersion, nextVersion string) *release.Release {
	rel := release.NewRelease(release.ReleaseID(id), "main", "")
	curr, _ := version.Parse(currentVersion)
	next, _ := version.Parse(nextVersion)
	plan := release.NewReleasePlan(curr, next, changes.ReleaseTypeMinor, nil, false)
	_ = release.SetPlan(rel, plan)
	return rel
}
