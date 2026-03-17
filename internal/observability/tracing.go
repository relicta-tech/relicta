// Package observability provides metrics, tracing, and monitoring for Relicta.
package observability

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// TracerConfig configures the tracing system.
type TracerConfig struct {
	// Enabled indicates whether tracing is enabled.
	Enabled bool
	// ServiceName is the name of the service for tracing.
	ServiceName string
	// ServiceVersion is the version of the service.
	ServiceVersion string
	// Environment is the deployment environment (dev, staging, prod).
	Environment string
	// Endpoint is the OTLP endpoint URL (e.g., "localhost:4317").
	Endpoint string
	// Insecure disables TLS for the OTLP connection.
	Insecure bool
	// SampleRate is the sampling rate (0.0 to 1.0, default 1.0 = sample all).
	SampleRate float64
	// Headers are additional headers to send with OTLP requests.
	Headers map[string]string
}

// DefaultTracerConfig returns a default tracer configuration.
func DefaultTracerConfig() TracerConfig {
	return TracerConfig{
		Enabled:        false,
		ServiceName:    "relicta",
		ServiceVersion: "unknown",
		Environment:    "development",
		Endpoint:       "localhost:4317",
		Insecure:       true,
		SampleRate:     1.0,
	}
}

// SpanKind represents the type of span.
type SpanKind int

const (
	// SpanKindInternal represents an internal operation.
	SpanKindInternal SpanKind = iota
	// SpanKindServer represents a server-side operation.
	SpanKindServer
	// SpanKindClient represents a client-side operation.
	SpanKindClient
	// SpanKindProducer represents a message producer.
	SpanKindProducer
	// SpanKindConsumer represents a message consumer.
	SpanKindConsumer
)

// SpanStatus represents the status of a span.
type SpanStatus int

const (
	// SpanStatusUnset indicates the span status is not set.
	SpanStatusUnset SpanStatus = iota
	// SpanStatusOK indicates the operation completed successfully.
	SpanStatusOK
	// SpanStatusError indicates the operation failed.
	SpanStatusError
)

// Span represents a unit of work or operation.
type Span interface {
	// End completes the span.
	End()
	// SetStatus sets the span status.
	SetStatus(status SpanStatus, description string)
	// SetAttribute sets a span attribute.
	SetAttribute(key string, value any)
	// SetAttributes sets multiple span attributes.
	SetAttributes(attrs map[string]any)
	// RecordError records an error on the span.
	RecordError(err error)
	// AddEvent adds an event to the span.
	AddEvent(name string, attrs map[string]any)
	// SpanContext returns the span context for propagation.
	SpanContext() SpanContext
}

// SpanContext contains identifying trace information about a Span.
type SpanContext struct {
	TraceID string
	SpanID  string
}

// Tracer creates spans for tracing operations.
type Tracer interface {
	// Start creates a new span and returns it along with a new context.
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span)
	// Shutdown gracefully shuts down the tracer.
	Shutdown(ctx context.Context) error
}

// SpanOption configures a span.
type SpanOption func(*spanConfig)

type spanConfig struct {
	kind       SpanKind
	attributes map[string]any
}

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return func(cfg *spanConfig) {
		cfg.kind = kind
	}
}

// WithAttributes sets initial span attributes.
func WithAttributes(attrs map[string]any) SpanOption {
	return func(cfg *spanConfig) {
		cfg.attributes = attrs
	}
}

// noopSpan is a span that does nothing.
type noopSpan struct{}

func (s *noopSpan) End()                                       {}
func (s *noopSpan) SetStatus(status SpanStatus, desc string)   {}
func (s *noopSpan) SetAttribute(key string, value any)         {}
func (s *noopSpan) SetAttributes(attrs map[string]any)         {}
func (s *noopSpan) RecordError(err error)                      {}
func (s *noopSpan) AddEvent(name string, attrs map[string]any) {}
func (s *noopSpan) SpanContext() SpanContext                   { return SpanContext{} }

// noopTracer is a tracer that does nothing.
type noopTracer struct{}

func (t *noopTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return ctx, &noopSpan{}
}

func (t *noopTracer) Shutdown(ctx context.Context) error {
	return nil
}

// loggingSpan is a span that logs operations.
type loggingSpan struct {
	name       string
	startTime  time.Time
	logger     *slog.Logger
	attributes map[string]any
	events     []spanEvent
	status     SpanStatus
	statusDesc string
	err        error
	traceID    string
	spanID     string
	mu         sync.Mutex
}

type spanEvent struct {
	name  string
	time  time.Time
	attrs map[string]any
}

func newLoggingSpan(name string, logger *slog.Logger, traceID, spanID string) *loggingSpan {
	return &loggingSpan{
		name:       name,
		startTime:  time.Now(),
		logger:     logger,
		attributes: make(map[string]any),
		traceID:    traceID,
		spanID:     spanID,
	}
}

func (s *loggingSpan) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	duration := time.Since(s.startTime)

	attrs := []any{
		"span", s.name,
		"duration_ms", duration.Milliseconds(),
		"trace_id", s.traceID,
		"span_id", s.spanID,
	}

	for k, v := range s.attributes {
		attrs = append(attrs, k, v)
	}

	if s.err != nil {
		attrs = append(attrs, "error", s.err.Error())
		s.logger.Error("span completed with error", attrs...)
	} else if s.status == SpanStatusError {
		attrs = append(attrs, "status", "error", "status_description", s.statusDesc)
		s.logger.Warn("span completed with error status", attrs...)
	} else {
		s.logger.Debug("span completed", attrs...)
	}
}

