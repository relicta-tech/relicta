package governance

import (
	"strings"

	"github.com/relicta-tech/relicta/v4/internal/cgp"
	"github.com/relicta-tech/relicta/v4/internal/cgp/attribution"
	"github.com/relicta-tech/relicta/v4/internal/domain/changes"
)

// detectAuthorship runs the attribution detector over a changeset's commits and
// returns the dominant authoring actor. Returns nil when there are no commits.
func (s *Service) detectAuthorship(cs *changes.ChangeSet) *attribution.DetectionResult {
	if cs == nil {
		return nil
	}
	commits := cs.Commits()
	if len(commits) == 0 {
		return nil
	}

	infos := make([]*attribution.CommitInfo, 0, len(commits))
	for _, c := range commits {
		if c == nil {
			continue
		}
		infos = append(infos, &attribution.CommitInfo{
			Hash:        c.Hash(),
			AuthorName:  c.Author(),
			AuthorEmail: c.AuthorEmail(),
			Message:     c.RawMessage(),
			Trailers:    parseTrailers(c.Footer()),
		})
	}
	if len(infos) == 0 {
		return nil
	}
	return s.detector.DetectMultiple(infos)
}

// applyAttribution decides which actor governs the proposal given the initiator
// and the detected author. It only ever tightens governance: a machine author
// (agent or CI) detected behind a human-initiated release takes over so the
// change faces machine governance rules. A human author, or an initiator that is
// already a machine, leaves the initiator in place. Returns the governing actor
// and the confidence to record on the proposal intent.
func applyAttribution(initiator cgp.Actor, detection *attribution.DetectionResult) (cgp.Actor, float64) {
	if detection == nil {
		return initiator, 1.0
	}

	detectedMachine := detection.Actor.Kind == cgp.ActorKindAgent || detection.Actor.Kind == cgp.ActorKindCI
	initiatorMachine := initiator.Kind == cgp.ActorKindAgent || initiator.Kind == cgp.ActorKindCI

	// A human (or system) initiated a release whose commits a machine authored:
	// let the machine author govern so agent rules apply.
	if detectedMachine && !initiatorMachine {
		return detection.Actor, detection.Confidence
	}

	// Otherwise keep the initiator, but reflect detection confidence.
	return initiator, detection.Confidence
}

// recordAttributionContext stashes the detection result on the proposal context
// for audit, preserving the original initiator alongside the detected author.
func recordAttributionContext(proposal *cgp.ChangeProposal, initiator cgp.Actor, detection *attribution.DetectionResult) {
	if proposal == nil || detection == nil {
		return
	}
	if proposal.Context == nil {
		proposal.Context = &cgp.ProposalContext{}
	}
	if proposal.Context.Metadata == nil {
		proposal.Context.Metadata = make(map[string]any)
	}
	proposal.Context.Metadata["attribution.initiator"] = initiator.ID
	proposal.Context.Metadata["attribution.author"] = detection.Actor.ID
	proposal.Context.Metadata["attribution.author_kind"] = string(detection.Actor.Kind)
	proposal.Context.Metadata["attribution.confidence"] = detection.Confidence
	proposal.Context.Metadata["attribution.method"] = detection.Method
}

// parseTrailers extracts git trailers ("Key: value" lines) from a commit footer.
// Returns nil when the footer carries no trailers so callers can treat the
// absence uniformly.
func parseTrailers(footer string) map[string]string {
	footer = strings.TrimSpace(footer)
	if footer == "" {
		return nil
	}
	var trailers map[string]string
	for line := range strings.SplitSeq(footer, "\n") {
		line = strings.TrimSpace(line)
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if key == "" || value == "" || strings.ContainsAny(key, " \t") {
			// Trailer keys are single tokens; skip prose lines with colons.
			continue
		}
		if trailers == nil {
			trailers = make(map[string]string)
		}
		trailers[key] = value
	}
	return trailers
}
