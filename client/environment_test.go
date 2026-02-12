package client_test

import (
	"testing"

	"github.com/dakota-xyz/go-sdk/client"
)

func TestEnvironmentBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		env     client.Environment
		wantURL string
		wantErr bool
	}{
		{name: "default empty uses sandbox", env: "", wantURL: "https://api.platform.sandbox.dakota.xyz"},
		{name: "sandbox", env: client.EnvironmentSandbox, wantURL: "https://api.platform.sandbox.dakota.xyz"},
		{name: "production", env: client.EnvironmentProduction, wantURL: "https://api.platform.dakota.xyz"},
		{name: "development", env: client.EnvironmentDevelopment, wantURL: "https://api.platform.dev.dakota.xyz"},
		{name: "local", env: client.EnvironmentLocal, wantURL: "http://localhost:6464"},
		{name: "invalid", env: "staging", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotURL, err := tt.env.BaseURL()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotURL != tt.wantURL {
				t.Fatalf("got %q, want %q", gotURL, tt.wantURL)
			}
		})
	}
}
