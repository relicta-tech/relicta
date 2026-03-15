package analytics

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Granularity defines the time bucket size for aggregation.
type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

// ParseGranularity parses a granularity string, defaulting to day.
func ParseGranularity(s string) Granularity {
	switch s {
	case "week":
		return GranularityWeek
	case "month":
		return GranularityMonth
	default:
		return GranularityDay
	}
}

// TimeBucketKey returns the time bucket key for the given timestamp and granularity.
func TimeBucketKey(t time.Time, g Granularity) string {
	t = t.UTC()
	switch g {
	case GranularityWeek:
		// ISO week: use Monday of the week
		year, week := t.ISOWeek()
		// Find Monday of this ISO week
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		monday := jan4.AddDate(0, 0, -(int(jan4.Weekday())+6)%7)
		weekStart := monday.AddDate(0, 0, (week-1)*7)
		return weekStart.Format("2006-01-02")
	case GranularityMonth:
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// RiskTrendPoint represents an aggregated risk data point.
type RiskTrendPoint struct {
	Bucket       string  `json:"bucket"`
	AvgRiskScore float64 `json:"avg_risk_score"`
	MaxRiskScore float64 `json:"max_risk_score"`
	MinRiskScore float64 `json:"min_risk_score"`
	Count        int     `json:"count"`
}

// DecisionDistribution represents counts of policy decisions.
type DecisionDistribution struct {
	Bucket        string `json:"bucket"`
	Approve       int    `json:"approve"`
	Deny          int    `json:"deny"`
	RequireReview int    `json:"require_review"`
	Total         int    `json:"total"`
}

// TeamMetrics represents per-actor analytics.
type TeamMetrics struct {
	ActorID        string  `json:"actor_id"`
	ApprovalCount  int     `json:"approval_count"`
	RejectionCount int     `json:"rejection_count"`
	ReleaseCount   int     `json:"release_count"`
	AvgDurationMs  float64 `json:"avg_duration_ms"`
	SuccessRate    float64 `json:"success_rate"`
}

// AggregateRiskTrends aggregates risk evaluation events into time-bucketed trend points.
func AggregateRiskTrends(events []Event, granularity Granularity) []RiskTrendPoint {
	type accumulator struct {
		totalScore float64
		maxScore   float64
		minScore   float64
		count      int
	}

	buckets := make(map[string]*accumulator)
	var order []string

	for _, e := range events {
		if e.Type != EventRiskEvaluation {
			continue
		}

		var payload RiskEvaluationPayload
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			continue
		}

		key := TimeBucketKey(e.Timestamp, granularity)
		acc, exists := buckets[key]
		if !exists {
			acc = &accumulator{minScore: payload.RiskScore}
			buckets[key] = acc
			order = append(order, key)
		}

		acc.totalScore += payload.RiskScore
		acc.count++
		if payload.RiskScore > acc.maxScore {
			acc.maxScore = payload.RiskScore
		}
		if payload.RiskScore < acc.minScore {
			acc.minScore = payload.RiskScore
		}
	}

	points := make([]RiskTrendPoint, 0, len(order))
	for _, key := range order {
		acc := buckets[key]
		points = append(points, RiskTrendPoint{
			Bucket:       key,
			AvgRiskScore: acc.totalScore / float64(acc.count),
			MaxRiskScore: acc.maxScore,
			MinRiskScore: acc.minScore,
			Count:        acc.count,
		})
	}

	return points
}

// AggregateDecisions aggregates policy decision events into time-bucketed distributions.
func AggregateDecisions(events []Event, granularity Granularity) []DecisionDistribution {
	buckets := make(map[string]*DecisionDistribution)
	var order []string

	for _, e := range events {
		if e.Type != EventPolicyDecision {
			continue
		}

		var payload PolicyDecisionPayload
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			continue
		}

		key := TimeBucketKey(e.Timestamp, granularity)
		dist, exists := buckets[key]
		if !exists {
			dist = &DecisionDistribution{Bucket: key}
			buckets[key] = dist
			order = append(order, key)
		}

		dist.Total++
		switch payload.Decision {
		case "approve":
			dist.Approve++
		case "deny":
			dist.Deny++
		case "require_review":
			dist.RequireReview++
		}
	}

	result := make([]DecisionDistribution, 0, len(order))
	for _, key := range order {
		result = append(result, *buckets[key])
	}

	return result
}

