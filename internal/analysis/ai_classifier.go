// Package analysis provides intelligent commit classification
// for repositories that don't follow conventional commit standards.
package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/relicta-tech/relicta/internal/domain/changes"
	"github.com/relicta-tech/relicta/internal/infrastructure/ai"
)

const (
	maxDiffChars   = 4000
	maxReasonChars = 240
)

// AIClassifierImpl uses AI to classify commits.
type AIClassifierImpl struct {
	service ai.Service
}

// NewAIClassifier creates a new AI classifier.
func NewAIClassifier(service ai.Service) *AIClassifierImpl {
	return &AIClassifierImpl{service: service}
}

// Classify uses AI to classify a commit.
func (c *AIClassifierImpl) Classify(ctx context.Context, commit CommitInfo) (*CommitClassification, error) {
	if c.service == nil || !c.service.IsAvailable() {
		return nil, errors.New("ai service not available")
	}

	systemPrompt := `You are a commit classification engine.
Return JSON only with fields:
{"type":"feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert|",
 "scope":"string",
 "confidence":0.0,
 "reasoning":"string",
 "is_breaking":true|false,
 "breaking_reason":"string",
 "should_skip":true|false,
 "skip_reason":"string"}
Use empty strings if unknown. Confidence must be between 0 and 1.`

	userPrompt := buildUserPrompt(commit)

	response, err := c.service.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	parsed, err := parseAIResponse(response)
	if err != nil {
		return nil, err
	}

	commitType, _ := changes.ParseCommitType(parsed.Type)
	confidence := clamp(parsed.Confidence, 0, 1)

	reason := strings.TrimSpace(parsed.Reasoning)
	if len(reason) > maxReasonChars {
		reason = reason[:maxReasonChars] + "..."
	}

	result := &CommitClassification{
		CommitHash:     commit.Hash,
		Type:           commitType,
		Scope:          strings.TrimSpace(parsed.Scope),
		Confidence:     confidence,
		Method:         MethodAI,
		Reasoning:      reason,
		IsBreaking:     parsed.IsBreaking,
		BreakingReason: strings.TrimSpace(parsed.BreakingReason),
		ShouldSkip:     parsed.ShouldSkip,
		SkipReason:     strings.TrimSpace(parsed.SkipReason),
	}

	if result.ShouldSkip {
		result.Method = MethodSkipped
		if result.SkipReason == "" {
			result.SkipReason = "ai recommended skip"
		}
	}

	return result, nil
}

type aiResponse struct {
	Type           string  `json:"type"`
	Scope          string  `json:"scope"`
	Confidence     float64 `json:"confidence"`
	Reasoning      string  `json:"reasoning"`
	IsBreaking     bool    `json:"is_breaking"`
	BreakingReason string  `json:"breaking_reason"`
	ShouldSkip     bool    `json:"should_skip"`
	SkipReason     string  `json:"skip_reason"`
}

func buildUserPrompt(commit CommitInfo) string {
	var b strings.Builder
	b.WriteString("Classify this commit.\n\n")
	b.WriteString(fmt.Sprintf("Hash: %s\n", commit.Hash.Short()))
	b.WriteString(fmt.Sprintf("Subject: %s\n", commit.Subject))
	if commit.Message != "" && commit.Message != commit.Subject {
		b.WriteString("Message:\n")
		b.WriteString(commit.Message)
		b.WriteString("\n")
	}

	if len(commit.Files) > 0 {
		b.WriteString("Files:\n")
		for _, file := range commit.Files {
			b.WriteString("- ")
			b.WriteString(file)
			b.WriteString("\n")
		}
	}

	if commit.Stats.FilesChanged > 0 {
		b.WriteString(fmt.Sprintf("Stats: %d files, +%d/-%d lines\n",
			commit.Stats.FilesChanged,
			commit.Stats.Additions,
			commit.Stats.Deletions))
	}

	if commit.Diff != "" {
		diff := commit.Diff
		if len(diff) > maxDiffChars {
			diff = diff[:maxDiffChars] + "\n..."
		}
		b.WriteString("Diff:\n")
		b.WriteString(diff)
		b.WriteString("\n")
	}

	b.WriteString("\nReturn JSON only.")
	return b.String()
}

func parseAIResponse(response string) (*aiResponse, error) {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return nil, errors.New("empty ai response")
	}

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, errors.New("no json object found in ai response")
	}

	payload := trimmed[start : end+1]
	var parsed aiResponse
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return nil, err
	}

	return &parsed, nil
}

func clamp(val, minVal, maxVal float64) float64 {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
