package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
)

// The client creates a named key with a preset, renames it, and changes its
// preset (ENG-4792).
func TestAPIKeys_CreateNamedKeyWithPresetThenRenameAndChangePreset(t *testing.T) {
	type call struct {
		method, path string
		body         map[string]any
	}
	var calls []call
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls = append(calls, call{r.Method, r.URL.Path, body})
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"2jFh8KzLm4NpQ6RsTvWx9Ab","key":"dk_live_abc","name":"payouts","preset":"operate"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"2jFh8KzLm4NpQ6RsTvWx9Ab","user_id":"2kGi9LaXn5OrR7StUwYy0Bc",` +
			`"key_suffix":"abc123","created_at":1,"name":"payouts-v2","preset":"read_only","last_used_at":null}`))
	}))
	defer ts.Close()

	c, err := client.New(client.WithBaseURL(ts.URL), client.WithAPIKey("test_key"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	name, operate := "payouts", gen.ApiKeyPresetOperate
	created, err := client.CheckResponse(c.Raw().CreateApiKeyWithResponse(ctx, nil,
		gen.CreateApiKeyJSONRequestBody{Name: &name, Preset: &operate}))
	if err != nil {
		t.Fatal(err)
	}
	if created.JSON201.Name != "payouts" || created.JSON201.Preset != gen.ApiKeyPresetOperate {
		t.Fatalf("created = %+v", created.JSON201)
	}

	newName := "payouts-v2"
	if _, err := client.CheckResponse(c.Raw().UpdateApiKeyWithResponse(ctx, created.JSON201.Id, nil,
		gen.UpdateApiKeyJSONRequestBody{Name: &newName})); err != nil {
		t.Fatal(err)
	}
	readOnly := gen.ApiKeyPresetReadOnly
	updated, err := client.CheckResponse(c.Raw().UpdateApiKeyWithResponse(ctx, created.JSON201.Id, nil,
		gen.UpdateApiKeyJSONRequestBody{Preset: &readOnly}))
	if err != nil {
		t.Fatal(err)
	}
	if updated.JSON200.Preset != gen.ApiKeyPresetReadOnly || updated.JSON200.LastUsedAt != nil {
		t.Fatalf("updated = %+v", updated.JSON200)
	}

	want := []call{
		{http.MethodPost, "/api-keys", map[string]any{"name": "payouts", "preset": "operate"}},
		{http.MethodPatch, "/api-keys/2jFh8KzLm4NpQ6RsTvWx9Ab", map[string]any{"name": "payouts-v2"}},
		{http.MethodPatch, "/api-keys/2jFh8KzLm4NpQ6RsTvWx9Ab", map[string]any{"preset": "read_only"}},
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %+v", calls)
	}
	for i := range want {
		if calls[i].method != want[i].method || calls[i].path != want[i].path ||
			string(mustJSON(t, calls[i].body)) != string(mustJSON(t, want[i].body)) {
			t.Errorf("call %d = %+v, want %+v", i, calls[i], want[i])
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
