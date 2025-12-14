// Package observability provides metrics and monitoring for Relicta.
package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics("1.0.0")
	if m == nil {
		t.Fatal("Expected non-nil metrics")
	}
	if m.version != "1.0.0" {
		t.Errorf("version = %v, want 1.0.0", m.version)
	}
}

func TestMetrics_RecordRelease_Success(t *testing.T) {
	m := NewMetrics("1.0.0")

	m.RecordRelease(true, 100*time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.ReleasesTotal != 1 {
		t.Errorf("ReleasesTotal = %d, want 1", snapshot.ReleasesTotal)
	}
	if snapshot.ReleasesSuccessful != 1 {
		t.Errorf("ReleasesSuccessful = %d, want 1", snapshot.ReleasesSuccessful)
	}
	if snapshot.ReleasesFailed != 0 {
		t.Errorf("ReleasesFailed = %d, want 0", snapshot.ReleasesFailed)
	}
}

func TestMetrics_RecordRelease_Failure(t *testing.T) {
	m := NewMetrics("1.0.0")

	m.RecordRelease(false, 50*time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.ReleasesTotal != 1 {
		t.Errorf("ReleasesTotal = %d, want 1", snapshot.ReleasesTotal)
	}
	if snapshot.ReleasesSuccessful != 0 {
		t.Errorf("ReleasesSuccessful = %d, want 0", snapshot.ReleasesSuccessful)
	}
	if snapshot.ReleasesFailed != 1 {
		t.Errorf("ReleasesFailed = %d, want 1", snapshot.ReleasesFailed)
	}
}

func TestMetrics_RecordPluginExecution(t *testing.T) {
	m := NewMetrics("1.0.0")

	m.RecordPluginExecution("github", true, 200*time.Millisecond)
	m.RecordPluginExecution("slack", false, 100*time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.PluginExecutions != 2 {
		t.Errorf("PluginExecutions = %d, want 2", snapshot.PluginExecutions)
	}
	if snapshot.PluginErrors != 1 {
		t.Errorf("PluginErrors = %d, want 1", snapshot.PluginErrors)
	}
}

func TestMetrics_RecordCommandInvocation(t *testing.T) {
	m := NewMetrics("1.0.0")

	m.RecordCommandInvocation("plan", 100*time.Millisecond)
	m.RecordCommandInvocation("plan", 150*time.Millisecond)
	m.RecordCommandInvocation("publish", 500*time.Millisecond)

	snapshot := m.Snapshot()
	if snapshot.CommandInvocations["plan"] != 2 {
		t.Errorf("CommandInvocations[plan] = %d, want 2", snapshot.CommandInvocations["plan"])
	}
	if snapshot.CommandInvocations["publish"] != 1 {
		t.Errorf("CommandInvocations[publish] = %d, want 1", snapshot.CommandInvocations["publish"])
	}
}

func TestMetrics_ActiveReleases(t *testing.T) {
	m := NewMetrics("1.0.0")

	m.SetActiveReleases(5)
	snapshot := m.Snapshot()
	if snapshot.ActiveReleases != 5 {
		t.Errorf("ActiveReleases = %d, want 5", snapshot.ActiveReleases)
	}

	m.IncrementActiveReleases()
	snapshot = m.Snapshot()
	if snapshot.ActiveReleases != 6 {
		t.Errorf("ActiveReleases = %d, want 6", snapshot.ActiveReleases)
	}

	m.DecrementActiveReleases()
	snapshot = m.Snapshot()
	if snapshot.ActiveReleases != 5 {
		t.Errorf("ActiveReleases = %d, want 5", snapshot.ActiveReleases)
	}
}

func TestMetrics_Handler(t *testing.T) {
	m := NewMetrics("1.2.3")
	m.RecordRelease(true, 100*time.Millisecond)
	m.RecordPluginExecution("github", true, 50*time.Millisecond)
	m.RecordCommandInvocation("plan", 200*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	m.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()

	// Check content type
	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Content-Type = %s, want text/plain", contentType)
	}

	// Check for expected metrics
	expectedMetrics := []string{
		"relicta_info",
		"relicta_uptime_seconds",
		"relicta_releases_total 1",
		"relicta_releases_successful_total 1",
		"relicta_releases_failed_total 0",
		"relicta_plugin_executions_total 1",
		"relicta_command_invocations_total{command=\"plan\"} 1",
	}

	for _, expected := range expectedMetrics {
		if !strings.Contains(body, expected) {
			t.Errorf("Expected metrics output to contain %q", expected)
		}
	}
}

func TestMetrics_Handler_Empty(t *testing.T) {
	m := NewMetrics("1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	m.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()

	// Should still have info and zero counters
	if !strings.Contains(body, "relicta_releases_total 0") {
		t.Error("Expected zero release counter")
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	m := NewMetrics("1.0.0")
	m.RecordRelease(true, 100*time.Millisecond)
	m.RecordRelease(false, 50*time.Millisecond)
	m.SetActiveReleases(3)
	m.RecordPluginExecution("test", true, 10*time.Millisecond)
	m.RecordCommandInvocation("version", 5*time.Millisecond)

	time.Sleep(10 * time.Millisecond) // Give uptime a non-zero value

	snapshot := m.Snapshot()

	if snapshot.ReleasesTotal != 2 {
		t.Errorf("ReleasesTotal = %d, want 2", snapshot.ReleasesTotal)
	}
	if snapshot.ReleasesSuccessful != 1 {
		t.Errorf("ReleasesSuccessful = %d, want 1", snapshot.ReleasesSuccessful)
	}
	if snapshot.ReleasesFailed != 1 {
		t.Errorf("ReleasesFailed = %d, want 1", snapshot.ReleasesFailed)
	}
	if snapshot.ActiveReleases != 3 {
		t.Errorf("ActiveReleases = %d, want 3", snapshot.ActiveReleases)
	}
	if snapshot.PluginExecutions != 1 {
		t.Errorf("PluginExecutions = %d, want 1", snapshot.PluginExecutions)
	}
	if snapshot.Uptime <= 0 {
		t.Error("Uptime should be > 0")
	}
}

func TestMetrics_ConcurrentAccess(t *testing.T) {
	m := NewMetrics("1.0.0")

	done := make(chan bool)

	// Start multiple goroutines recording metrics
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.RecordRelease(true, time.Millisecond)
				m.RecordPluginExecution("test", true, time.Millisecond)
				m.RecordCommandInvocation("plan", time.Millisecond)
				m.IncrementActiveReleases()
				m.DecrementActiveReleases()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	snapshot := m.Snapshot()
	if snapshot.ReleasesTotal != 1000 {
		t.Errorf("ReleasesTotal = %d, want 1000", snapshot.ReleasesTotal)
	}
	if snapshot.PluginExecutions != 1000 {
		t.Errorf("PluginExecutions = %d, want 1000", snapshot.PluginExecutions)
	}
	if snapshot.CommandInvocations["plan"] != 1000 {
		t.Errorf("CommandInvocations[plan] = %d, want 1000", snapshot.CommandInvocations["plan"])
	}
	if snapshot.ActiveReleases != 0 {
		t.Errorf("ActiveReleases = %d, want 0 (after increments and decrements)", snapshot.ActiveReleases)
	}
}

func TestGlobal(t *testing.T) {
	// Get the global instance
	m := Global()
	if m == nil {
		t.Fatal("Global() returned nil")
	}

	// Should return same instance
	m2 := Global()
	if m != m2 {
		t.Error("Global() should return the same instance")
	}
}
