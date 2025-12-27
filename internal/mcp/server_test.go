package mcp

import (
	"log/slog"
	"os"
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
		assert.NotNil(t, server.server)
		assert.NotNil(t, server.riskCalc)
		assert.NotNil(t, server.cache)
		assert.Equal(t, "1.0.0", server.version)
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

	t.Run("cache can be disabled", func(t *testing.T) {
		server, err := NewServer("1.0.0", WithCacheDisabled())
		require.NoError(t, err)
		assert.Nil(t, server.cache)
	})

	t.Run("adapter can be set", func(t *testing.T) {
		adapter := NewAdapter()
		server, err := NewServer("1.0.0", WithAdapter(adapter))
		require.NoError(t, err)
		assert.Equal(t, adapter, server.adapter)
	})
}

func TestToolInputTypes(t *testing.T) {
	t.Run("PlanToolInput fields", func(t *testing.T) {
		input := PlanToolInput{
			From:    "v1.0.0",
			Analyze: true,
		}
		assert.Equal(t, "v1.0.0", input.From)
		assert.True(t, input.Analyze)
	})

	t.Run("BumpToolInput fields", func(t *testing.T) {
		input := BumpToolInput{
			Bump:    "minor",
			Version: "1.2.0",
		}
		assert.Equal(t, "minor", input.Bump)
		assert.Equal(t, "1.2.0", input.Version)
	})

	t.Run("NotesToolInput fields", func(t *testing.T) {
		input := NotesToolInput{AI: true}
		assert.True(t, input.AI)
	})

	t.Run("ApproveToolInput fields", func(t *testing.T) {
		input := ApproveToolInput{Notes: "Updated notes"}
		assert.Equal(t, "Updated notes", input.Notes)
	})

	t.Run("PublishToolInput fields", func(t *testing.T) {
		input := PublishToolInput{DryRun: true}
		assert.True(t, input.DryRun)
	})
}
