package governance

import (
	"testing"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/attribution"
	"github.com/relicta-tech/relicta/v4/internal/cgp/evaluator"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/policy"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
	"github.com/relicta-tech/relicta/v4/internal/domain/release"
	"github.com/relicta-tech/relicta/v4/internal/domain/version"
)

// agentReleaseWith builds a release whose single commit is authored by the given
// name/email, used to exercise authorship detection.
func agentReleaseWith(t *testing.T, author, email string) *release.ReleaseRun {
	t.Helper()
	rel := release.NewReleaseRunForTest("release-attr", "main", "owner/repo")
	cs := changes.NewChangeSet("cs-attr", "v1.0.0", "HEAD")
	cs.AddCommit(changes.NewConventionalCommit(
		"deadbeef", changes.CommitTypeFeat, "add feature",
		changes.WithAuthor(author, email),
	))
	current, _ := version.Parse("1.0.0")
	next, _ := version.Parse("1.1.0")
	plan := release.NewReleasePlan(current, next, changes.ReleaseTypeMinor, cs, false)
	_ = release.SetPlan(rel, plan)
	return rel
}

func detectionFor(actor cgp.Actor, confidence float64) *attribution.DetectionResult {
	return &attribution.DetectionResult{Actor: actor, Confidence: confidence}
}

func TestApplyAttribution_MachineAuthorBehindHumanGoverns(t *testing.T) {
	initiator := cgp.NewHumanActor("dev@example.com", "Dev")
	detection := detectionFor(cgp.NewAgentActor("claude", "Claude Code", "claude"), 0.9)

	actor, conf := applyAttribution(initiator, detection)
	if actor.Kind != cgp.ActorKindAgent {
		t.Fatalf("machine author must govern; got kind %s", actor.Kind)
	}
	if actor.ID != detection.Actor.ID {
		t.Fatalf("expected detected agent to govern; got %s", actor.ID)
	}
	if conf != detection.Confidence {
		t.Fatalf("confidence must reflect detection; got %v", conf)
	}
}

func TestApplyAttribution_HumanAuthorKeepsInitiator(t *testing.T) {
	initiator := cgp.NewHumanActor("dev@example.com", "Dev")
	human := detectionFor(cgp.NewHumanActor("other@example.com", "Other"), 0.95)

	actor, _ := applyAttribution(initiator, human)
	if actor.ID != initiator.ID {
		t.Fatalf("human author must keep initiator; got %s", actor.ID)
	}
}

func TestApplyAttribution_MachineInitiatorUnchanged(t *testing.T) {
	initiator := cgp.NewCIActor("github-actions", "release", "1")
	agent := detectionFor(cgp.NewAgentActor("claude", "Claude Code", "claude"), 0.9)

	actor, _ := applyAttribution(initiator, agent)
	if actor.ID != initiator.ID {
		t.Fatalf("machine initiator must be left in place; got %s", actor.ID)
	}
}

func TestApplyAttribution_NilDetectionIsFullConfidence(t *testing.T) {
	initiator := cgp.NewHumanActor("dev@example.com", "Dev")
	actor, conf := applyAttribution(initiator, nil)
	if actor.ID != initiator.ID || conf != 1.0 {
		t.Fatalf("nil detection must keep initiator at full confidence; got %s %v", actor.ID, conf)
	}
}

func TestParseTrailers(t *testing.T) {
	footer := "Co-Authored-By: Claude <noreply@anthropic.com>\nAI-Agent: Claude\nthis is prose, not a trailer: nope"
	got := parseTrailers(footer)
	if got["AI-Agent"] != "Claude" {
		t.Fatalf("expected AI-Agent trailer; got %#v", got)
	}
	if _, ok := got["this is prose, not a trailer"]; ok {
		t.Fatal("prose line with spaces in key must not be treated as a trailer")
	}
	if parseTrailers("") != nil {
		t.Fatal("empty footer must yield nil")
	}
}

// TestBuildProposal_AttributionGovernsAgentAuthor verifies the full
// buildProposalAndAnalysis path: an agent-authored changeset under a human
// initiator yields an agent-governed proposal when attribution is enabled.
func TestBuildProposal_AttributionGovernsAgentAuthor(t *testing.T) {
	svc := newAttributionService(true)
	rel := agentReleaseWith(t, "Claude", "claude@anthropic.com")
	input := EvaluateReleaseInput{
		Release:    rel,
		Actor:      cgp.NewHumanActor("dev@example.com", "Dev"),
		Repository: "owner/repo",
	}

	proposal, _ := svc.buildProposalAndAnalysis(input)
	if proposal.Actor.Kind != cgp.ActorKindAgent {
		t.Fatalf("agent-authored release must be governed as agent; got %s", proposal.Actor.Kind)
	}
	if proposal.Context == nil || proposal.Context.Metadata["attribution.initiator"] != "human:dev@example.com" {
		t.Fatalf("initiator must be preserved in attribution context; got %#v", proposal.Context)
	}
}

func TestBuildProposal_AttributionDisabledKeepsInitiator(t *testing.T) {
	svc := newAttributionService(false)
	rel := agentReleaseWith(t, "Claude", "claude@anthropic.com")
	input := EvaluateReleaseInput{
		Release:    rel,
		Actor:      cgp.NewHumanActor("dev@example.com", "Dev"),
		Repository: "owner/repo",
	}

	proposal, _ := svc.buildProposalAndAnalysis(input)
	if proposal.Actor.Kind != cgp.ActorKindHuman {
		t.Fatalf("attribution disabled must keep human initiator; got %s", proposal.Actor.Kind)
	}
}

func newAttributionService(enabled bool) *Service {
	eval := evaluator.New(
		evaluator.WithRiskCalculator(risk.NewCalculatorWithDefaults()),
		evaluator.WithPolicyEngine(policy.NewEngine(nil, nil)),
	)
	return NewService(eval, WithMemoryStore(memory.NewInMemoryStore()), WithAttribution(enabled))
}
