package middleware

import (
	"net/http"
	"strings"
)

const (
	// CurrentAPIVersion is the current API version supported by the server.
	CurrentAPIVersion = "v1"

	// apiVersionHeader is the response header that communicates the API version.
	apiVersionHeader = "X-API-Version"

	// vendorMediaTypePrefix is the prefix for vendor-specific media types.
	// Full format: application/vnd.relicta.v1+json
	vendorMediaTypePrefix = "application/vnd.relicta."

	// vendorMediaTypeSuffix is the suffix for vendor-specific media types.
	vendorMediaTypeSuffix = "+json"
)

// APIVersion returns middleware that performs API version negotiation.
//
// It inspects the Accept header for a vendor media type of the form
// "application/vnd.relicta.v1+json". If the header is absent or does not
// contain a vendor media type, the middleware defaults to the current version.
// If a vendor media type is present but requests an unsupported version,
// the middleware responds with 406 Not Acceptable.
//
// On all successful requests, the middleware sets the X-API-Version response
// header to the negotiated version.
func APIVersion() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			version := negotiateVersion(r)

			if version != "" && version != CurrentAPIVersion {
				http.Error(w, "Not Acceptable: unsupported API version", http.StatusNotAcceptable)
				return
			}

			w.Header().Set(apiVersionHeader, CurrentAPIVersion)
			next.ServeHTTP(w, r)
		})
	}
}

// negotiateVersion extracts the API version from the Accept header.
// It looks for a vendor media type matching "application/vnd.relicta.<version>+json".
// Returns empty string if no vendor media type is found (caller should default to current).
// Returns the version string (e.g. "v1") if a vendor media type is found.
func negotiateVersion(r *http.Request) string {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return ""
	}

	// The Accept header may contain multiple media types separated by commas.
	for _, mediaType := range strings.Split(accept, ",") {
		mediaType = strings.TrimSpace(mediaType)

		// Strip quality parameter if present (e.g. ";q=0.9")
		if idx := strings.Index(mediaType, ";"); idx != -1 {
			mediaType = strings.TrimSpace(mediaType[:idx])
		}

		if !strings.HasPrefix(mediaType, vendorMediaTypePrefix) {
			continue
		}

		// Extract version from "application/vnd.relicta.v1+json"
		rest := mediaType[len(vendorMediaTypePrefix):]
		if !strings.HasSuffix(rest, vendorMediaTypeSuffix) {
			continue
		}

		version := rest[:len(rest)-len(vendorMediaTypeSuffix)]
		if version != "" {
			return version
		}
	}

	return ""
}
