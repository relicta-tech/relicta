// Package observability provides metrics and monitoring for Relicta.
package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics provides application metrics collection.
// It exposes Prometheus-compatible metrics at /metrics endpoint.
type Metrics struct {
	mu sync.RWMutex

	// Counters
	releasesTotal      atomic.Int64
	releasesSuccessful atomic.Int64
	releasesFailed     atomic.Int64
	pluginExecutions   atomic.Int64
	pluginErrors       atomic.Int64
	commandInvocations map[string]*atomic.Int64

	// Gauges
	activeReleases atomic.Int64

	// Histograms (simplified - just track count and sum)
	releaseLatencyCount atomic.Int64
	releaseLatencySum   atomic.Int64
	pluginLatencyCount  atomic.Int64
	pluginLatencySum    atomic.Int64
	commandLatencyCount map[string]*atomic.Int64
	commandLatencySum   map[string]*atomic.Int64

	// Info
	version   string
	startTime time.Time
}

// knownCommands lists CLI commands to pre-initialize metrics for,
// avoiding lock contention on the hot path during command invocation.
var knownCommands = []string{
	"init", "plan", "bump", "notes", "approve", "publish",
	"health", "metrics", "version",
}

// NewMetrics creates a new Metrics instance.
// Pre-initializes metrics for known commands to reduce lock contention.
func NewMetrics(version string) *Metrics {
	// Pre-allocate maps with known capacity
	commandInvocations := make(map[string]*atomic.Int64, len(knownCommands))
	commandLatencyCount := make(map[string]*atomic.Int64, len(knownCommands))
	commandLatencySum := make(map[string]*atomic.Int64, len(knownCommands))

	// Pre-initialize known commands to avoid locking on hot path
	for _, cmd := range knownCommands {
		commandInvocations[cmd] = &atomic.Int64{}
		commandLatencyCount[cmd] = &atomic.Int64{}
		commandLatencySum[cmd] = &atomic.Int64{}
	}

	return &Metrics{
		commandInvocations:  commandInvocations,
		commandLatencyCount: commandLatencyCount,
		commandLatencySum:   commandLatencySum,
		version:             version,
		startTime:           time.Now(),
	}
}

// RecordRelease records a release operation.
func (m *Metrics) RecordRelease(success bool, duration time.Duration) {
	m.releasesTotal.Add(1)
	if success {
		m.releasesSuccessful.Add(1)
	} else {
		m.releasesFailed.Add(1)
	}
	m.releaseLatencyCount.Add(1)
	m.releaseLatencySum.Add(duration.Milliseconds())
}

// RecordPluginExecution records a plugin execution.
func (m *Metrics) RecordPluginExecution(pluginName string, success bool, duration time.Duration) {
	m.pluginExecutions.Add(1)
	if !success {
		m.pluginErrors.Add(1)
	}
	m.pluginLatencyCount.Add(1)
	m.pluginLatencySum.Add(duration.Milliseconds())
}

// RecordCommandInvocation records a CLI command invocation.
// Uses optimistic read lock for pre-initialized commands (hot path),
// falling back to write lock only for unknown commands.
func (m *Metrics) RecordCommandInvocation(command string, duration time.Duration) {
	// Optimistic path: check with read lock (no contention for known commands)
	m.mu.RLock()
	counter := m.commandInvocations[command]
	latencyCount := m.commandLatencyCount[command]
	latencySum := m.commandLatencySum[command]
	m.mu.RUnlock()

	// If not found, initialize with write lock (rare for known commands)
	if counter == nil {
		m.mu.Lock()
		// Double-check after acquiring write lock
		if m.commandInvocations[command] == nil {
			m.commandInvocations[command] = &atomic.Int64{}
			m.commandLatencyCount[command] = &atomic.Int64{}
			m.commandLatencySum[command] = &atomic.Int64{}
		}
		counter = m.commandInvocations[command]
		latencyCount = m.commandLatencyCount[command]
		latencySum = m.commandLatencySum[command]
		m.mu.Unlock()
	}

	// Atomic operations - no lock needed
	counter.Add(1)
	latencyCount.Add(1)
	latencySum.Add(duration.Milliseconds())
}

