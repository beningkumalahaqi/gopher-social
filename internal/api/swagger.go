package api

import (
	"net/http"

	"github.com/swaggest/swgui/v5emb"

	"github.com/haqi/gophersocial/docs"
)

const (
	swaggerBasePath = "/swagger/"
	swaggerSpecPath = "/swagger/openapi.yaml"
)

// swaggerUI serves the embedded Swagger UI. Assets come from the Go binary, so
// the page works offline and does not depend on a CDN.
func swaggerUI() http.Handler {
	return v5emb.New("GopherSocial API", swaggerSpecPath, swaggerBasePath)
}

// openAPISpec serves the contract loaded from docs/openapi.yaml at build time.
func openAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(docs.OpenAPI)
}
