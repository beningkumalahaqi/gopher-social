// Package docs exposes the API contract to the server.
//
// The OpenAPI file is the source of truth for the interactive docs. Embedding it
// keeps the running binary self-contained: no file path, no separate web server.
package docs

import _ "embed"

// OpenAPI is the OpenAPI 3.1 contract served at /swagger/openapi.yaml.
//
//go:embed openapi.yaml
var OpenAPI []byte
