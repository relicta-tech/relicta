package handlers

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/relicta-tech/relicta/internal/domain/release/domain"
	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

// ListGovernanceDecisions returns governance decision history.
func ListGovernanceDecisions(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.GovernanceDecisionDTO]{
			Data:       []dto.GovernanceDecisionDTO{},
			Total:      0,
			Page:       1,
			PageSize:   20,
			TotalPages: 0,
		})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// List all run IDs to extract governance decisions
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// Load runs and extract governance decisions
	var decisions []dto.GovernanceDecisionDTO
	for _, runID := range runIDs {
		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue
		}

		// Create a governance decision from the release data
		decision := dto.GovernanceDecisionDTO{
			ID:        string(run.ID()) + "-decision",
			ReleaseID: string(run.ID()),
			RiskScore: run.RiskScore(),
			RiskLevel: getRiskLevel(run.RiskScore()),
			Factors:   run.Reasons(),
			Timestamp: run.CreatedAt(),
			ActorID:   run.ActorID(),
			ActorKind: string(run.ActorType()),
		}

		// Determine decision based on state and approval
		switch {
		case run.IsApproved():
			decision.Decision = "approve"
			if approval := run.Approval(); approval != nil {
				decision.RequiresReview = !approval.AutoApproved
				if approval.Justification != "" {
					decision.ReviewReason = approval.Justification
				}
			}
		case run.State() == domain.StateFailed || run.State() == domain.StateCanceled:
			decision.Decision = "deny"
			decision.ReviewReason = run.LastError()
		case run.RequiresApproval():
			decision.Decision = "require_review"
			decision.RequiresReview = true
			decision.ReviewReason = "Risk score exceeds auto-approve threshold"
		default:
			decision.Decision = "pending"
		}

		decisions = append(decisions, decision)
	}

	total := len(decisions)
	totalPages := (total + pageSize - 1) / pageSize

	// Apply pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	respondJSON(w, http.StatusOK, dto.PaginatedResponse[dto.GovernanceDecisionDTO]{
		Data:       decisions[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	})
}

// GetRiskTrends returns risk score trends over time.
func GetRiskTrends(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, map[string]any{"trends": []dto.RiskTrendDTO{}})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// Get time range from query params
	daysStr := r.URL.Query().Get("days")
	days := 30
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 365 {
		days = d
	}

	since := time.Now().AddDate(0, 0, -days)

	// List all runs
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Group by date and calculate daily averages
	dailyStats := make(map[string]struct {
		totalScore float64
		count      int
	})

	for _, runID := range runIDs {
		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue
		}

		if run.CreatedAt().Before(since) {
			continue
		}

		dateKey := run.CreatedAt().Format("2006-01-02")
		stats := dailyStats[dateKey]
		stats.totalScore += run.RiskScore()
		stats.count++
		dailyStats[dateKey] = stats
	}

	// Convert to trends
	var trends []dto.RiskTrendDTO
	for dateStr, stats := range dailyStats {
		date, _ := time.Parse("2006-01-02", dateStr)
		trends = append(trends, dto.RiskTrendDTO{
			Date:      date,
			RiskScore: stats.totalScore / float64(stats.count),
			Releases:  stats.count,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{"trends": trends})
}

// GetFactorDistribution returns the distribution of risk factors.
func GetFactorDistribution(w http.ResponseWriter, r *http.Request) {
	ctx := GetContext()
	if ctx == nil || ctx.ReleaseServices == nil {
		respondJSON(w, http.StatusOK, map[string]any{"factors": []dto.FactorDistributionDTO{}})
		return
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get working directory", "")
		return
	}

	// List all runs
	runIDs, err := ctx.ReleaseServices.Repository.List(r.Context(), repoRoot)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list releases", err.Error())
		return
	}

	// Count factor occurrences
	factorCounts := make(map[string]int)
	totalFactors := 0

	for _, runID := range runIDs {
		run, err := ctx.ReleaseServices.Repository.Load(r.Context(), runID)
		if err != nil {
			continue
		}

		for _, reason := range run.Reasons() {
			factorCounts[reason]++
			totalFactors++
		}
	}

	// Convert to distribution
	var distribution []dto.FactorDistributionDTO
	for factor, count := range factorCounts {
		percentage := 0.0
		if totalFactors > 0 {
			percentage = float64(count) / float64(totalFactors) * 100
		}
		distribution = append(distribution, dto.FactorDistributionDTO{
			Factor:     factor,
			Count:      count,
			Percentage: percentage,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{"factors": distribution})
}