// SetActiveReleases sets the number of active releases.
func (m *Metrics) SetActiveReleases(count int64) {
	m.activeReleases.Store(count)
}

// IncrementActiveReleases increments the active release count.
func (m *Metrics) IncrementActiveReleases() {
	m.activeReleases.Add(1)
}

// DecrementActiveReleases decrements the active release count.
func (m *Metrics) DecrementActiveReleases() {
	m.activeReleases.Add(-1)
}

// Handler returns an HTTP handler for the /metrics endpoint.
// The output is Prometheus-compatible text format.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var sb strings.Builder

		// Build info
		sb.WriteString("# HELP relicta_info Build information\n")
		sb.WriteString("# TYPE relicta_info gauge\n")
		sb.WriteString(fmt.Sprintf("relicta_info{version=%q} 1\n\n", m.version))

		// Uptime
		uptime := time.Since(m.startTime).Seconds()
		sb.WriteString("# HELP relicta_uptime_seconds Uptime in seconds\n")
		sb.WriteString("# TYPE relicta_uptime_seconds gauge\n")
		sb.WriteString(fmt.Sprintf("relicta_uptime_seconds %.2f\n\n", uptime))

		// Release counters
		sb.WriteString("# HELP relicta_releases_total Total number of releases attempted\n")
		sb.WriteString("# TYPE relicta_releases_total counter\n")
		sb.WriteString(fmt.Sprintf("relicta_releases_total %d\n\n", m.releasesTotal.Load()))

		sb.WriteString("# HELP relicta_releases_successful_total Total number of successful releases\n")
		sb.WriteString("# TYPE relicta_releases_successful_total counter\n")
		sb.WriteString(fmt.Sprintf("relicta_releases_successful_total %d\n\n", m.releasesSuccessful.Load()))

		sb.WriteString("# HELP relicta_releases_failed_total Total number of failed releases\n")
		sb.WriteString("# TYPE relicta_releases_failed_total counter\n")
		sb.WriteString(fmt.Sprintf("relicta_releases_failed_total %d\n\n", m.releasesFailed.Load()))

		// Active releases gauge
		sb.WriteString("# HELP relicta_active_releases Number of releases currently in progress\n")
		sb.WriteString("# TYPE relicta_active_releases gauge\n")
		sb.WriteString(fmt.Sprintf("relicta_active_releases %d\n\n", m.activeReleases.Load()))

		// Release latency
		count := m.releaseLatencyCount.Load()
		sum := m.releaseLatencySum.Load()
		sb.WriteString("# HELP relicta_release_duration_milliseconds Release operation duration\n")
		sb.WriteString("# TYPE relicta_release_duration_milliseconds summary\n")
		sb.WriteString(fmt.Sprintf("relicta_release_duration_milliseconds_count %d\n", count))
		sb.WriteString(fmt.Sprintf("relicta_release_duration_milliseconds_sum %d\n\n", sum))

		// Plugin metrics
		sb.WriteString("# HELP relicta_plugin_executions_total Total plugin executions\n")
		sb.WriteString("# TYPE relicta_plugin_executions_total counter\n")
		sb.WriteString(fmt.Sprintf("relicta_plugin_executions_total %d\n\n", m.pluginExecutions.Load()))

		sb.WriteString("# HELP relicta_plugin_errors_total Total plugin errors\n")
		sb.WriteString("# TYPE relicta_plugin_errors_total counter\n")
		sb.WriteString(fmt.Sprintf("relicta_plugin_errors_total %d\n\n", m.pluginErrors.Load()))

		// Plugin latency
		pluginCount := m.pluginLatencyCount.Load()
		pluginSum := m.pluginLatencySum.Load()
		sb.WriteString("# HELP relicta_plugin_duration_milliseconds Plugin execution duration\n")
		sb.WriteString("# TYPE relicta_plugin_duration_milliseconds summary\n")
		sb.WriteString(fmt.Sprintf("relicta_plugin_duration_milliseconds_count %d\n", pluginCount))
		sb.WriteString(fmt.Sprintf("relicta_plugin_duration_milliseconds_sum %d\n\n", pluginSum))

		// Command invocations
		sb.WriteString("# HELP relicta_command_invocations_total CLI command invocations\n")
		sb.WriteString("# TYPE relicta_command_invocations_total counter\n")

		m.mu.RLock()
		commands := make([]string, 0, len(m.commandInvocations))
		for cmd := range m.commandInvocations {
			commands = append(commands, cmd)
		}
		sort.Strings(commands)

		for _, cmd := range commands {
			sb.WriteString(fmt.Sprintf("relicta_command_invocations_total{command=%q} %d\n",
				cmd, m.commandInvocations[cmd].Load()))
		}
		m.mu.RUnlock()

		sb.WriteString("\n")

		// Command latency
		sb.WriteString("# HELP relicta_command_duration_milliseconds CLI command duration\n")
		sb.WriteString("# TYPE relicta_command_duration_milliseconds summary\n")

		m.mu.RLock()
		for _, cmd := range commands {
			if m.commandLatencyCount[cmd] != nil {
				sb.WriteString(fmt.Sprintf("relicta_command_duration_milliseconds_count{command=%q} %d\n",
					cmd, m.commandLatencyCount[cmd].Load()))
				sb.WriteString(fmt.Sprintf("relicta_command_duration_milliseconds_sum{command=%q} %d\n",
					cmd, m.commandLatencySum[cmd].Load()))
			}
		}
		m.mu.RUnlock()

		_, _ = w.Write([]byte(sb.String()))
	})
}

