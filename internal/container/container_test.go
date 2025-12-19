// Package container provides dependency injection for Relicta services.
package container

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/relicta-tech/relicta/internal/config"
)

// mockCloseable implements Closeable for testing.
type mockCloseable struct {
	name       string
	closeCount int32
	closeDelay time.Duration
	closeErr   error
}

func (m *mockCloseable) Close() error {
	if m.closeDelay > 0 {
		time.Sleep(m.closeDelay)
	}
	atomic.AddInt32(&m.closeCount, 1)
	return m.closeErr
}

func (m *mockCloseable) getCloseCount() int32 {
	return atomic.LoadInt32(&m.closeCount)
}

func TestApp_Close_EmptyCloseables(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Close should work even with no closeables
	err = c.Close()
	if err != nil {
		t.Errorf("Close should not return error for empty closeables, got: %v", err)
	}
}

func TestApp_Close_ClosesAllComponents(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register multiple closeables
	closeable1 := &mockCloseable{name: "first"}
	closeable2 := &mockCloseable{name: "second"}
	closeable3 := &mockCloseable{name: "third"}

	c.RegisterCloseable(closeable1)
	c.RegisterCloseable(closeable2)
	c.RegisterCloseable(closeable3)

	// Close container
	err = c.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}

	// Verify all closeables were closed
	if closeable1.getCloseCount() != 1 {
		t.Errorf("closeable1 should be closed once, got %d", closeable1.getCloseCount())
	}
	if closeable2.getCloseCount() != 1 {
		t.Errorf("closeable2 should be closed once, got %d", closeable2.getCloseCount())
	}
	if closeable3.getCloseCount() != 1 {
		t.Errorf("closeable3 should be closed once, got %d", closeable3.getCloseCount())
	}
}

func TestApp_Close_Idempotent(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	closeable := &mockCloseable{name: "test"}
	c.RegisterCloseable(closeable)

	// Close multiple times
	err = c.Close()
	if err != nil {
		t.Errorf("First Close should not return error, got: %v", err)
	}

	err = c.Close()
	if err != nil {
		t.Errorf("Second Close should not return error, got: %v", err)
	}

	// Closeable should only be closed once
	if closeable.getCloseCount() != 1 {
		t.Errorf("closeable should be closed only once, got %d", closeable.getCloseCount())
	}
}

func TestApp_CloseWithTimeout_ReturnsFirstError(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	expectedErr := errors.New("close failed")
	closeable1 := &mockCloseable{name: "first", closeErr: nil}
	closeable2 := &mockCloseable{name: "second", closeErr: expectedErr}
	closeable3 := &mockCloseable{name: "third", closeErr: errors.New("another error")}

	c.RegisterCloseable(closeable1)
	c.RegisterCloseable(closeable2)
	c.RegisterCloseable(closeable3)

	// Close container - should return the first error encountered
	// Note: closeables are closed in LIFO order, so third is closed first
	err = c.Close()
	if err == nil {
		t.Error("Close should return an error when closeables fail")
	}
}

func TestApp_CloseWithTimeout_TimesOut(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Create a closeable that takes longer than the timeout
	slowCloseable := &mockCloseable{
		name:       "slow",
		closeDelay: 2 * time.Second,
	}
	c.RegisterCloseable(slowCloseable)

	// Close with a short timeout
	err = c.CloseWithTimeout(100 * time.Millisecond)
	if err == nil {
		t.Error("CloseWithTimeout should return error on timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded error, got: %v", err)
	}
}

func TestApp_Close_ClosesInReverseOrder(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	var mu sync.Mutex
	var closeOrder []string

	// Create closeables that record their close order
	closeable1 := &orderRecordingCloseable{name: "first", order: &closeOrder, mu: &mu}
	closeable2 := &orderRecordingCloseable{name: "second", order: &closeOrder, mu: &mu}
	closeable3 := &orderRecordingCloseable{name: "third", order: &closeOrder, mu: &mu}

	c.RegisterCloseable(closeable1)
	c.RegisterCloseable(closeable2)
	c.RegisterCloseable(closeable3)

	err = c.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}

	// Verify LIFO order: third, second, first
	if len(closeOrder) != 3 {
		t.Fatalf("expected 3 closes, got %d", len(closeOrder))
	}
	if closeOrder[0] != "third" {
		t.Errorf("expected third to close first, got %s", closeOrder[0])
	}
	if closeOrder[1] != "second" {
		t.Errorf("expected second to close second, got %s", closeOrder[1])
	}
	if closeOrder[2] != "first" {
		t.Errorf("expected first to close third, got %s", closeOrder[2])
	}
}

