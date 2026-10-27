// internal/httpapi/server.go
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/OnyxDevTools/api-starter/internal/store"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type TaskInput struct {
	Title     string `json:"title" minLength:"1" maxLength:"120" pattern:"\\S" example:"Shopping list" doc:"Trimmed, nonblank title"`
	Body      string `json:"body" maxLength:"10000" example:"Bread and coffee"`
	Completed bool   `json:"completed" example:"false"`
}

func (v *TaskInput) Resolve(_ huma.Context) []error {
	v.Title = strings.TrimSpace(v.Title)
	if v.Title == "" {
		return []error{&huma.ErrorDetail{Message: "title must not be blank", Location: "body.title"}}
	}
	return nil
}

type IDInput struct {
	ID string `path:"id" format:"uuid"`
}
type ListInput struct {
	Limit  int    `query:"limit" default:"20" minimum:"1" maximum:"100"`
	Cursor string `query:"cursor" maxLength:"4096"`
}
type TaskWrite struct{ Body TaskInput }
type TaskUpdate struct {
	IDInput
	Body TaskInput
}
type TaskOutput struct{ Body store.Task }
type TaskCreated struct {
	Location string `header:"Location"`
	Body     store.Task
}
type TasksOutput struct{ Body store.Page }
type Health struct {
	Status string `json:"status"`
}
type HealthOutput struct{ Body Health }

// New registers the same typed operations for runtime and offline OpenAPI export.
// A nil repository is used only for exporting contracts, never by cmd/api.
func New(repo store.Repository, log *slog.Logger) (http.Handler, huma.API) {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(observe(log))
	r.Use(recoverJSON(log))
	r.Use(deadline)
	cfg := huma.DefaultConfig("API Starter", "1.0.0")
	cfg.DocsPath = ""
	api := humachi.New(r, cfg)
	register(api, repo, log)
	r.Get("/docs", serveDocs())
	r.Get("/docs/assets/{file}", serveDocsAsset)
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { writeProblem(w, 404) })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) { writeProblem(w, 405) })
	return r, api
}
func deadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func observe(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			id := middleware.GetReqID(r.Context())
			ww.Header().Set("X-Request-ID", id)
			next.ServeHTTP(ww, r)
			log.Info("http request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "duration", time.Since(start))
		})
	}
}
func recoverJSON(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recover() != nil {
					log.Error("request panic", "request_id", middleware.GetReqID(r.Context()))
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"title":"Internal Server Error","status":500,"detail":"An unexpected error occurred"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
func failure(ctx context.Context, log *slog.Logger, err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return huma.Error404NotFound("Record not found")
	case errors.Is(err, store.ErrCursor):
		return huma.Error400BadRequest("Invalid continuation cursor")
	case errors.Is(err, store.ErrConflict):
		return huma.Error409Conflict("Record already exists")
	default:
		// Never log SDK error strings: they may contain response bodies or credentials.
		kind := "unexpected"
		if errors.Is(err, store.ErrAuth) {
			kind = "authentication"
		} else if errors.Is(err, store.ErrUnavailable) {
			kind = "unavailable"
		}
		log.Error("storage operation failed", "request_id", middleware.GetReqID(ctx), "kind", kind)
		if kind == "unexpected" {
			return huma.Error500InternalServerError("Unexpected storage response")
		}
		return huma.Error503ServiceUnavailable("Database temporarily unavailable")
	}
}
func writeProblem(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"status": status, "title": http.StatusText(status)})
}
func operation(id, method, path, tag, summary string, status int) huma.Operation {
	codes := []int{404, 422, 500, 503}
	switch method {
	case "POST":
		codes = []int{400, 409, 413, 422, 500, 503}
	case "PUT":
		codes = []int{400, 404, 413, 422, 500, 503}
	case "GET":
		if id == "health" {
			codes = nil
		} else if id == "ready" {
			codes = []int{503}
		} else if strings.HasPrefix(id, "list-") {
			codes = []int{400, 422, 500, 503}
		}
	}
	return huma.Operation{OperationID: id, Method: method, Path: path, Tags: []string{tag}, Summary: summary, DefaultStatus: status, MaxBodyBytes: 65536, Errors: codes, RejectUnknownQueryParameters: true}
}
func register(api huma.API, repo store.Repository, log *slog.Logger) {
	huma.Register(api, operation("health", "GET", "/healthz", "Health", "Process liveness", 200), func(_ context.Context, _ *struct{}) (*HealthOutput, error) {
		return &HealthOutput{Body: Health{Status: "ok"}}, nil
	})
	huma.Register(api, operation("ready", "GET", "/readyz", "Health", "Read-only database readiness", 200), func(ctx context.Context, _ *struct{}) (*HealthOutput, error) {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := repo.Ready(ctx); err != nil {
			log.Warn("database readiness failed", "request_id", middleware.GetReqID(ctx))
			return nil, huma.Error503ServiceUnavailable("Database not ready")
		}
		return &HealthOutput{Body: Health{Status: "ready"}}, nil
	})
	registerTasks(api, repo, log)
}