// Snapshot returns a snapshot of current metrics.
func (m *Metrics) Snapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	commandCounts := make(map[string]int64)
	for cmd, count := range m.commandInvocations {
		commandCounts[cmd] = count.Load()
	}

	return MetricsSnapshot{
		ReleasesTotal:      m.releasesTotal.Load(),
		ReleasesSuccessful: m.releasesSuccessful.Load(),
		ReleasesFailed:     m.releasesFailed.Load(),
		ActiveReleases:     m.activeReleases.Load(),
		PluginExecutions:   m.pluginExecutions.Load(),
		PluginErrors:       m.pluginErrors.Load(),
		CommandInvocations: commandCounts,
		Uptime:             time.Since(m.startTime),
	}
}

// MetricsSnapshot represents a point-in-time snapshot of metrics.
type MetricsSnapshot struct {
	ReleasesTotal      int64
	ReleasesSuccessful int64
	ReleasesFailed     int64
	ActiveReleases     int64
	PluginExecutions   int64
	PluginErrors       int64
	CommandInvocations map[string]int64
	Uptime             time.Duration
}

// Global metrics instance with separate sync.Once for initialization control.
// This prevents race conditions where Global() could initialize with "unknown"
// before InitGlobal() is called.
var (
	globalMetrics     *Metrics
	globalMetricsOnce sync.Once
	initOnce          sync.Once
	initialized       bool
)

// Global returns the global metrics instance.
// If InitGlobal has not been called, this will initialize with "unknown" version.
// For proper initialization, call InitGlobal before any calls to Global.
func Global() *Metrics {
	globalMetricsOnce.Do(func() {
		if !initialized {
			globalMetrics = NewMetrics("unknown")
		}
	})
	return globalMetrics
}

// InitGlobal initializes the global metrics instance with version info.
// This should be called early in application startup, before any calls to Global().
// If Global() was called first, the version may already be set to "unknown".
func InitGlobal(version string) *Metrics {
	initOnce.Do(func() {
		initialized = true
		globalMetrics = NewMetrics(version)
	})
	// Also trigger globalMetricsOnce so Global() returns correctly
	globalMetricsOnce.Do(func() {})
	return globalMetrics
}