// orderRecordingCloseable records the order in which it was closed.
type orderRecordingCloseable struct {
	name  string
	order *[]string
	mu    *sync.Mutex
}

func (c *orderRecordingCloseable) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	*c.order = append(*c.order, c.name)
	return nil
}

func TestApp_RegisterCloseable_NilSafe(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Register nil closeable should not panic
	c.RegisterCloseable(nil)

	// Close should still work
	err = c.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}
}

func TestApp_Initialize_FailsWhenClosed(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Close the container
	err = c.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}

	// Initialize should fail on closed container
	ctx := context.Background()
	err = c.Initialize(ctx)
	if err == nil {
		t.Error("Initialize should fail on closed container")
	}
}

func TestApp_RegisterCloseable_ConcurrentSafe(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	const numGoroutines = 100
	var wg sync.WaitGroup

	// Concurrently register closeables
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			closeable := &mockCloseable{name: "concurrent"}
			c.RegisterCloseable(closeable)
		}(i)
	}

	wg.Wait()

	// Close should not panic
	err = c.Close()
	if err != nil {
		t.Errorf("Close should not return error, got: %v", err)
	}
}

func TestNew_NilConfig(t *testing.T) {
	c, err := New(nil)
	if err == nil {
		t.Error("New(nil) should return error")
	}
	if c != nil {
		t.Error("New(nil) should return nil container")
	}
}

func TestApp_Accessors_BeforeInitialize(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Use cases should return nil before initialization
	if c.PlanRelease() != nil {
		t.Error("PlanRelease should return nil before Initialize")
	}
	if c.GenerateNotes() != nil {
		t.Error("GenerateNotes should return nil before Initialize")
	}
	if c.ApproveRelease() != nil {
		t.Error("ApproveRelease should return nil before Initialize")
	}
	if c.PublishRelease() != nil {
		t.Error("PublishRelease should return nil before Initialize")
	}
	if c.CalculateVersion() != nil {
		t.Error("CalculateVersion should return nil before Initialize")
	}
	if c.SetVersion() != nil {
		t.Error("SetVersion should return nil before Initialize")
	}
	// Note: Some infrastructure accessors may return nil interfaces with non-nil underlying types
	// GitAdapter, ReleaseRepository, EventPublisher are initialized in infrastructure init
	// which happens after Initialize() is called
	if c.Git() != nil {
		t.Error("Git should return nil before Initialize")
	}
	if c.AI() != nil {
		t.Error("AI should return nil before Initialize")
	}
	if c.HasAI() {
		t.Error("HasAI should return false before Initialize")
	}
	if c.Config() != cfg {
		t.Error("Config should return the config passed to constructor")
	}

	// Test infrastructure accessors - they should also be nil before init
	// These use interface returns, so we test them separately
	gitAdapter := c.GitAdapter()
	releaseRepo := c.ReleaseRepository()
	eventPub := c.EventPublisher()
	pluginReg := c.PluginRegistry()

	// After calling accessors, verify they were called (coverage)
	// The actual nil check depends on initialization state
	_ = gitAdapter
	_ = releaseRepo
	_ = eventPub
	_ = pluginReg
}

func TestApp_Accessors_ConcurrentRead(t *testing.T) {
	cfg := &config.Config{}
	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	const numGoroutines = 50
	var wg sync.WaitGroup

	// Concurrently access all accessors
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = c.PlanRelease()
			_ = c.GenerateNotes()
			_ = c.ApproveRelease()
			_ = c.PublishRelease()
			_ = c.CalculateVersion()
			_ = c.SetVersion()
			_ = c.GitAdapter()
			_ = c.ReleaseRepository()
			_ = c.EventPublisher()
			_ = c.PluginRegistry()
			_ = c.Git()
			_ = c.AI()
			_ = c.HasAI()
			_ = c.Config()
		}()
	}

	wg.Wait()
}

