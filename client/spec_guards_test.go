package client

import (
	"os"
	"strings"
	"testing"
)

// TestSpecGuards pins deviations the SDK deliberately maintains against the
// platform file it syncs from.
//
// A spec sync is a file copy, and a copy can silently reintroduce a bug that
// was already fixed by hand. Each case below encodes one such correction so a
// future sync fails here instead of shipping.
func TestSpecGuards(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("gen/openapi.yaml")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	spec := string(raw)

	// The agentic policy route takes NO client id: it resolves the client from
	// the API key, and the platform's router registers exactly /agentic-policy.
	//
	// The platform's published openapi.public.yaml still describes the older
	// /clients/{client_id}/agentic-policy shape — it was not regenerated when
	// the route was simplified — and the TypeScript SDK shipped that stale path
	// in 2.2.0, so every agenticPolicy call 404'd. Until upstream regenerates,
	// this SDK carries the corrected path, and this guard keeps it corrected.
	if !strings.Contains(spec, "\n  /agentic-policy:") {
		t.Error("spec is missing /agentic-policy — did a sync overwrite the corrected route?")
	}
	if strings.Contains(spec, "/clients/{client_id}/agentic-policy") {
		t.Error("spec reintroduced /clients/{client_id}/agentic-policy — that route does not exist on the platform and always 404s")
	}

	// A {client_id} path parameter is the shape that caused the bug: its only
	// legal value is one the server already knows, and a caller has no way to
	// discover it. A new one appearing in a sync deserves a second look.
	if strings.Contains(spec, "{client_id}") {
		t.Error("spec has a {client_id}-scoped path; verify it against the platform's internal openapi.yaml before adopting it")
	}
}
