package rotation

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestUniqueDeploymentsControlsRevocation(t *testing.T) {
	tests := []struct {
		name   string
		logs   []LogEntry
		want   []string
		revoke bool
	}{
		{name: "all deployments moved", want: []string{}, revoke: true},
		{name: "old value remains", logs: []LogEntry{{Deployment: "worker-eu"}}, want: []string{"worker-eu"}},
		{name: "duplicate sightings", logs: []LogEntry{{Deployment: "api-us"}, {Deployment: "api-us"}, {Deployment: "jobs-us"}}, want: []string{"api-us", "jobs-us"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueDeployments(tt.logs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("deployments = %v, want %v", got, tt.want)
			}
			if safe := len(got) == 0; safe != tt.revoke {
				t.Fatalf("safe to revoke = %v, want %v", safe, tt.revoke)
			}
		})
	}
}

func TestRotateTenantKeyRequestBoundary(t *testing.T) {
	step := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step++
		if r.Header.Get("Authorization") != "Bearer operator-key" {
			t.Fatal("missing bearer authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		switch step {
		case 1:
			if r.Method != http.MethodPost || r.URL.Path != "/v1/account/keys/create" {
				t.Fatalf("create request = %s %s", r.Method, r.URL.Path)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["project_id"] != "tenant-42" || body["idempotency_key"] != "change-17-create" {
				t.Fatalf("create body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true,"data":{"key_id":"key-9","key_secret":"first-value"},"error":null,"metadata":{}}`))
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/v1/account/keys/rotate/key-9" {
				t.Fatalf("rotate request = %s %s", r.Method, r.URL.Path)
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["grace_hours"] != float64(24) || len(body) != 2 {
				t.Fatalf("rotate body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"ok":true,"data":{"id":"key-9","key":"second-value"},"error":null,"metadata":{}}`))
		case 3:
			if r.Method != http.MethodGet || r.URL.Path != "/v1/logs/search" || r.URL.Query().Get("q") != "key_id:key-9" {
				t.Fatalf("search request = %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"ok":true,"data":{"entries":[{"deployment":"billing-worker"}]},"error":null,"metadata":{}}`))
		default:
			t.Fatalf("unexpected request %d", step)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "operator-key", server.Client())
	result, err := client.RotateTenantKey(context.Background(), RotationInput{
		TenantID: "tenant-42", Name: "billing", Scopes: []string{"logs:read"}, GraceHours: 24, IdempotencyID: "change-17",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.SafeToRevokeOldKey || !reflect.DeepEqual(result.OldKeyDeployments, []string{"billing-worker"}) {
		t.Fatalf("result = %#v", result)
	}
}
