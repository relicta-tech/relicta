package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestApprovalModel_UpdateKeys(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{
		CurrentVersion: "1.0.0",
		NextVersion:    "1.1.0",
		ReleaseType:    "minor",
	})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	model = updated.(ApprovalModel)

	updated, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	model = updated.(ApprovalModel)
	if model.Result() != ApprovalAccepted {
		t.Errorf("Result = %v, want %v", model.Result(), ApprovalAccepted)
	}
	if cmd == nil {
		t.Errorf("expected quit command for approval")
	}
}

func TestApprovalModel_ViewSections(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{
		CurrentVersion: "1.0.0",
		NextVersion:    "1.1.0",
		ReleaseType:    "minor",
		CommitCount:    5,
		Branch:         "main",
		ReleaseNotes:   "Changes",
		Plugins:        []string{"github"},
		Governance: &GovernanceSummary{
			RiskScore:      0.4,
			Severity:       "medium",
			Decision:       "approval_required",
			CanAutoApprove: false,
			RiskFactors: []RiskFactor{
				{Category: "scope", Description: "critical", Score: 0.6},
			},
			RequiredActions: []string{"human_review"},
		},
	})

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	model = updated.(ApprovalModel)
	model.showNotes = true

	view := model.View()
	if !strings.Contains(view, "Release Summary") {
		t.Errorf("view missing summary section")
	}
	if !strings.Contains(view, "Governance") {
		t.Errorf("view missing governance section")
	}
	if !strings.Contains(view, "Plugins") {
		t.Errorf("view missing plugins section")
	}
	if !strings.Contains(view, "Release Notes") {
		t.Errorf("view missing release notes section")
	}
}

func TestApprovalModel_ViewNotReady(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{})
	view := model.View()
	if view != "Initializing..." {
		t.Errorf("View = %q, want %q", view, "Initializing...")
	}
}

func TestApprovalModel_ToggleNotes(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{ReleaseNotes: "notes"})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	model = updated.(ApprovalModel)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(ApprovalModel)
	if !model.showNotes || !model.focused {
		t.Error("expected notes to be shown and focused")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyTab})
	model = updated.(ApprovalModel)
	if model.showNotes || model.focused {
		t.Error("expected notes to be hidden and not focused")
	}
}

func TestApprovalModel_ViewHelp(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	model = updated.(ApprovalModel)

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model = updated.(ApprovalModel)

	view := model.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Errorf("view missing help section")
	}
}

func TestApprovalModel_NoNotesContent(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	model = updated.(ApprovalModel)
	model.showNotes = true

	view := model.View()
	if !strings.Contains(view, "No release notes available") {
		t.Errorf("view missing empty notes message")
	}
}

func TestApprovalModel_Init(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{})
	if cmd := model.Init(); cmd != nil {
		t.Errorf("Init() = %v, want nil", cmd)
	}
}

func TestApprovalModel_RenderChangesOverview(t *testing.T) {
	model := NewApprovalModel(ReleaseSummary{
		BreakingCount: 1,
		FeatureCount:  2,
		FixCount:      3,
		PerfCount:     4,
		OtherCount:    5,
	})

	output := model.renderChangesOverview()
	if !strings.Contains(output, "Breaking") || !strings.Contains(output, "Features") {
		t.Errorf("renderChangesOverview missing expected content")
	}
}

func TestRunApprovalTUI_ReturnsResult(t *testing.T) {
	origNewProgram := newApprovalProgram
	t.Cleanup(func() { newApprovalProgram = origNewProgram })
	newApprovalProgram = func(model ApprovalModel) approvalProgramRunner {
		return stubApprovalProgram{model: ApprovalModel{result: ApprovalAccepted}}
	}

	result, err := RunApprovalTUI(ReleaseSummary{})
	if err != nil {
		t.Fatalf("RunApprovalTUI error: %v", err)
	}
	if result != ApprovalAccepted {
		t.Fatalf("RunApprovalTUI result = %v, want %v", result, ApprovalAccepted)
	}
}

func TestRunApprovalTUI_UnexpectedModel(t *testing.T) {
	origNewProgram := newApprovalProgram
	t.Cleanup(func() { newApprovalProgram = origNewProgram })
	newApprovalProgram = func(model ApprovalModel) approvalProgramRunner {
		return stubApprovalProgram{model: invalidModel{}}
	}

	if _, err := RunApprovalTUI(ReleaseSummary{}); err == nil {
		t.Fatal("expected RunApprovalTUI to fail with unexpected model")
	}
}

type stubApprovalProgram struct {
	model tea.Model
	err   error
}

func (s stubApprovalProgram) Run() (tea.Model, error) {
	return s.model, s.err
}

type invalidModel struct{}

func (invalidModel) Init() tea.Cmd { return nil }

func (invalidModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return invalidModel{}, nil }

func (invalidModel) View() string { return "" }
