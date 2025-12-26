package domain

import (
	"testing"
	"time"
)

func TestAndSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test")

	t.Run("all specs satisfied", func(t *testing.T) {
		spec := And(
			ByState(StatePlanned),
			Active(),
		)
		if !spec.IsSatisfiedBy(run) {
			t.Error("And spec should be satisfied when all child specs are satisfied")
		}
	})

	t.Run("one spec not satisfied", func(t *testing.T) {
		spec := And(
			ByState(StateVersioned), // wrong state
			Active(),
		)
		if spec.IsSatisfiedBy(run) {
			t.Error("And spec should not be satisfied when any child spec fails")
		}
	})

	t.Run("empty specs", func(t *testing.T) {
		spec := And()
		if !spec.IsSatisfiedBy(run) {
			t.Error("And spec with no children should be satisfied")
		}
	})
}

func TestOrSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test")

	t.Run("one spec satisfied", func(t *testing.T) {
		spec := Or(
			ByState(StateVersioned), // wrong state
			ByState(StatePlanned),   // correct state
		)
		if !spec.IsSatisfiedBy(run) {
			t.Error("Or spec should be satisfied when any child spec is satisfied")
		}
	})

	t.Run("no specs satisfied", func(t *testing.T) {
		spec := Or(
			ByState(StateVersioned),
			ByState(StateApproved),
		)
		if spec.IsSatisfiedBy(run) {
			t.Error("Or spec should not be satisfied when no child specs are satisfied")
		}
	})

	t.Run("empty specs", func(t *testing.T) {
		spec := Or()
		if !spec.IsSatisfiedBy(run) {
			t.Error("Or spec with no children should be satisfied")
		}
	})
}

func TestNotSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()
	_ = run.Plan("test")

	t.Run("negates false", func(t *testing.T) {
		spec := Not(ByState(StateVersioned))
		if !spec.IsSatisfiedBy(run) {
			t.Error("Not spec should be satisfied when child is not satisfied")
		}
	})

	t.Run("negates true", func(t *testing.T) {
		spec := Not(ByState(StatePlanned))
		if spec.IsSatisfiedBy(run) {
			t.Error("Not spec should not be satisfied when child is satisfied")
		}
	})
}

func TestStateSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()

	t.Run("matches draft state", func(t *testing.T) {
		spec := ByState(StateDraft)
		if !spec.IsSatisfiedBy(run) {
			t.Error("State spec should match draft state")
		}
	})

	t.Run("does not match wrong state", func(t *testing.T) {
		spec := ByState(StatePlanned)
		if spec.IsSatisfiedBy(run) {
			t.Error("State spec should not match wrong state")
		}
	})
}

func TestActiveSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("active run", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		spec := Active()
		if !spec.IsSatisfiedBy(run) {
			t.Error("Active spec should be satisfied for active runs")
		}
	})

	t.Run("final run", func(t *testing.T) {
		run := newPublishingRun()
		_ = run.MarkPublished("test")
		spec := Active()
		if spec.IsSatisfiedBy(run) {
			t.Error("Active spec should not be satisfied for final runs")
		}
	})
}

func TestFinalSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("final run", func(t *testing.T) {
		run := newPublishingRun()
		_ = run.MarkPublished("test")
		spec := Final()
		if !spec.IsSatisfiedBy(run) {
			t.Error("Final spec should be satisfied for final runs")
		}
	})

	t.Run("active run", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		spec := Final()
		if spec.IsSatisfiedBy(run) {
			t.Error("Final spec should not be satisfied for active runs")
		}
	})
}

func TestRepositoryPathSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()

	t.Run("matching path", func(t *testing.T) {
		spec := ByRepositoryPath("/path/to/repo")
		if !spec.IsSatisfiedBy(run) {
			t.Error("RepoPath spec should match correct path")
		}
	})

	t.Run("non-matching path", func(t *testing.T) {
		spec := ByRepositoryPath("/other/path")
		if spec.IsSatisfiedBy(run) {
			t.Error("RepoPath spec should not match wrong path")
		}
	})
}

func TestRepoIDSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()

	t.Run("matching repo ID", func(t *testing.T) {
		spec := ByRepoID("github.com/test/repo")
		if !spec.IsSatisfiedBy(run) {
			t.Error("RepoID spec should match correct ID")
		}
	})

	t.Run("non-matching repo ID", func(t *testing.T) {
		spec := ByRepoID("github.com/other/repo")
		if spec.IsSatisfiedBy(run) {
			t.Error("RepoID spec should not match wrong ID")
		}
	})
}

func TestReadyForPublishSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("ready run", func(t *testing.T) {
		run := newApprovedRun()
		spec := ReadyForPublish()
		if !spec.IsSatisfiedBy(run) {
			t.Error("ReadyForPublish spec should be satisfied for approved run with version and notes")
		}
	})

	t.Run("not approved", func(t *testing.T) {
		run := newNotesReadyRun()
		spec := ReadyForPublish()
		if spec.IsSatisfiedBy(run) {
			t.Error("ReadyForPublish spec should not be satisfied for non-approved run")
		}
	})
}

func TestHasNotesSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("has notes", func(t *testing.T) {
		run := newNotesReadyRun()
		spec := HasNotes()
		if !spec.IsSatisfiedBy(run) {
			t.Error("HasNotes spec should be satisfied when run has notes")
		}
	})

	t.Run("no notes", func(t *testing.T) {
		run := newVersionedRun()
		spec := HasNotes()
		if spec.IsSatisfiedBy(run) {
			t.Error("HasNotes spec should not be satisfied when run has no notes")
		}
	})
}

func TestIsApprovedSpecification_IsSatisfiedBy(t *testing.T) {
	tests := []struct {
		name      string
		run       *ReleaseRun
		satisfied bool
	}{
		{"approved", newApprovedRun(), true},
		{"publishing", func() *ReleaseRun {
			run := newApprovedRun()
			run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
			_ = run.StartPublishing("test")
			return run
		}(), true},
		{"notes ready", newNotesReadyRun(), false},
		{"versioned", newVersionedRun(), false},
	}

	spec := IsApproved()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if spec.IsSatisfiedBy(tt.run) != tt.satisfied {
				t.Errorf("IsApproved spec IsSatisfiedBy = %v, want %v", !tt.satisfied, tt.satisfied)
			}
		})
	}
}

func TestHeadSHAMatchesSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()

	t.Run("matching SHA", func(t *testing.T) {
		spec := HeadSHAMatches(run.HeadSHA())
		if !spec.IsSatisfiedBy(run) {
			t.Error("HeadSHAMatches spec should be satisfied when SHA matches")
		}
	})

	t.Run("non-matching SHA", func(t *testing.T) {
		spec := HeadSHAMatches("differentsha")
		if spec.IsSatisfiedBy(run) {
			t.Error("HeadSHAMatches spec should not be satisfied when SHA doesn't match")
		}
	})
}

func TestCanBumpSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("planned state", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		spec := CanBump()
		if !spec.IsSatisfiedBy(run) {
			t.Error("CanBump spec should be satisfied in planned state")
		}
	})

	t.Run("non-planned state", func(t *testing.T) {
		run := newVersionedRun()
		spec := CanBump()
		if spec.IsSatisfiedBy(run) {
			t.Error("CanBump spec should not be satisfied in versioned state")
		}
	})
}

func TestCanGenerateNotesSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("versioned state", func(t *testing.T) {
		run := newVersionedRun()
		spec := CanGenerateNotes()
		if !spec.IsSatisfiedBy(run) {
			t.Error("CanGenerateNotes spec should be satisfied in versioned state")
		}
	})

	t.Run("non-versioned state", func(t *testing.T) {
		run := newTestRun()
		_ = run.Plan("test")
		spec := CanGenerateNotes()
		if spec.IsSatisfiedBy(run) {
			t.Error("CanGenerateNotes spec should not be satisfied in planned state")
		}
	})
}

func TestCanApproveSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("notes ready state", func(t *testing.T) {
		run := newNotesReadyRun()
		spec := CanApprove()
		if !spec.IsSatisfiedBy(run) {
			t.Error("CanApprove spec should be satisfied in notes_ready state with notes")
		}
	})

	t.Run("non-notes_ready state", func(t *testing.T) {
		run := newVersionedRun()
		spec := CanApprove()
		if spec.IsSatisfiedBy(run) {
			t.Error("CanApprove spec should not be satisfied in versioned state")
		}
	})
}

