// cmd/docs/main.go
// Preview generated API documentation without initializing persistence.
package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/OnyxDevTools/api-starter/internal/config"
	"github.com/OnyxDevTools/api-starter/internal/httpapi"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration error", "error", err.Error())
		os.Exit(1)
	}
	docs, _ := httpapi.New(nil, log)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && (r.URL.Path == "/docs" || strings.HasPrefix(r.URL.Path, "/docs/assets/") || r.URL.Path == "/openapi.json" || r.URL.Path == "/openapi.yaml" || strings.HasPrefix(r.URL.Path, "/schemas/")) {
			docs.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":503,"title":"Documentation preview only","detail":"Start the configured API to execute requests."}`))
	})
	server := &http.Server{Addr: cfg.Address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}
	log.Info("documentation preview", "url", "http://"+cfg.Address+"/docs")
	if err := server.ListenAndServe(); err != nil {
		log.Error("docs server stopped", "error", err.Error())
		os.Exit(1)
	}
}
