package mcp

import (
	"testing"

	cgpprotocol "github.com/relicta-tech/relicta/v4/internal/cgp/protocol"
)

func TestWithCGPService(t *testing.T) {
	svc := cgpprotocol.NewService(nil)
	opt := WithCGPService(svc)

	s := &Server{}
	opt(s)

	if s.cgpService != svc {
		t.Error("WithCGPService did not set the service")
	}
}

func TestWithAdapter(t *testing.T) {
	adapter := &Adapter{}
	opt := WithAdapter(adapter)

	s := &Server{}
	opt(s)

	if s.adapter != adapter {
		t.Error("WithAdapter did not set the adapter")
	}
}