func TestRiskBelowThresholdSpecification_IsSatisfiedBy(t *testing.T) {
	run := newTestRun()
	run.SetPolicyEvaluation(0.3, nil, PolicyThresholds{})

	t.Run("below threshold", func(t *testing.T) {
		spec := RiskBelowThreshold(0.5)
		if !spec.IsSatisfiedBy(run) {
			t.Error("RiskBelowThreshold spec should be satisfied when risk is below threshold")
		}
	})

	t.Run("equal to threshold", func(t *testing.T) {
		spec := RiskBelowThreshold(0.3)
		if !spec.IsSatisfiedBy(run) {
			t.Error("RiskBelowThreshold spec should be satisfied when risk equals threshold")
		}
	})

	t.Run("above threshold", func(t *testing.T) {
		spec := RiskBelowThreshold(0.2)
		if spec.IsSatisfiedBy(run) {
			t.Error("RiskBelowThreshold spec should not be satisfied when risk is above threshold")
		}
	})
}

func TestCanAutoApproveSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("can auto approve", func(t *testing.T) {
		run := newTestRun()
		run.SetPolicyEvaluation(0.3, nil, PolicyThresholds{
			AutoApproveRiskThreshold: 0.5,
		})
		spec := CanAutoApprove()
		if !spec.IsSatisfiedBy(run) {
			t.Error("CanAutoApprove spec should be satisfied when risk is below threshold")
		}
	})

	t.Run("cannot auto approve", func(t *testing.T) {
		run := newTestRun()
		run.SetPolicyEvaluation(0.8, nil, PolicyThresholds{
			AutoApproveRiskThreshold: 0.5,
		})
		spec := CanAutoApprove()
		if spec.IsSatisfiedBy(run) {
			t.Error("CanAutoApprove spec should not be satisfied when risk is above threshold")
		}
	})
}

func TestAllStepsSucceededSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("all steps succeeded", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepDone("tag", "done")
		spec := AllStepsSucceeded()
		if !spec.IsSatisfiedBy(run) {
			t.Error("AllStepsSucceeded spec should be satisfied when all steps succeeded")
		}
	})

	t.Run("step failed", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepStarted("tag")
		_ = run.MarkStepFailed("tag", ErrStepNotFound)
		spec := AllStepsSucceeded()
		if spec.IsSatisfiedBy(run) {
			t.Error("AllStepsSucceeded spec should not be satisfied when a step failed")
		}
	})
}

func TestHasFailedStepsSpecification_IsSatisfiedBy(t *testing.T) {
	t.Run("has failed steps", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepStarted("tag")
		_ = run.MarkStepFailed("tag", ErrStepNotFound)
		spec := HasFailedSteps()
		if !spec.IsSatisfiedBy(run) {
			t.Error("HasFailedSteps spec should be satisfied when a step failed")
		}
	})

	t.Run("no failed steps", func(t *testing.T) {
		run := newApprovedRun()
		run.SetExecutionPlan([]StepPlan{{Name: "tag", Type: StepTypeTag}})
		_ = run.StartPublishing("test")
		_ = run.MarkStepDone("tag", "done")
		spec := HasFailedSteps()
		if spec.IsSatisfiedBy(run) {
			t.Error("HasFailedSteps spec should not be satisfied when no steps failed")
		}
	})

	t.Run("no steps at all", func(t *testing.T) {
		run := newApprovedRun()
		spec := HasFailedSteps()
		if spec.IsSatisfiedBy(run) {
			t.Error("HasFailedSteps spec should not be satisfied when there are no steps")
		}
	})
}

func TestComplexSpecificationCombinations(t *testing.T) {
	run := newApprovedRun()

	t.Run("complex AND/OR/NOT", func(t *testing.T) {
		spec := And(
			Or(
				ByState(StateApproved),
				ByState(StatePublishing),
			),
			Not(Final()),
			HasNotes(),
		)
		if !spec.IsSatisfiedBy(run) {
			t.Error("Complex spec should be satisfied for approved run with notes")
		}
	})

	t.Run("nested NOT", func(t *testing.T) {
		spec := Not(Not(Active()))
		if !spec.IsSatisfiedBy(run) {
			t.Error("Double negation should preserve original value")
		}
	})
}

// Helper to create a notes-ready run for specification tests
func newSpecNotesReadyRun() *ReleaseRun {
	run := newVersionedRun()
	notes := &ReleaseNotes{
		Text:        "## Release Notes",
		Provider:    "test",
		GeneratedAt: time.Now(),
	}
	_ = run.GenerateNotes(notes, "input-hash", "test-actor")
	return run
}
