package governance

import (
	"context"
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/identity"
)

// Trust-score thresholds for mapping a registered identity's aggregate trust
// score to an effective trust level. An organization sets the score explicitly,
// so these bands express how much autonomy a given score is worth.
const (
	identityFullTrustScore    = 0.9
	identityTrustedTrustScore = 0.7
	identityLimitedTrustScore = 0.5
)

// applyIdentityTrust raises the governing actor's trust level to the one its
// organization registered for it. It only ever raises trust (never lowers what
// the actor already holds) and returns the grant info when an escalation
// occurred, or nil otherwise.
func (s *Service) applyIdentityTrust(ctx context.Context, proposal *cgp.ChangeProposal) *IdentityTrustInfo {
	if s.identityRegistry == nil {
		return nil
	}
	actor := proposal.Actor

	match := s.matchIdentity(ctx, actor)
	if match == nil {
		return nil
	}

	granted := trustScoreToLevel(match.TrustScore)
	if granted <= actor.TrustLevel {
		return nil
	}

	from := actor.TrustLevel
	proposal.Actor.TrustLevel = granted

	info := &IdentityTrustInfo{
		ActorID:    actor.ID,
		IdentityID: match.ID,
		FromLevel:  from.String(),
		ToLevel:    granted.String(),
		TrustScore: match.TrustScore,
	}

	if proposal.Context == nil {
		proposal.Context = &cgp.ProposalContext{}
	}
	if proposal.Context.Metadata == nil {
		proposal.Context.Metadata = make(map[string]any)
	}
	proposal.Context.Metadata["identity_trust.id"] = match.ID
	proposal.Context.Metadata["identity_trust.from"] = info.FromLevel
	proposal.Context.Metadata["identity_trust.to"] = info.ToLevel
	proposal.Context.Metadata["identity_trust.score"] = match.TrustScore

	s.logger.Info("trust granted from identity registry",
		"actor", actor.ID,
		"identity", match.ID,
		"from", info.FromLevel,
		"to", info.ToLevel,
		"score", match.TrustScore)

	return info
}

// matchIdentity finds the registered identity for an actor, returning the one
// that grants the highest trust when several match. Matching is by actor kind
// plus identity: a registry ID ("name@scope") matches when its local part (before
// "@") equals the actor's local name (after "kind:"), or when the full IDs/names
// are equal. Returns nil when no identity matches.
func (s *Service) matchIdentity(ctx context.Context, actor cgp.Actor) *identity.ActorIdentity {
	identities, err := s.identityRegistry.List(ctx)
	if err != nil {
		s.logger.Debug("identity trust skipped: could not list registry", "error", err)
		return nil
	}

	actorLocal := actorLocalName(actor.ID)
	var best *identity.ActorIdentity
	for _, id := range identities {
		if id == nil || id.Kind != actor.Kind {
			continue
		}
		if !identityMatchesActor(id, actor, actorLocal) {
			continue
		}
		if best == nil || id.TrustScore > best.TrustScore {
			best = id
		}
	}
	return best
}

// identityMatchesActor reports whether a registry identity refers to the actor.
func identityMatchesActor(id *identity.ActorIdentity, actor cgp.Actor, actorLocal string) bool {
	if id.ID == actor.ID {
		return true
	}
	local := identityLocalName(id.ID)
	if local != "" && (local == actorLocal || local == actor.Name) {
		return true
	}
	return false
}

// actorLocalName returns the actor identifier without its "kind:" prefix
// (e.g. "agent:claude-code" -> "claude-code"). Unprefixed IDs are returned as-is.
func actorLocalName(actorID string) string {
	if i := strings.Index(actorID, ":"); i >= 0 {
		return actorID[i+1:]
	}
	return actorID
}

// identityLocalName returns the registry identity's name without its "@scope"
// suffix (e.g. "claude-code@team-platform" -> "claude-code").
func identityLocalName(identityID string) string {
	if i := strings.Index(identityID, "@"); i >= 0 {
		return identityID[:i]
	}
	return identityID
}

// trustScoreToLevel maps an organization-registered trust score to a trust
// level. Below the limited band it grants nothing (Untrusted), so a low score
// never escalates an actor.
func trustScoreToLevel(score float64) cgp.TrustLevel {
	switch {
	case score >= identityFullTrustScore:
		return cgp.TrustLevelFull
	case score >= identityTrustedTrustScore:
		return cgp.TrustLevelTrusted
	case score >= identityLimitedTrustScore:
		return cgp.TrustLevelLimited
	default:
		return cgp.TrustLevelUntrusted
	}
}
