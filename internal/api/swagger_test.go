package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haqi/gophersocial/internal/config"
)

func TestSwaggerUIServesHTML(t *testing.T) {
	rec := httptest.NewRecorder()
	swaggerUI().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("content type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatal("page does not initialize Swagger UI")
	}
	if !strings.Contains(body, swaggerSpecPath) {
		t.Fatalf("page does not reference %s", swaggerSpecPath)
	}
}

func TestSwaggerUIAssets(t *testing.T) {
	rec := httptest.NewRecorder()
	swaggerUI().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/swagger-ui.css", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty asset")
	}
}

func TestOpenAPISpec(t *testing.T) {
	rec := httptest.NewRecorder()
	openAPISpec(rec, httptest.NewRequest(http.MethodGet, swaggerSpecPath, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Fatalf("content type = %q", ct)
	}
	if !strings.HasPrefix(rec.Body.String(), "openapi: 3.1.0") {
		t.Fatalf("unexpected spec: %.40q", rec.Body.String())
	}
}

func TestRouterServesSwagger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(Deps{
		Config: config.Config{
			RequestTimeout: time.Second,
			AuthRateLimit:  10,
			AuthRateWindow: time.Minute,
		},
		Logger: logger,
	})

	t.Run("redirect", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger", nil))
		if rec.Code != http.StatusTemporaryRedirect &&
			rec.Code != http.StatusMovedPermanently &&
			rec.Code != http.StatusPermanentRedirect {
			t.Fatalf("status = %d", rec.Code)
		}
		if loc := rec.Header().Get("Location"); loc != swaggerBasePath {
			t.Fatalf("location = %q", loc)
		}
	})

	t.Run("ui", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/swagger/", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("spec", func(t *testing.T) {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, swaggerSpecPath, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "GopherSocial API") {
			t.Fatal("spec not served")
		}
	})
}
