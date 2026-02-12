package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAPIVersion_ResponseHeader tests that the X-API-Version header is set on all responses.
func TestAPIVersion_ResponseHeader(t *testing.T) {
	handler := APIVersion()(testHandler())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "v1", rec.Header().Get("X-API-Version"))
}

// TestAPIVersion_Negotiation tests version negotiation via the Accept header.
func TestAPIVersion_Negotiation(t *testing.T) {
	tests := []struct {
		name       string
		accept     string
		expectCode int
		expectBody string
	}{
		{
			name:       "no Accept header defaults to v1",
			accept:     "",
			expectCode: http.StatusOK,
		},
		{
			name:       "standard JSON Accept defaults to v1",
			accept:     "application/json",
			expectCode: http.StatusOK,
		},
		{
			name:       "wildcard Accept defaults to v1",
			accept:     "*/*",
			expectCode: http.StatusOK,
		},
		{
			name:       "vendor media type v1 accepted",
			accept:     "application/vnd.relicta.v1+json",
			expectCode: http.StatusOK,
		},
		{
			name:       "vendor media type v2 rejected",
			accept:     "application/vnd.relicta.v2+json",
			expectCode: http.StatusNotAcceptable,
			expectBody: "Not Acceptable: unsupported API version\n",
		},
		{
			name:       "vendor media type v99 rejected",
			accept:     "application/vnd.relicta.v99+json",
			expectCode: http.StatusNotAcceptable,
			expectBody: "Not Acceptable: unsupported API version\n",
		},
		{
			name:       "mixed Accept with valid vendor type",
			accept:     "text/html, application/vnd.relicta.v1+json, application/json",
			expectCode: http.StatusOK,
		},
		{
			name:       "mixed Accept with invalid vendor type",
			accept:     "text/html, application/vnd.relicta.v3+json, application/json",
			expectCode: http.StatusNotAcceptable,
			expectBody: "Not Acceptable: unsupported API version\n",
		},
		{
			name:       "vendor type with quality parameter",
			accept:     "application/vnd.relicta.v1+json;q=0.9",
			expectCode: http.StatusOK,
		},
		{
			name:       "unsupported version with quality parameter",
			accept:     "application/vnd.relicta.v2+json;q=0.8",
			expectCode: http.StatusNotAcceptable,
			expectBody: "Not Acceptable: unsupported API version\n",
		},
		{
			name:       "vendor type with spaces around comma",
			accept:     "application/json , application/vnd.relicta.v1+json",
			expectCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := APIVersion()(testHandler())

			req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectCode, rec.Code)

			if tt.expectCode == http.StatusOK {
				assert.Equal(t, "v1", rec.Header().Get("X-API-Version"))
			}

			if tt.expectBody != "" {
				assert.Equal(t, tt.expectBody, rec.Body.String())
			}
		})
	}
}

// TestAPIVersion_MalformedVendorType tests that malformed vendor types are ignored.
func TestAPIVersion_MalformedVendorType(t *testing.T) {
	tests := []struct {
		name   string
		accept string
	}{
		{
			name:   "missing json suffix",
			accept: "application/vnd.relicta.v1",
		},
		{
			name:   "wrong suffix",
			accept: "application/vnd.relicta.v1+xml",
		},
		{
			name:   "empty version",
			accept: "application/vnd.relicta.+json",
		},
		{
			name:   "partial prefix",
			accept: "application/vnd.relicta",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := APIVersion()(testHandler())

			req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
			req.Header.Set("Accept", tt.accept)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			// Malformed vendor types are ignored; defaults to v1
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "v1", rec.Header().Get("X-API-Version"))
		})
	}
}

// TestNegotiateVersion tests the internal negotiateVersion function directly.
func TestNegotiateVersion(t *testing.T) {
	tests := []struct {
		name    string
		accept  string
		version string
	}{
		{
			name:    "empty header",
			accept:  "",
			version: "",
		},
		{
			name:    "plain JSON",
			accept:  "application/json",
			version: "",
		},
		{
			name:    "valid v1",
			accept:  "application/vnd.relicta.v1+json",
			version: "v1",
		},
		{
			name:    "valid v2",
			accept:  "application/vnd.relicta.v2+json",
			version: "v2",
		},
		{
			name:    "first vendor type wins",
			accept:  "application/vnd.relicta.v1+json, application/vnd.relicta.v2+json",
			version: "v1",
		},
		{
			name:    "quality parameter stripped",
			accept:  "application/vnd.relicta.v1+json;q=0.5",
			version: "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}

			got := negotiateVersion(req)
			assert.Equal(t, tt.version, got)
		})
	}
}
