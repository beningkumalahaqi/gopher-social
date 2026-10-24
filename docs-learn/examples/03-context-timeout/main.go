// Command context-timeout shows how a deadline cancels downstream work.
//
//	go run ./docs-learn/examples/03-context-timeout
//	curl -i localhost:8093/slow
package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", slow)

	srv := &http.Server{
		Addr:              ":8093",
		Handler:           timeout(50 * time.Millisecond)(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("listening on http://localhost:8093")
	log.Fatal(srv.ListenAndServe())
}

func timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func slow(w http.ResponseWriter, r *http.Request) {
	select {
	case <-time.After(200 * time.Millisecond):
		_, _ = w.Write([]byte("finished\n"))
	case <-r.Context().Done():
		// The client may already be gone, so this write can fail.
		w.WriteHeader(http.StatusGatewayTimeout)
		_, _ = w.Write([]byte("deadline exceeded\n"))
		log.Printf("cancelled: %v", r.Context().Err())
	}
}
