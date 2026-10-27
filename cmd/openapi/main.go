// cmd/openapi/main.go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/OnyxDevTools/api-starter/internal/httpapi"
	"io"
	"log/slog"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	_, api := httpapi.New(nil, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	data, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile("api/openapi.json", append(data, '\n'), 0644); err != nil {
		return err
	}
	data, err = api.OpenAPI().YAML()
	if err != nil {
		return err
	}
	return os.WriteFile("api/openapi.yaml", data, 0644)
}
