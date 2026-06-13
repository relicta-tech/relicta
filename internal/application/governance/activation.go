package governance

import (
	"context"
	"fmt"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/memory"
	"github.com/relicta-tech/relicta/v4/internal/cgp/reputation"
	"github.com/relicta-tech/relicta/v4/internal/cgp/risk"
)

const (
	// calibrationHistoryLimit bounds how many historical releases feed a
	// calibration run.
	calibrationHistoryLimit = 500

	// reputationHistoryLimit bounds how many historical releases/incidents feed
	// a reputation computation.
	reputationHistoryLimit = 200

	// minReputationSamples is the minimum number of an actor's own release
	// records required before reputation may tighten a decision. Below this we
	// have too little signal and never penalize the actor.
	minReputationSamples = 3
)

// ensureCalibrated retunes the evaluator's risk weights from historical release
// outcomes exactly once per Service lifetime. Failures are logged and ignored —
// calibration is an optimization, never a hard dependency of evaluation.
func (s *Service) ensureCalibrated(ctx context.Context, repository string) {
	s.calibrateOnce.Do(func() {
		records, err := s.memoryStore.GetReleaseHistory(ctx, repository, calibrationHistoryLimit)
		if err != nil {
			s.logger.Warn("risk calibration skipped: could not load release history", "error", err)
			return
		}
		if len(records) == 0 {
			s.logger.Debug("risk calibration skipped: no historical releases")
			return
		}

		calRecords := toCalibrationRecords(records)
		result := risk.NewCalibrator().Calibrate(calRecords)

		// Validate prediction accuracy before trusting the new weights. Stale or
		// noisy history can yield weights that degrade risk scoring silently.
		if s.calibrationMinAccuracy > 0 && result.Accuracy < s.calibrationMinAccuracy {
			if s.calibrationStrict {
				s.logger.Warn("risk calibration rejected: accuracy below threshold, keeping default weights",
					"accuracy", result.Accuracy,
					"min_accuracy", s.calibrationMinAccuracy,
					"samples", result.SampleSize)
				return
			}
			s.logger.Warn("risk calibration accuracy below threshold; applying anyway (non-strict)",
				"accuracy", result.Accuracy,
				"min_accuracy", s.calibrationMinAccuracy,
				"samples", result.SampleSize)
		}

		s.evaluator.ApplyCalibration(result)

		s.logger.Info("risk weights calibrated from history",
			"samples", result.SampleSize,
			"accuracy", result.Accuracy,
			"calibrated_at", result.CalibratedAt)
	})
}

// toCalibrationRecords maps memory release records into calibration records.
// Per-factor scores are approximated from the record's tags carrying the
// release's overall risk score, mirroring the analytics calibration path.
func toCalibrationRecords(records []*memory.ReleaseRecord) []risk.CalibrationRecord {
	out := make([]risk.CalibrationRecord, 0, len(records))
	for _, r := range records {
		if r == nil {
			continue
		}
		factorScores := make(map[string]float64, len(r.Tags))
		for _, tag := range r.Tags {
			factorScores[tag] = r.RiskScore
		}
		out = append(out, risk.CalibrationRecord{
			RiskScore:    r.RiskScore,
			FactorScores: factorScores,
			Outcome:      string(r.Outcome),
			ReleasedAt:   r.ReleasedAt,
		})
	}
	return out
}

// applyReputationGuard computes the initiating actor's reputation and, when it
// is poor enough on a sufficient sample, downgrades an auto-approved decision to
// require human review. It only ever tightens the decision.
func (s *Service) applyReputationGuard(ctx context.Context, input EvaluateReleaseInput, output *EvaluateReleaseOutput) {
	score, ok := s.computeReputation(ctx, input.Repository, input.Actor.ID)
	if !ok {
		return
	}

	info := &ReputationInfo{
		Overall:    score.Overall,
		Level:      score.Level(),
		SampleSize: score.SampleSize,
		Trend:      string(score.Trend),
	}
	output.Reputation = info

	// Only act on actors with a real, sufficiently-sampled poor track record.
	// Newcomers (SampleSize below the floor) are never penalized.
	if score.SampleSize < minReputationSamples {
		return
	}
	probationThreshold := reputation.ThresholdProbation
	if s.reputationProbationThreshold > 0 {
		probationThreshold = s.reputationProbationThreshold
	}
	if score.Overall >= probationThreshold {
		return
	}

	// Tighten: an auto-approved / approved decision now needs human review.
	if output.Decision == cgp.DecisionApproved {
		output.Decision = cgp.DecisionApprovalRequired
		output.CanAutoApprove = false
		info.Guarded = true
		rationale := fmt.Sprintf("actor reputation is restricted (%.1f%%, %s) — human review required",
			score.Overall*100, score.Level())
		output.Rationale = append(output.Rationale, rationale)
		output.RequiredActions = append(output.RequiredActions, cgp.RequiredAction{
			Type:        "human_approval",
			Description: "Review release from a low-reputation actor before proceeding",
		})
		s.logger.Warn("reputation guard downgraded decision",
			"actor", input.Actor.ID,
			"reputation", score.Overall,
			"level", score.Level(),
			"samples", score.SampleSize)
	}
}

// computeReputation loads the actor's release and incident history and computes
// a reputation score. Returns ok=false when history cannot be loaded.
func (s *Service) computeReputation(ctx context.Context, repository, actorID string) (reputation.Score, bool) {
	releases, err := s.memoryStore.GetReleaseHistory(ctx, repository, reputationHistoryLimit)
	if err != nil {
		s.logger.Debug("reputation skipped: could not load release history", "error", err)
		return reputation.Score{}, false
	}
	incidents, err := s.memoryStore.GetIncidentHistory(ctx, repository, reputationHistoryLimit)
	if err != nil {
		// Incidents are optional; proceed with releases only.
		s.logger.Debug("reputation: incident history unavailable", "error", err)
		incidents = nil
	}

	engine, err := reputation.NewEngine(noopReputationStore{}, reputation.WithLogger(s.logger))
	if err != nil {
		s.logger.Debug("reputation skipped: engine init failed", "error", err)
		return reputation.Score{}, false
	}
	return engine.ComputeScore(releases, incidents, actorID), true
}

// noopReputationStore satisfies reputation.ReputationStore for compute-only use.
// ComputeScore is pure; the engine never touches the store on this path, so we
// avoid persisting reputation during evaluation.
type noopReputationStore struct{}

func (noopReputationStore) GetScore(context.Context, string) (*reputation.Score, error) {
	return nil, nil
}
func (noopReputationStore) SaveScore(context.Context, string, *reputation.Score) error {
	return nil
}
func (noopReputationStore) GetHistory(context.Context, string, int) ([]reputation.Score, error) {
	return nil, nil
}
