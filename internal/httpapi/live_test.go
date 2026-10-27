// internal/httpapi/live_test.go
package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/OnyxDevTools/api-starter/internal/store"
	"github.com/google/uuid"
)

// TestLiveCRUD is opt-in and uses only IDs created by this run. Offline tests
// never contact a database or the running API.
func TestLiveCRUD(t *testing.T) {
	if os.Getenv("LIVE_SMOKE_CONFIRM") != "development" {
		t.Skip("live check is explicitly opt-in")
	}
	if os.Getenv("ONYX_DATABASE_ID") == "" {
		t.Fatal("set ONYX_DATABASE_ID to the same development database as the running API")
	}
	origin := os.Getenv("API_URL")
	if origin == "" {
		origin = "http://127.0.0.1:8080"
	}
	client := &http.Client{Timeout: 15 * time.Second}
	request := func(t *testing.T, method, path string, body any, want int) []byte {
		t.Helper()
		var data []byte
		var err error
		if body != nil {
			data, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequestWithContext(t.Context(), method, strings.TrimRight(origin, "/")+path, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = response.Body.Close() }()
		data, err = io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: want %d, got %d", method, path, want, response.StatusCode)
		}
		return data
	}
	resource := "tasks"
	input := map[string]any{"title": "smoke-" + uuid.NewString(), "body": "Live check"}
	input["completed"] = false
	raw := request(t, "POST", "/api/v1/"+resource, input, 201)
	var first store.Task
	if err := json.Unmarshal(raw, &first); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(first.ID); err != nil {
		t.Fatal("server returned invalid ID")
	}
	path := "/api/v1/" + resource + "/" + first.ID
	deleted := false
	defer func() {
		if !deleted {
			request(t, "DELETE", path, nil, 204)
		}
	}()
	request(t, "GET", path, nil, 200)
	input["title"] = "Updated smoke"
	input["completed"] = true
	raw = request(t, "PUT", path, input, 200)
	var updated store.Task
	if err := json.Unmarshal(raw, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID != first.ID || !updated.CreatedAt.Equal(first.CreatedAt) || updated.Title != "Updated smoke" {
		t.Fatal("update did not preserve fields")
	}
	request(t, "GET", "/api/v1/"+resource+"?limit=1", nil, 200)
	request(t, "DELETE", path, nil, 204)
	deleted = true
	request(t, "GET", path, nil, 404)
}
