package app

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/domain/release/domain"
)

func TestSetAttestationEnabled(t *testing.T) {
	uc := &ApproveReleaseUseCase{}

	if uc.attestationEnabled {
		t.Error("expected attestation disabled by default")
	}

	uc.SetAttestationEnabled(true)
	if !uc.attestationEnabled {
		t.Error("expected attestation enabled after SetAttestationEnabled(true)")
	}

	uc.SetAttestationEnabled(false)
	if uc.attestationEnabled {
		t.Error("expected attestation disabled after SetAttestationEnabled(false)")
	}
}

func TestEnsureAttestationStep(t *testing.T) {
	uc := &ApproveReleaseUseCase{}

	t.Run("adds attestation step when missing", func(t *testing.T) {
		run := domain.NewReleaseRun("repo", "/path", "v1.0.0", "abc123", nil, "cfg", "plug")
		run.SetExecutionPlan([]domain.StepPlan{
			{Name: "create-tag", Type: domain.StepTypeTag},
		})

		uc.ensureAttestationStep(run)

		steps := run.Steps()
		found := false
		for _, s := range steps {
			if s.Type == domain.StepTypeAttestation {
				found = true
			}
		}
		if !found {
			t.Error("expected attestation step to be added")
		}
		if len(steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(steps))
		}
	})

	t.Run("does not duplicate attestation step", func(t *testing.T) {
		run := domain.NewReleaseRun("repo", "/path", "v1.0.0", "abc123", nil, "cfg", "plug")
		run.SetExecutionPlan([]domain.StepPlan{
			{Name: "create-tag", Type: domain.StepTypeTag},
			{Name: "generate-attestation", Type: domain.StepTypeAttestation},
		})

		uc.ensureAttestationStep(run)

		steps := run.Steps()
		count := 0
		for _, s := range steps {
			if s.Type == domain.StepTypeAttestation {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 attestation step, got %d", count)
		}
	})

	t.Run("works with empty execution plan", func(t *testing.T) {
		run := domain.NewReleaseRun("repo", "/path", "v1.0.0", "abc123", nil, "cfg", "plug")
		run.SetExecutionPlan(nil)

		uc.ensureAttestationStep(run)

		steps := run.Steps()
		if len(steps) != 1 {
			t.Errorf("expected 1 step, got %d", len(steps))
		}
		if steps[0].Type != domain.StepTypeAttestation {
			t.Errorf("expected attestation step, got %v", steps[0].Type)
		}
	})
}

func TestEnsureTagStep(t *testing.T) {
	uc := &ApproveReleaseUseCase{}

	t.Run("adds tag step when missing", func(t *testing.T) {
		run := domain.NewReleaseRun("repo", "/path", "v1.0.0", "abc123", nil, "cfg", "plug")
		run.SetExecutionPlan([]domain.StepPlan{
			{Name: "generate-attestation", Type: domain.StepTypeAttestation},
		})

		uc.ensureTagStep(run)

		steps := run.Steps()
		if len(steps) != 2 {
			t.Errorf("expected 2 steps, got %d", len(steps))
		}
		// Tag step should be first
		if steps[0].Type != domain.StepTypeTag {
			t.Errorf("expected first step to be tag, got %v", steps[0].Type)
		}
	})

	t.Run("does not duplicate tag step", func(t *testing.T) {
		run := domain.NewReleaseRun("repo", "/path", "v1.0.0", "abc123", nil, "cfg", "plug")
		run.SetExecutionPlan([]domain.StepPlan{
			{Name: "create-tag", Type: domain.StepTypeTag},
			{Name: "generate-attestation", Type: domain.StepTypeAttestation},
		})

		uc.ensureTagStep(run)

		steps := run.Steps()
		count := 0
		for _, s := range steps {
			if s.Type == domain.StepTypeTag {
				count++
			}
		}
		if count != 1 {
			t.Errorf("expected exactly 1 tag step, got %d", count)
		}
	})

	t.Run("works with empty execution plan", func(t *testing.T) {
		run := domain.NewReleaseRun("repo", "/path", "v1.0.0", "abc123", nil, "cfg", "plug")
		run.SetExecutionPlan(nil)

		uc.ensureTagStep(run)

		steps := run.Steps()
		if len(steps) != 1 {
			t.Errorf("expected 1 step, got %d", len(steps))
		}
		if steps[0].Type != domain.StepTypeTag {
			t.Errorf("expected tag step, got %v", steps[0].Type)
		}
	})
}
