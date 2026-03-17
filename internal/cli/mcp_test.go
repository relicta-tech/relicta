package cli

import "testing"

func TestResolveMCPServeTransport(t *testing.T) {
	origTransport := mcpServeTransport
	origPort := mcpServePort
	origAddress := mcpServeAddress
	defer func() {
		mcpServeTransport = origTransport
		mcpServePort = origPort
		mcpServeAddress = origAddress
	}()

	tests := []struct {
		name      string
		transport string
		port      string
		address   string
		wantTrans string
		wantAddr  string
		wantErr   bool
	}{
		{name: "default stdio", wantTrans: "stdio"},
		{name: "http explicit", transport: "http", wantTrans: "http", wantAddr: ":8080"},
		{name: "http with port", transport: "http", port: "9090", wantTrans: "http", wantAddr: ":9090"},
		{name: "http with address", transport: "http", address: "127.0.0.1:8081", wantTrans: "http", wantAddr: "127.0.0.1:8081"},
		{name: "port implies http", port: "8080", wantTrans: "http", wantAddr: ":8080"},
		{name: "invalid transport", transport: "grpc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpServeTransport = tt.transport
			mcpServePort = tt.port
			mcpServeAddress = tt.address

			gotTransport, gotAddress, err := resolveMCPServeTransport()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotTransport != tt.wantTrans {
				t.Fatalf("transport = %q, want %q", gotTransport, tt.wantTrans)
			}
			if gotAddress != tt.wantAddr {
				t.Fatalf("address = %q, want %q", gotAddress, tt.wantAddr)
			}
		})
	}
}
