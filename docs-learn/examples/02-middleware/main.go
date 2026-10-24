// Command middleware demonstrates a request ID and logging chain.
//
//	go run ./docs-learn/examples/02-middleware
//	curl -i localhost:8092/echo -H 'X-Request-ID: abc12345'
package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /echo", func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(requestIDKey).(string)
		w.Header().Set("X-Request-ID", id)
		_, _ = w.Write([]byte("request " + id + "\n"))
	})

	handler := requestID(logging(mux))
	srv := &http.Server{
		Addr:              ":8092",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("listening on http://localhost:8092")
	log.Fatal(srv.ListenAndServe())
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if len(id) < 8 {
			id = time.Now().Format("20060102150405.000000")
		}
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		id, _ := r.Context().Value(requestIDKey).(string)
		log.Printf("id=%s method=%s path=%s status=%d dur=%s",
			id, r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