// AggregateTeamMetrics aggregates approval and release events per actor.
func AggregateTeamMetrics(events []Event) []TeamMetrics {
	type accumulator struct {
		approvals    int
		rejections   int
		releases     int
		successCount int
		totalDurMs   int64
		durCount     int
	}

	actors := make(map[string]*accumulator)
	var order []string

	for _, e := range events {
		switch e.Type {
		case EventApprovalOutcome:
			var payload ApprovalOutcomePayload
			if err := json.Unmarshal(e.Payload, &payload); err != nil {
				continue
			}
			actorID := payload.ActorID
			if actorID == "" {
				actorID = "unknown"
			}
			acc, exists := actors[actorID]
			if !exists {
				acc = &accumulator{}
				actors[actorID] = acc
				order = append(order, actorID)
			}
			switch payload.Outcome {
			case "approved":
				acc.approvals++
			case "rejected":
				acc.rejections++
			}

		case EventReleaseDuration:
			var payload ReleaseDurationPayload
			if err := json.Unmarshal(e.Payload, &payload); err != nil {
				continue
			}
			// Attribute releases to "system" since ReleaseDuration doesn't have actor
			actorID := "system"
			acc, exists := actors[actorID]
			if !exists {
				acc = &accumulator{}
				actors[actorID] = acc
				order = append(order, actorID)
			}
			acc.releases++
			if payload.Success {
				acc.successCount++
			}
			acc.totalDurMs += payload.DurationMs
			acc.durCount++
		}
	}

	metrics := make([]TeamMetrics, 0, len(order))
	for _, actorID := range order {
		acc := actors[actorID]
		var avgDur float64
		if acc.durCount > 0 {
			avgDur = float64(acc.totalDurMs) / float64(acc.durCount)
		}
		var successRate float64
		if acc.releases > 0 {
			successRate = float64(acc.successCount) / float64(acc.releases)
		}
		metrics = append(metrics, TeamMetrics{
			ActorID:        actorID,
			ApprovalCount:  acc.approvals,
			RejectionCount: acc.rejections,
			ReleaseCount:   acc.releases,
			AvgDurationMs:  avgDur,
			SuccessRate:    successRate,
		})
	}

	return metrics
}

// CachedAggregator wraps aggregation functions with in-memory TTL caching.
type CachedAggregator struct {
	service  *Service
	cacheTTL time.Duration
	mu       sync.RWMutex
	cache    map[string]cachedEntry
}

type cachedEntry struct {
	data      any
	expiresAt time.Time
}

// NewCachedAggregator creates a new cached aggregator with the given TTL.
func NewCachedAggregator(service *Service, cacheTTL time.Duration) *CachedAggregator {
	return &CachedAggregator{
		service:  service,
		cacheTTL: cacheTTL,
		cache:    make(map[string]cachedEntry),
	}
}

// RiskTrends returns aggregated risk trends, using cache when available.
func (ca *CachedAggregator) RiskTrends(ctx context.Context, filter QueryFilter, granularity Granularity) ([]RiskTrendPoint, error) {
	cacheKey := "risk:" + granularity.cacheKey(filter)

	if cached, ok := ca.getCached(cacheKey); ok {
		if points, ok := cached.([]RiskTrendPoint); ok {
			return points, nil
		}
	}

	riskType := EventRiskEvaluation
	filter.EventType = &riskType
	events, err := ca.service.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	points := AggregateRiskTrends(events, granularity)
	ca.setCache(cacheKey, points)
	return points, nil
}

// Decisions returns aggregated decision distributions, using cache when available.
func (ca *CachedAggregator) Decisions(ctx context.Context, filter QueryFilter, granularity Granularity) ([]DecisionDistribution, error) {
	cacheKey := "decisions:" + granularity.cacheKey(filter)

	if cached, ok := ca.getCached(cacheKey); ok {
		if dist, ok := cached.([]DecisionDistribution); ok {
			return dist, nil
		}
	}

	policyType := EventPolicyDecision
	filter.EventType = &policyType
	events, err := ca.service.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	dist := AggregateDecisions(events, granularity)
	ca.setCache(cacheKey, dist)
	return dist, nil
}

// Team returns aggregated team metrics, using cache when available.
func (ca *CachedAggregator) Team(ctx context.Context, filter QueryFilter) ([]TeamMetrics, error) {
	cacheKey := "team:" + GranularityDay.cacheKeySuffix(filter)

	if cached, ok := ca.getCached(cacheKey); ok {
		if metrics, ok := cached.([]TeamMetrics); ok {
			return metrics, nil
		}
	}

	// Query both approval and release events
	filter.EventType = nil // query all types
	events, err := ca.service.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	metrics := AggregateTeamMetrics(events)
	ca.setCache(cacheKey, metrics)
	return metrics, nil
}

func (ca *CachedAggregator) getCached(key string) (any, bool) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	entry, ok := ca.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

func (ca *CachedAggregator) setCache(key string, data any) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	ca.cache[key] = cachedEntry{
		data:      data,
		expiresAt: time.Now().Add(ca.cacheTTL),
	}
}

// cacheKey builds a cache key suffix from the granularity and filter.
func (g Granularity) cacheKey(filter QueryFilter) string {
	return string(g) + ":" + g.cacheKeySuffix(filter)
}

func (g Granularity) cacheKeySuffix(filter QueryFilter) string {
	key := ""
	if filter.From != nil {
		key += filter.From.Format(time.RFC3339)
	}
	key += ":"
	if filter.To != nil {
		key += filter.To.Format(time.RFC3339)
	}
	return key
}
