package governance

import (
	"context"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/reputation"
)

const (
	// defaultEarnedTrustMinSamples is the release-record floor for raising an
	// actor to Trusted from track record.
	defaultEarnedTrustMinSamples = 10

	// defaultEarnedTrustFullSamples is the release-record floor for raising an
	// actor to Full (human-equivalent) autonomy from track record.
	defaultEarnedTrustFullSamples = 50

	// earnedTrustFullReputation is the reputation an actor must hold, on top of
	// the sample floor, before earning Full autonomy. Deliberately stricter than
	// the Trusted band so full automation requires a near-spotless record.
	earnedTrustFullReputation = 0.95
)

// applyEarnedTrust raises the governing actor's effective trust level from a
// verifiable track record, so an earned-trusted actor can auto-approve low-risk
// changes downstream. It ONLY ever raises trust, never lowers it, and only from
// stored release outcomes (non-spoofable). Returns the escalation info when an
// escalation occurred, or nil otherwise.
func (s *Service) applyEarnedTrust(ctx context.Context, repository string, proposal *cgp.ChangeProposal) *EarnedTrustInfo {
	actor := proposal.Actor

	score, ok := s.computeReputation(ctx, repository, actor.ID)
	if !ok {
		return nil
	}

	minSamples := s.earnedTrustMinSamples
	if minSamples <= 0 {
		minSamples = defaultEarnedTrustMinSamples
	}
	fullSamples := s.earnedTrustFullSamples
	if fullSamples <= 0 {
		fullSamples = defaultEarnedTrustFullSamples
	}

	earned := earnedTrustLevel(score, minSamples, fullSamples)

	// Escalation only: never lower the trust the actor was constructed with.
	if earned <= actor.TrustLevel {
		return nil
	}

	from := actor.TrustLevel
	proposal.Actor.TrustLevel = earned

	info := &EarnedTrustInfo{
		ActorID:    actor.ID,
		FromLevel:  from.String(),
		ToLevel:    earned.String(),
		Reputation: score.Overall,
		SampleSize: score.SampleSize,
	}

	if proposal.Context == nil {
		proposal.Context = &cgp.ProposalContext{}
	}
	if proposal.Context.Metadata == nil {
		proposal.Context.Metadata = make(map[string]any)
	}
	proposal.Context.Metadata["earned_trust.from"] = info.FromLevel
	proposal.Context.Metadata["earned_trust.to"] = info.ToLevel
	proposal.Context.Metadata["earned_trust.reputation"] = info.Reputation
	proposal.Context.Metadata["earned_trust.samples"] = info.SampleSize

	s.logger.Info("trust raised from track record",
		"actor", actor.ID,
		"from", info.FromLevel,
		"to", info.ToLevel,
		"reputation", score.Overall,
		"samples", score.SampleSize)

	return info
}

// earnedTrustLevel maps a reputation score and sample size to the trust level an
// actor has earned. It returns Untrusted (the zero level) when the record is
// insufficient to earn any escalation, so callers can treat "no signal" and
// "earned nothing" uniformly. A declining trend bars Full autonomy.
func earnedTrustLevel(score reputation.Score, minSamples, fullSamples int) cgp.TrustLevel {
	if score.SampleSize >= fullSamples &&
		score.Overall >= earnedTrustFullReputation &&
		score.Trend != reputation.TrendDeclining {
		return cgp.TrustLevelFull
	}
	if score.SampleSize >= minSamples && score.Overall >= reputation.ThresholdTrusted {
		return cgp.TrustLevelTrusted
	}
	return cgp.TrustLevelUntrusted
}
