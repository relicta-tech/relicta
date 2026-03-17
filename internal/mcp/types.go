// Package mcp provides MCP server implementation for Relicta.
package mcp

// ResourceContent represents the content of a resource (for cache compatibility).
type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"`
}

// ReadResourceResult contains the contents of a resource (for cache compatibility).
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}
