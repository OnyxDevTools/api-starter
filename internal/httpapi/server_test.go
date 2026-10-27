// internal/httpapi/server_test.go
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OnyxDevTools/api-starter/internal/store"
)

type memory struct {
	mu      sync.Mutex
	tasks   map[string]store.Task
	failure error
	writes  int
	limit   int
	cursor  string
}

func newMemory() *memory {
	return &memory{tasks: map[string]store.Task{}}
}
func (m *memory) Ready(ctx context.Context) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing readiness deadline")
	}
	return m.failure
}

func (m *memory) CreateTask(_ context.Context, v store.Task) (store.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failure != nil {
		return v, m.failure
	}
	m.writes++
	if _, ok := m.tasks[v.ID]; ok {
		return v, store.ErrConflict
	}
	m.tasks[v.ID] = v
	return v, nil
}
func (m *memory) GetTask(_ context.Context, id string) (store.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failure != nil {
		return store.Task{}, m.failure
	}
	v, ok := m.tasks[id]
	if !ok {
		return v, store.ErrNotFound
	}
	return v, nil
}
func (m *memory) ListTasks(_ context.Context, limit int, cursor string) (store.Page, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.limit = limit
	m.cursor = cursor
	if m.failure != nil {
		return store.Page{}, m.failure
	}
	keys := make([]string, 0, len(m.tasks))
	for key := range m.tasks {
		if key > cursor {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	page := store.Page{Items: []store.Task{}}
	for _, key := range keys {
		if len(page.Items) == limit {
			page.NextCursor = page.Items[len(page.Items)-1].ID
			break
		}
		page.Items = append(page.Items, m.tasks[key])
	}
	return page, nil
}
func (m *memory) UpdateTask(_ context.Context, id, title, body string, completed bool, now time.Time) (store.Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failure != nil {
		return store.Task{}, m.failure
	}
	v, ok := m.tasks[id]
	if !ok {
		return v, store.ErrNotFound
	}
	m.writes++
	v.Title = title
	v.Body = body
	v.UpdatedAt = now
	v.Completed = completed
	m.tasks[id] = v
	return v, nil
}
func (m *memory) DeleteTask(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failure != nil {
		return m.failure
	}
	if _, ok := m.tasks[id]; !ok {
		return store.ErrNotFound
	}
	m.writes++
	delete(m.tasks, id)
	return nil
}

func handler(m *memory) http.Handler {
	h, _ := New(m, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	return h
}
func call(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("want %d, got %d: %s", want, w.Code, w.Body.String())
	}
}
func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestCRUD(t *testing.T) {
	resource := "tasks"
	m := newMemory()
	h := handler(m)
	path := "/api/v1/" + resource
	payload := `{"title":"  First  ","body":"","completed":false}`
	w := call(h, "POST", path, payload)
	status(t, w, 201)
	created := decode[store.Task](t, w)
	if created.Title != "First" || created.ID == "" || created.CreatedAt.IsZero() || created.UpdatedAt.Location() != time.UTC {
		t.Fatalf("bad server fields: %+v", created)
	}
	location := w.Header().Get("Location")
	if location != path+"/"+created.ID {
		t.Fatal(location)
	}
	w = call(h, "GET", location, "")
	status(t, w, 200)
	payload = `{"title":"Changed","body":"New","completed":true}`
	w = call(h, "PUT", location, payload)
	status(t, w, 200)
	updated := decode[store.Task](t, w)
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) || updated.Title != "Changed" || updated.UpdatedAt.Before(created.UpdatedAt) {
		t.Fatalf("update lost identity: %+v", updated)
	}
	if !updated.Completed {
		t.Fatal("completion lost")
	}
	status(t, call(h, "GET", path, ""), 200)
	status(t, call(h, "DELETE", location, ""), 204)
	status(t, call(h, "GET", location, ""), 404)
	status(t, call(h, "PUT", location, payload), 404)
	status(t, call(h, "DELETE", location, ""), 404)
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
}
func TestValidationBeforeWrites(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"title":" ","body":"","completed":false}`,
		`{"title":"ok","completed":false}`,
		`{"title":"ok","body":""}`,
		`{"title":"ok","body":"","completed":false,"id":"client"}`,
		`{"title":"` + strings.Repeat("a", 121) + `","body":"","completed":false}`,
		`{"title":"ok","body":"` + strings.Repeat("b", 10001) + `","completed":false}`,
	} {
		m := newMemory()
		h := handler(m)
		status(t, call(h, "POST", "/api/v1/tasks", payload), 422)
		if m.writes != 0 {
			t.Fatal("invalid input wrote to storage")
		}
	}
	h := handler(newMemory())
	status(t, call(h, "POST", "/api/v1/tasks", `{"title":`), 400)
	status(t, call(h, "GET", "/api/v1/tasks/not-a-uuid", ""), 422)
	status(t, call(h, "POST", "/api/v1/tasks", `{"title":"ok","body":"`+strings.Repeat("b", 70000)+`","completed":false}`), 413)
}
func TestPagination(t *testing.T) {
	resource := "tasks"
	m := newMemory()
	h := handler(m)
	path := "/api/v1/" + resource
	for _, suffix := range []string{"?limit=0", "?limit=101", "?limit=no", "?cursor=" + strings.Repeat("a", 4097)} {
		status(t, call(h, "GET", path+suffix, ""), 422)
	}
	status(t, call(h, "GET", path, ""), 200)
	if m.limit != 20 {
		t.Fatal(m.limit)
	}
	for range 3 {
		body := `{"title":"row","body":"","completed":false}`
		status(t, call(h, "POST", path, body), 201)
	}
	first := decode[store.Page](t, call(h, "GET", path+"?limit=2", ""))
	if len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("bad page: %+v", first)
	}
	second := decode[store.Page](t, call(h, "GET", path+"?limit=2&cursor="+first.NextCursor, ""))
	if len(second.Items) != 1 || second.NextCursor != "" || second.Items[0].ID == first.Items[0].ID {
		t.Fatal("bad continuation")
	}
}
func TestDependencyFailuresAndHealth(t *testing.T) {
	m := newMemory()
	m.failure = errors.Join(store.ErrUnavailable, errors.New("apiSecret=do-not-leak task-body=private"))
	h := handler(m)
	status(t, call(h, "GET", "/healthz", ""), 200)
	for _, path := range []string{"/readyz", "/api/v1/tasks"} {
		w := call(h, "GET", path, "")
		status(t, w, 503)
		if strings.Contains(w.Body.String(), "do-not-leak") || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatal(w.Body.String())
		}
	}
	resource := "tasks"
	path := "/api/v1/" + resource
	body := `{"title":"valid","body":"","completed":false}`
	status(t, call(h, "POST", path, body), 503)
	id := "00000000-0000-4000-8000-000000000000"
	status(t, call(h, "GET", path+"/"+id, ""), 503)
	status(t, call(h, "PUT", path+"/"+id, body), 503)
	status(t, call(h, "DELETE", path+"/"+id, ""), 503)
	m.failure = store.ErrConflict
	status(t, call(h, "POST", "/api/v1/tasks", `{"title":"ok","body":"","completed":false}`), 409)
	m.failure = nil
	status(t, call(h, "GET", "/readyz", ""), 200)
}
func TestOpenAPIAndDocs(t *testing.T) {
	h := handler(newMemory())
	w := call(h, "GET", "/openapi.json", "")
	status(t, w, 200)
	var spec struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if len(spec.Paths) != 4 {
		t.Fatalf("unexpected resources in spec: %v", spec.Paths)
	}
	status(t, call(h, "GET", "/api/v1/notes", ""), 404)
	if !strings.HasPrefix(spec.OpenAPI, "3.1.") {
		t.Fatal(spec.OpenAPI)
	}
	resource := "tasks"
	for path, methods := range map[string][]string{"/api/v1/" + resource: {"get", "post"}, "/api/v1/" + resource + "/{id}": {"get", "put", "delete"}} {
		for _, method := range methods {
			if spec.Paths[path][method] == nil {
				t.Fatalf("missing %s %s", method, path)
			}
		}
	}
	status(t, call(h, "GET", "/openapi.yaml", ""), 200)
	w = call(h, "GET", "/docs", "")
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), "/docs/assets/swagger-ui-bundle.js") || strings.Contains(w.Body.String(), `src="http`) || strings.Contains(w.Body.String(), `href="http`) {
		t.Fatal("docs must use local Swagger UI assets")
	}
	for _, asset := range []string{"swagger-ui-bundle.js", "swagger-ui.css", "init.js"} {
		status(t, call(h, "GET", "/docs/assets/"+asset, ""), 200)
	}
	init := call(h, "GET", "/docs/assets/init.js", "")
	if !strings.Contains(init.Body.String(), "validatorUrl: null") || !strings.Contains(init.Body.String(), `url: "/openapi.json"`) {
		t.Fatal("docs configuration is not local")
	}
	status(t, call(h, "GET", "/docs/assets/missing.js", ""), 404)
}

func TestUnexpectedFailuresAndRouting(t *testing.T) {
	m := newMemory()
	m.failure = errors.New("private unexpected payload")
	h := handler(m)
	w := call(h, "GET", "/api/v1/tasks", "")
	status(t, w, 500)
	if strings.Contains(w.Body.String(), "private") {
		t.Fatal("internal error exposed")
	}
	status(t, call(h, "GET", "/readyz", ""), 503)
	m.failure = store.ErrAuth
	status(t, call(h, "GET", "/api/v1/tasks", ""), 503)
	m.failure = store.ErrCursor
	status(t, call(h, "GET", "/api/v1/tasks?cursor=bad", ""), 400)
	w = call(h, "GET", "/missing", "")
	status(t, w, 404)
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatal("routing failure is not a problem")
	}
	status(t, call(h, "PATCH", "/api/v1/tasks", ""), 405)
}
