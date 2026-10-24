// Command strict-json shows bounded, strict JSON decoding for an API.
//
//	go run ./docs-learn/examples/05-strict-json
//	curl -i -X POST localhost:8095/users -d '{"email":"a@b.co","password":"secret123"}'
//	curl -i -X POST localhost:8095/users -d '{"email":"a@b.co","extra":1}'
package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", register)

	srv := &http.Server{
		Addr:              ":8095",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Println("listening on http://localhost:8095")
	log.Fatal(srv.ListenAndServe())
}

func register(w http.ResponseWriter, r *http.Request) {
	const maxBody = 1 << 20 // 1 MiB
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req registerRequest
	if err := dec.Decode(&req); err != nil {
		status := http.StatusBadRequest
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, err.Error())
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "body must contain a single JSON object")
		return
	}
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "email and password (8+) are required")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]string{"email": req.Email},
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"message": msg},
	})
}