func (s *loggingSpan) SetStatus(status SpanStatus, desc string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
	s.statusDesc = desc
}

func (s *loggingSpan) SetAttribute(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attributes[key] = value
}

func (s *loggingSpan) SetAttributes(attrs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range attrs {
		s.attributes[k] = v
	}
}

func (s *loggingSpan) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
	s.status = SpanStatusError
}

func (s *loggingSpan) AddEvent(name string, attrs map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, spanEvent{
		name:  name,
		time:  time.Now(),
		attrs: attrs,
	})
	s.logger.Debug("span event", "span", s.name, "event", name, "trace_id", s.traceID)
}

func (s *loggingSpan) SpanContext() SpanContext {
	return SpanContext{
		TraceID: s.traceID,
		SpanID:  s.spanID,
	}
}

// loggingTracer is a tracer that logs span operations for debugging.
type loggingTracer struct {
	logger      *slog.Logger
	serviceName string
	spanCounter uint64
	mu          sync.Mutex
}

// NewLoggingTracer creates a new logging tracer for development/debugging.
func NewLoggingTracer(logger *slog.Logger, serviceName string) Tracer {
	if logger == nil {
		logger = slog.Default()
	}
	return &loggingTracer{
		logger:      logger,
		serviceName: serviceName,
	}
}

func (t *loggingTracer) Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	cfg := &spanConfig{
		kind:       SpanKindInternal,
		attributes: make(map[string]any),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	t.mu.Lock()
	t.spanCounter++
	spanID := fmt.Sprintf("%016x", t.spanCounter)
	t.mu.Unlock()

	// Try to get parent trace ID from context, or generate new one
	traceID := getTraceIDFromContext(ctx)
	if traceID == "" {
		traceID = fmt.Sprintf("%032x", time.Now().UnixNano())
	}

	span := newLoggingSpan(name, t.logger, traceID, spanID)
	span.SetAttributes(cfg.attributes)

	t.logger.Debug("span started",
		"span", name,
		"trace_id", traceID,
		"span_id", spanID,
		"service", t.serviceName,
	)

	return contextWithSpan(ctx, span), span
}

func (t *loggingTracer) Shutdown(ctx context.Context) error {
	t.logger.Info("tracer shutdown", "service", t.serviceName)
	return nil
}

// Context keys for span propagation.
type spanContextKey struct{}
type traceIDContextKey struct{}

func contextWithSpan(ctx context.Context, span Span) context.Context {
	ctx = context.WithValue(ctx, spanContextKey{}, span)
	if ls, ok := span.(*loggingSpan); ok {
		ctx = context.WithValue(ctx, traceIDContextKey{}, ls.traceID)
	}
	return ctx
}

// SpanFromContext returns the current span from context, or a noop span.
func SpanFromContext(ctx context.Context) Span {
	if span, ok := ctx.Value(spanContextKey{}).(Span); ok {
		return span
	}
	return &noopSpan{}
}

func getTraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDContextKey{}).(string); ok {
		return traceID
	}
	return ""
}

// Global tracer instance.
var (
	globalTracer   Tracer = &noopTracer{}
	globalTracerMu sync.RWMutex
)

// GetTracer returns the global tracer instance.
func GetTracer() Tracer {
	globalTracerMu.RLock()
	defer globalTracerMu.RUnlock()
	return globalTracer
}

// SetTracer sets the global tracer instance.
func SetTracer(t Tracer) {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()
	globalTracer = t
}

// InitTracer initializes the global tracer with the given configuration.
// If tracing is disabled, a noop tracer is used.
// If no OTLP endpoint is available, a logging tracer is used for development.
func InitTracer(cfg TracerConfig) (Tracer, error) {
	globalTracerMu.Lock()
	defer globalTracerMu.Unlock()

	if !cfg.Enabled {
		globalTracer = &noopTracer{}
		return globalTracer, nil
	}

	// For now, use logging tracer. OTLP integration can be added later
	// when go.opentelemetry.io/otel dependencies are added.
	logger := slog.Default()
	globalTracer = NewLoggingTracer(logger, cfg.ServiceName)

	return globalTracer, nil
}

// ShutdownTracer gracefully shuts down the global tracer.
func ShutdownTracer(ctx context.Context) error {
	globalTracerMu.RLock()
	t := globalTracer
	globalTracerMu.RUnlock()
	return t.Shutdown(ctx)
}

// Convenience functions for common tracing patterns.

// StartSpan starts a new span using the global tracer.
func StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, Span) {
	return GetTracer().Start(ctx, name, opts...)
}

// TraceFunc is a helper to trace a function execution.
func TraceFunc(ctx context.Context, name string, fn func(ctx context.Context) error) error {
	ctx, span := StartSpan(ctx, name)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(SpanStatusError, err.Error())
	} else {
		span.SetStatus(SpanStatusOK, "")
	}
	return err
}

// Common span attribute keys.
const (
	AttrReleaseVersion  = "release.version"
	AttrReleaseType     = "release.type"
	AttrRepositoryOwner = "repository.owner"
	AttrRepositoryName  = "repository.name"
	AttrPluginName      = "plugin.name"
	AttrPluginHook      = "plugin.hook"
	AttrCommandName     = "command.name"
	AttrGitBranch       = "git.branch"
	AttrGitCommit       = "git.commit"
	AttrAIProvider      = "ai.provider"
	AttrAIModel         = "ai.model"
	AttrErrorType       = "error.type"
	AttrErrorMessage    = "error.message"
)
