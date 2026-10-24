// Command hello-server is the smallest useful HTTP server in this module.
//
//	go run ./docs-learn/examples/01-hello-server
//	curl localhost:8091/hello?name=Ada
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", hello)

	srv := &http.Server{
		Addr:              ":8091",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Println("listening on http://localhost:8091")
	log.Fatal(srv.ListenAndServe())
}

func hello(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "hello, " + name})
}
