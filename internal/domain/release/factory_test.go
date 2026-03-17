package release

import (
	"encoding/json"
	"testing"
)

func TestNewServices(t *testing.T) {
	cfg := Config{}
	services, err := NewServices(cfg)
	if err != nil {
		t.Fatalf("NewServices() error = %v", err)
	}

	if services.PlanRelease == nil {
		t.Error("PlanRelease should not be nil")
	}
	if services.BumpVersion == nil {
		t.Error("BumpVersion should not be nil")
	}
	if services.GenerateNotes == nil {
		t.Error("GenerateNotes should not be nil")
	}
	if services.ApproveRelease == nil {
		t.Error("ApproveRelease should not be nil")
	}
	if services.PublishRelease == nil {
		t.Error("PublishRelease should not be nil")
	}
	if services.RetryPublish == nil {
		t.Error("RetryPublish should not be nil")
	}
	if services.GetStatus == nil {
		t.Error("GetStatus should not be nil")
	}
	if services.Repository == nil {
		t.Error("Repository should not be nil")
	}
	if services.RepoInspector == nil {
		t.Error("RepoInspector should not be nil")
	}
	if services.LockManager == nil {
		t.Error("LockManager should not be nil")
	}
	if services.StateMachine == nil {
		t.Error("StateMachine should not be nil")
	}
}

func TestExportStateMachineJSON(t *testing.T) {
	cfg := Config{}
	services, err := NewServices(cfg)
	if err != nil {
		t.Fatalf("NewServices() error = %v", err)
	}

	data, err := services.ExportStateMachineJSON()
	if err != nil {
		t.Fatalf("ExportStateMachineJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON output")
	}

	// Validate it's valid JSON
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}
}
