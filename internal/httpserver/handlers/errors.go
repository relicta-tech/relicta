package handlers

import (
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

// Standard error codes following the pattern "RESOURCE_ACTION" or "CATEGORY_REASON".
const (
	// Client errors
	ErrCodeBadRequest         = "BAD_REQUEST"
	ErrCodeInvalidJSON        = "INVALID_JSON"
	ErrCodeValidationFailed   = "VALIDATION_FAILED"
	ErrCodeMissingField       = "MISSING_FIELD"
	ErrCodeUnauthorized       = "UNAUTHORIZED"
	ErrCodeInvalidCredentials = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired       = "TOKEN_EXPIRED"
	ErrCodeTokenInvalid       = "TOKEN_INVALID"
	ErrCodeForbidden          = "FORBIDDEN"
	ErrCodeNotFound           = "NOT_FOUND"
	ErrCodeReleaseNotFound    = "RELEASE_NOT_FOUND"

	// Server errors
	ErrCodeInternal           = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	ErrCodeAuthNotConfigured  = "AUTH_NOT_CONFIGURED"
)

// writeError writes a structured error response with the request ID from chi middleware.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	resp := dto.ErrorResponse{
		Error:     message,
		Code:      code,
		RequestID: chimw.GetReqID(r.Context()),
	}
	if details != nil {
		resp.Details = details
	}
	respondJSON(w, status, resp)
}
