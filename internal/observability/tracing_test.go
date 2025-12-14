// Package observability provides tests for tracing functionality.
package observability

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
)

func TestDefaultTracerConfig(t *testing.T) {
	cfg := DefaultTracerConfig()

	if cfg.Enabled {
		t.Error("Expected Enabled to be false by default")
	}
	if cfg.ServiceName != "relicta" {
		t.Errorf("ServiceName = %q, want \"relicta\"", cfg.ServiceName)
	}
	if cfg.Environment != "development" {
		t.Errorf("Environment = %q, want \"development\"", cfg.Environment)
	}
	if cfg.Endpoint != "localhost:4317" {
		t.Errorf("Endpoint = %q, want \"localhost:4317\"", cfg.Endpoint)
	}
	if !cfg.Insecure {
		t.Error("Expected Insecure to be true by default")
	}
	if cfg.SampleRate != 1.0 {
		t.Errorf("SampleRate = %f, want 1.0", cfg.SampleRate)
	}
}

func TestNoopSpan(t *testing.T) {
	span := &noopSpan{}

	// All methods should be no-ops and not panic
	span.End()
	span.SetStatus(SpanStatusOK, "ok")
	span.SetAttribute("key", "value")
	span.SetAttributes(map[string]any{"key": "value"})
	span.RecordError(errors.New("test error"))
	span.AddEvent("event", map[string]any{"key": "value"})

	ctx := span.SpanContext()
	if ctx.TraceID != "" {
		t.Error("Expected empty TraceID for noop span")
	}
	if ctx.SpanID != "" {
		t.Error("Expected empty SpanID for noop span")
	}
}

func TestNoopTracer(t *testing.T) {
	tracer := &noopTracer{}
	ctx := context.Background()

	newCtx, span := tracer.Start(ctx, "test-span")

	if newCtx != ctx {
		t.Error("Expected noop tracer to return same context")
	}
	if _, ok := span.(*noopSpan); !ok {
		t.Error("Expected noop tracer to return noopSpan")
	}

	err := tracer.Shutdown(ctx)
	if err != nil {
		t.Errorf("Shutdown error = %v", err)
	}
}

func TestLoggingTracer(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tracer := NewLoggingTracer(logger, "test-service")
	ctx := context.Background()

	newCtx, span := tracer.Start(ctx, "test-operation")

	if newCtx == ctx {
		t.Error("Expected new context with span")
	}

	// Verify span is a loggingSpan
	ls, ok := span.(*loggingSpan)
	if !ok {
		t.Fatal("Expected loggingSpan")
	}

	if ls.name != "test-operation" {
		t.Errorf("span name = %q, want \"test-operation\"", ls.name)
	}

	span.End()

	// Verify logging occurred
	logOutput := buf.String()
	if logOutput == "" {
		t.Error("Expected log output")
	}
}

func TestLoggingSpan_SetAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	span := newLoggingSpan("test", logger, "trace123", "span456")
	span.SetAttribute("key1", "value1")
	span.SetAttributes(map[string]any{"key2": 42, "key3": true})

	if span.attributes["key1"] != "value1" {
		t.Errorf("key1 = %v, want \"value1\"", span.attributes["key1"])
	}
	if span.attributes["key2"] != 42 {
		t.Errorf("key2 = %v, want 42", span.attributes["key2"])
	}
	if span.attributes["key3"] != true {
		t.Errorf("key3 = %v, want true", span.attributes["key3"])
	}
}

func TestLoggingSpan_SetStatus(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	span := newLoggingSpan("test", logger, "trace123", "span456")
	span.SetStatus(SpanStatusError, "something failed")

	if span.status != SpanStatusError {
		t.Errorf("status = %v, want SpanStatusError", span.status)
	}
	if span.statusDesc != "something failed" {
		t.Errorf("statusDesc = %q, want \"something failed\"", span.statusDesc)
	}
}

func TestLoggingSpan_RecordError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	span := newLoggingSpan("test", logger, "trace123", "span456")
	testErr := errors.New("test error")
	span.RecordError(testErr)

	if span.err != testErr {
		t.Error("Expected error to be recorded")
	}
	if span.status != SpanStatusError {
		t.Error("Expected status to be set to error")
	}

	span.End()

	logOutput := buf.String()
	if logOutput == "" {
		t.Error("Expected error log output")
	}
}

func TestLoggingSpan_AddEvent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	span := newLoggingSpan("test", logger, "trace123", "span456")
	span.AddEvent("event1", map[string]any{"key": "value"})
	span.AddEvent("event2", nil)

	if len(span.events) != 2 {
		t.Errorf("events count = %d, want 2", len(span.events))
	}
	if span.events[0].name != "event1" {
		t.Errorf("first event name = %q, want \"event1\"", span.events[0].name)
	}
}

func TestLoggingSpan_SpanContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	span := newLoggingSpan("test", logger, "trace123", "span456")
	ctx := span.SpanContext()

	if ctx.TraceID != "trace123" {
		t.Errorf("TraceID = %q, want \"trace123\"", ctx.TraceID)
	}
	if ctx.SpanID != "span456" {
		t.Errorf("SpanID = %q, want \"span456\"", ctx.SpanID)
	}
}

func TestLoggingTracer_ChildSpan(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tracer := NewLoggingTracer(logger, "test-service")
	ctx := context.Background()

	// Start parent span
	ctx, parentSpan := tracer.Start(ctx, "parent")
	parentCtx := parentSpan.SpanContext()

	// Start child span - should inherit trace ID
	_, childSpan := tracer.Start(ctx, "child")
	childCtx := childSpan.SpanContext()

	if childCtx.TraceID != parentCtx.TraceID {
		t.Errorf("Child TraceID = %q, want %q (parent)", childCtx.TraceID, parentCtx.TraceID)
	}
	if childCtx.SpanID == parentCtx.SpanID {
		t.Error("Child SpanID should be different from parent")
	}

	childSpan.End()
	parentSpan.End()
}

