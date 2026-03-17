package plugin

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

func TestGRPCPlugin_ServerAndClient(t *testing.T) {
	p := &GRPCPlugin{Impl: &mockPlugin{}}

	server := grpc.NewServer()
	if err := p.GRPCServer(&plugin.GRPCBroker{}, server); err != nil {
		t.Fatalf("GRPCServer error: %v", err)
	}

	client, err := p.GRPCClient(context.Background(), &plugin.GRPCBroker{}, new(grpc.ClientConn))
	if err != nil {
		t.Fatalf("GRPCClient error: %v", err)
	}
	if _, ok := client.(*GRPCClient); !ok {
		t.Fatalf("unexpected client type: %T", client)
	}
}

func TestServeTestAndIsPlugin(t *testing.T) {
	reattach, closeFn := ServeTest(&mockPlugin{})
	if reattach == nil {
		t.Fatal("expected reattach config")
	}
	closeFn()

	orig := os.Getenv(MagicCookieKey)
	defer os.Setenv(MagicCookieKey, orig)

	os.Setenv(MagicCookieKey, MagicCookieValue)
	if !IsPlugin() {
		t.Fatal("expected IsPlugin true when magic cookie set")
	}
	os.Unsetenv(MagicCookieKey)
	if IsPlugin() {
		t.Fatal("expected IsPlugin false when magic cookie missing")
	}
}
