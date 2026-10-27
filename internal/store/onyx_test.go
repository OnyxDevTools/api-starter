// internal/store/onyx_test.go
package store

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sdk "github.com/OnyxDevTools/onyx-database-go/onyx"
)

func TestSDKWriteAndPaginationContracts(t *testing.T) {
	resource := "Task"
	var mode string
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		switch mode {
		case "insert":
			if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/"+resource) {
				t.Errorf("not insert: %s %s", r.Method, r.URL.Path)
			}
			if _, ok := body["body"]; !ok {
				t.Error("empty body omitted")
			}
			if completed, ok := body["completed"]; !ok || completed != false {
				t.Error("false completion omitted")
			}
			w.WriteHeader(409)
			_, _ = w.Write([]byte(`{"code":"CONFLICT","message":"private duplicate"}`))
		case "update":
			if !strings.Contains(r.URL.Path, "/query/update/") {
				t.Error(r.URL.Path)
			}
			if strings.Contains(string(mustJSON(t, body)), "createdAt") {
				t.Error("update changes creation time")
			}
			_, _ = w.Write([]byte(`0`))
		case "delete":
			if !strings.Contains(r.URL.Path, "/query/delete/") {
				t.Error(r.URL.Path)
			}
			_, _ = w.Write([]byte(`0`))
		case "page":
			if r.URL.Query().Get("pageSize") != "20" || r.URL.Query().Get("nextPage") != "opaque+/=" {
				t.Error(r.URL.String())
			}
			if !strings.Contains(string(mustJSON(t, body)), "id") {
				t.Error("missing deterministic order")
			}
			_, _ = w.Write([]byte(`{"records":[],"nextPage":"next"}`))
		case "outage":
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"message":"private auth failure"}`))
		case "empty":
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Error("unexpected request")
		}
	}))
	defer server.Close()
	sdk.ClearConfigCache()
	repo, err := NewOnyx(context.Background(), sdk.Config{DatabaseID: "offline-test", DatabaseBaseURL: server.URL, APIKey: "test-key", APISecret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	mode = "insert"
	_, err = repo.CreateTask(ctx, Task{ID: "id", Title: "test", Body: "", CreatedAt: now, UpdatedAt: now})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	mode = "update"
	_, err = repo.UpdateTask(ctx, "missing", "title", "", false, now)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update: %v", err)
	}
	mode = "delete"
	err = repo.DeleteTask(ctx, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete: %v", err)
	}
	mode = "page"
	page, e := repo.ListTasks(ctx, 20, "opaque+/=")
	err = e
	if page.NextCursor != "next" || page.Items == nil {
		t.Error("page envelope lost")
	}
	if err != nil {
		t.Fatal(err)
	}
	mode = "empty"
	_, err = repo.GetTask(ctx, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing read: %v", err)
	}
	mode = "outage"
	_, err = repo.GetTask(ctx, "missing")
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatal("authentication failure became missing record")
	}
	if requests != 6 {
		t.Fatalf("expected six bounded requests, got %d", requests)
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

func TestSDKContextCancellation(t *testing.T) {
	sdk.ClearConfigCache()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	repo, err := NewOnyx(context.Background(), sdk.Config{DatabaseID: "deadline", DatabaseBaseURL: server.URL, APIKey: "key", APISecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = repo.ListTasks(ctx, 20, "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("request lost cancellation: %v", err)
	}
}