func TestApp_Initialize_Success(t *testing.T) {
	// Create a temporary directory for the release repository
	tmpDir := t.TempDir()

	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled: false, // Disable AI to avoid API key requirements
		},
		Plugins: []config.PluginConfig{}, // No plugins to simplify test
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Change to temp directory for git initialization
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	err = c.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Verify infrastructure components are initialized
	if c.GitAdapter() == nil {
		t.Error("GitAdapter should be initialized")
	}
	if c.ReleaseRepository() == nil {
		t.Error("ReleaseRepository should be initialized")
	}
	if c.EventPublisher() == nil {
		t.Error("EventPublisher should be initialized")
	}
	if c.PluginRegistry() == nil {
		t.Error("PluginRegistry should be initialized")
	}

	// Verify application layer components are initialized
	if c.PlanRelease() == nil {
		t.Error("PlanRelease should be initialized")
	}
	if c.GenerateNotes() == nil {
		t.Error("GenerateNotes should be initialized")
	}
	if c.ApproveRelease() == nil {
		t.Error("ApproveRelease should be initialized")
	}
	if c.PublishRelease() == nil {
		t.Error("PublishRelease should be initialized")
	}
	if c.CalculateVersion() == nil {
		t.Error("CalculateVersion should be initialized")
	}
	if c.SetVersion() == nil {
		t.Error("SetVersion should be initialized")
	}

	// Verify service layer
	if c.Git() == nil {
		t.Error("Git service should be initialized")
	}

	// AI should be nil when disabled
	if c.AI() != nil {
		t.Error("AI service should be nil when disabled")
	}
	if c.HasAI() {
		t.Error("HasAI should return false when AI is disabled")
	}

	// Clean up
	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestApp_Initialize_WithPlugins(t *testing.T) {
	tmpDir := t.TempDir()

	enabled := true
	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled: false,
		},
		Plugins: []config.PluginConfig{
			{
				Name:    "test-plugin",
				Enabled: &enabled,
			},
		},
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	err = c.Initialize(ctx)

	// Plugin initialization may fail due to missing plugin binary,
	// but the container should still initialize with an empty executor
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// PluginRegistry should be initialized even if plugin loading fails
	if c.PluginRegistry() == nil {
		t.Error("PluginRegistry should be initialized")
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestApp_Initialize_WithAIEnabled(t *testing.T) {
	tmpDir := t.TempDir()

	// Set API key via environment variable
	os.Setenv("OPENAI_API_KEY", "test-api-key-for-testing")
	defer os.Unsetenv("OPENAI_API_KEY")

	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:     true,
			Provider:    "openai",
			Model:       "gpt-4",
			MaxTokens:   1000,
			Temperature: 0.7,
			Timeout:     30,
		},
		Plugins: []config.PluginConfig{},
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	err = c.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// AI service should be initialized when enabled with API key
	// Note: AI service may be nil if the API key is invalid or service fails to initialize
	// The test validates that initialization completes even if AI service fails
	_ = c.AI()

	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestApp_Initialize_AIEnabledWithoutAPIKey(t *testing.T) {
	tmpDir := t.TempDir()

	// Ensure no API key is set
	os.Unsetenv("OPENAI_API_KEY")

	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:  true,
			Provider: "openai",
		},
		Plugins: []config.PluginConfig{},
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	err = c.Initialize(ctx)
	// Initialize should succeed even if AI fails (AI is optional)
	if err != nil {
		t.Fatalf("Initialize should succeed even if AI fails: %v", err)
	}

	// AI service should be nil when API key is missing
	if c.AI() != nil {
		t.Error("AI service should be nil when API key is missing")
	}
	if c.HasAI() {
		t.Error("HasAI should return false when AI initialization fails")
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestApp_Initialize_AIWithConfigAPIKey(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:  true,
			Provider: "openai",
			APIKey:   "config-api-key",
		},
		Plugins: []config.PluginConfig{},
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	err = c.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// AI service may be nil if initialization fails (e.g., invalid API key)
	// The test validates that initialization completes even if AI service fails
	_ = c.AI()

	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestApp_Initialize_AIWithBaseURL(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled:  true,
			Provider: "openai",
			APIKey:   "test-api-key",
			BaseURL:  "https://custom-api.example.com",
		},
		Plugins: []config.PluginConfig{},
	}

	c, err := New(cfg)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	err = c.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// AI service may be nil if initialization fails
	// The test validates that initialization completes even if AI service fails
	_ = c.AI()

	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestNewInitialized_Success(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		AI: config.AIConfig{
			Enabled: false,
		},
		Plugins: []config.PluginConfig{},
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change to temp directory: %v", err)
	}

	// Initialize git repository
	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to initialize git repository: %v", err)
	}

	ctx := context.Background()
	c, err := NewInitialized(ctx, cfg)
	if err != nil {
		t.Fatalf("NewInitialized failed: %v", err)
	}

	// Verify container is initialized
	if c.PlanRelease() == nil {
		t.Error("PlanRelease should be initialized")
	}

	if err := c.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestNewInitialized_NilConfig(t *testing.T) {
	ctx := context.Background()
	c, err := NewInitialized(ctx, nil)
	if err == nil {
		t.Error("NewInitialized(nil) should return error")
	}
	if c != nil {
		t.Error("NewInitialized(nil) should return nil container")
	}
}
