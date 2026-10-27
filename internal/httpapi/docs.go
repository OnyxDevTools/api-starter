// internal/httpapi/docs.go
package httpapi

import (
	"bytes"
	"embed"
	"net/http"
	"strings"
	"time"
)

//go:embed docs.html docs-assets/swagger-ui-bundle.js docs-assets/swagger-ui.css docs-assets/init.js
//go:embed docs-assets/LICENSE docs-assets/NOTICE docs-assets/swagger-ui-bundle.js.LICENSE.txt
var docsFiles embed.FS

// serveDocs serves self-hosted Swagger UI. No CDN, online validator, account,
// or hosted documentation service is used; the browser calls this API directly.
func serveDocs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := docsFiles.ReadFile("docs.html")
		if err != nil {
			writeProblem(w, 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeContent(w, r, "docs.html", time.Time{}, bytes.NewReader(page))
	}
}
func serveDocsAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/docs/assets/")
	switch name {
	case "swagger-ui-bundle.js", "init.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case "LICENSE", "NOTICE", "swagger-ui-bundle.js.LICENSE.txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	case "swagger-ui.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	default:
		writeProblem(w, 404)
		return
	}
	data, err := docsFiles.ReadFile("docs-assets/" + name)
	if err != nil {
		writeProblem(w, 404)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}
