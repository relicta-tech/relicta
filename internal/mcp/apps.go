package mcp

import (
	"context"
	"embed"

	"github.com/felixgeelhaar/mcp-go"
)

//go:embed all:dist
var distFS embed.FS

type appEntry struct {
	uri      string
	filePath string
}

var appEntries = []appEntry{
	{uri: "ui://relicta/status", filePath: "dist/status.html"},
	{uri: "ui://relicta/pipeline", filePath: "dist/pipeline.html"},
	{uri: "ui://relicta/risk", filePath: "dist/risk.html"},
	{uri: "ui://relicta/commits", filePath: "dist/commits.html"},
	{uri: "ui://relicta/approval", filePath: "dist/approval.html"},
	{uri: "ui://relicta/blast", filePath: "dist/blast.html"},
}

// registerApps registers MCP app resources for interactive UI rendering.
func (s *Server) registerApps() {
	for _, entry := range appEntries {
		fp := entry.filePath
		uri := entry.uri
		s.server.Resource(uri).
			Name(uri).
			Description("Relicta MCP App: " + uri).
			MimeType("text/html;profile=mcp-app").
			Handler(func(_ context.Context, resURI string, _ map[string]string) (*mcp.ResourceContent, error) {
				data, err := distFS.ReadFile(fp)
				if err != nil {
					return nil, err
				}
				return &mcp.ResourceContent{
					URI:      resURI,
					MimeType: "text/html;profile=mcp-app",
					Text:     string(data),
				}, nil
			})
	}
}