func TestSpanFromContext(t *testing.T) {
	t.Run("with span in context", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

		tracer := NewLoggingTracer(logger, "test")
		ctx, span := tracer.Start(context.Background(), "test")

		retrieved := SpanFromContext(ctx)
		if retrieved != span {
			t.Error("Expected same span from context")
		}
	})

	t.Run("without span in context", func(t *testing.T) {
		ctx := context.Background()
		span := SpanFromContext(ctx)

		if _, ok := span.(*noopSpan); !ok {
			t.Error("Expected noopSpan for empty context")
		}
	})
}

func TestSpanOptions(t *testing.T) {
	t.Run("WithSpanKind", func(t *testing.T) {
		cfg := &spanConfig{}
		opt := WithSpanKind(SpanKindClient)
		opt(cfg)

		if cfg.kind != SpanKindClient {
			t.Errorf("kind = %v, want SpanKindClient", cfg.kind)
		}
	})

	t.Run("WithAttributes", func(t *testing.T) {
		cfg := &spanConfig{}
		opt := WithAttributes(map[string]any{"key": "value"})
		opt(cfg)

		if cfg.attributes["key"] != "value" {
			t.Errorf("attributes[key] = %v, want \"value\"", cfg.attributes["key"])
		}
	})
}

func TestInitTracer_Disabled(t *testing.T) {
	cfg := TracerConfig{Enabled: false}

	tracer, err := InitTracer(cfg)
	if err != nil {
		t.Errorf("InitTracer error = %v", err)
	}

	if _, ok := tracer.(*noopTracer); !ok {
		t.Error("Expected noopTracer when disabled")
	}
}

func TestInitTracer_Enabled(t *testing.T) {
	cfg := TracerConfig{
		Enabled:     true,
		ServiceName: "test-service",
	}

	tracer, err := InitTracer(cfg)
	if err != nil {
		t.Errorf("InitTracer error = %v", err)
	}

	// Currently returns logging tracer when OTLP is not available
	if _, ok := tracer.(*loggingTracer); !ok {
		t.Error("Expected loggingTracer when enabled without OTLP")
	}
}

func TestGetSetTracer(t *testing.T) {
	original := GetTracer()

	newTracer := &noopTracer{}
	SetTracer(newTracer)

	if GetTracer() != newTracer {
		t.Error("Expected GetTracer to return the set tracer")
	}

	// Restore original
	SetTracer(original)
}

func TestStartSpan(t *testing.T) {
	ctx := context.Background()
	newCtx, span := StartSpan(ctx, "test-span")

	if newCtx == nil {
		t.Error("Expected non-nil context")
	}
	if span == nil {
		t.Error("Expected non-nil span")
	}

	span.End()
}

func TestTraceFunc_Success(t *testing.T) {
	ctx := context.Background()

	called := false
	err := TraceFunc(ctx, "test-func", func(ctx context.Context) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("TraceFunc error = %v", err)
	}
	if !called {
		t.Error("Expected function to be called")
	}
}

func TestTraceFunc_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test error")

	err := TraceFunc(ctx, "test-func", func(ctx context.Context) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("TraceFunc error = %v, want %v", err, expectedErr)
	}
}

func TestLoggingSpan_ConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	span := newLoggingSpan("test", logger, "trace", "span")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			span.SetAttribute("key", idx)
			span.AddEvent("event", nil)
		}(i)
	}

	wg.Wait()
	span.End()
}

func TestLoggingTracer_ConcurrentSpans(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	tracer := NewLoggingTracer(logger, "test")
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, span := tracer.Start(ctx, "concurrent-span")
			span.SetAttribute("index", idx)
			span.End()
		}(i)
	}

	wg.Wait()
}

func TestShutdownTracer(t *testing.T) {
	ctx := context.Background()

	// Should not error even with noop tracer
	err := ShutdownTracer(ctx)
	if err != nil {
		t.Errorf("ShutdownTracer error = %v", err)
	}
}

func TestLoggingTracer_NilLogger(t *testing.T) {
	// Should use default logger when nil
	tracer := NewLoggingTracer(nil, "test")

	if tracer == nil {
		t.Fatal("Expected non-nil tracer")
	}

	ctx, span := tracer.Start(context.Background(), "test")
	if ctx == nil || span == nil {
		t.Error("Expected valid context and span")
	}
	span.End()
}

func TestAttributeConstants(t *testing.T) {
	// Just verify the constants are defined and have expected values
	attrs := []struct {
		name     string
		expected string
	}{
		{AttrReleaseVersion, "release.version"},
		{AttrReleaseType, "release.type"},
		{AttrRepositoryOwner, "repository.owner"},
		{AttrRepositoryName, "repository.name"},
		{AttrPluginName, "plugin.name"},
		{AttrPluginHook, "plugin.hook"},
		{AttrCommandName, "command.name"},
		{AttrGitBranch, "git.branch"},
		{AttrGitCommit, "git.commit"},
		{AttrAIProvider, "ai.provider"},
		{AttrAIModel, "ai.model"},
		{AttrErrorType, "error.type"},
		{AttrErrorMessage, "error.message"},
	}

	for _, attr := range attrs {
		if attr.name != attr.expected {
			t.Errorf("%s = %q, want %q", attr.name, attr.name, attr.expected)
		}
	}
}
