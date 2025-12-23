package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/relicta-tech/relicta/internal/cgp/risk"
	"github.com/relicta-tech/relicta/internal/config"
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
		assert.Len(t, result.Prompts, 2) // 2 prompts registered
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
