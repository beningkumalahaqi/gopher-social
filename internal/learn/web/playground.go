package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// playgroundClient forwards code to the Go playground compile service. The
// service is outside this application's security boundary and receives only the
// code the learner typed.
type playgroundClient struct {
	url    string
	client *http.Client
}

func newPlaygroundClient(endpoint string, timeout time.Duration) *playgroundClient {
	if endpoint == "" {
		endpoint = "https://go.dev/_/compile"
	}
	return &playgroundClient{
		url:    endpoint,
		client: &http.Client{Timeout: timeout},
	}
}

type playgroundEvent struct {
	Message string `json:"Message"`
	Kind    string `json:"Kind"`
	Delay   int    `json:"Delay"`
}

type playgroundResponse struct {
	Errors string            `json:"Errors"`
	Events []playgroundEvent `json:"Events"`
}

func (c *playgroundClient) run(ctx context.Context, code string) (string, error) {
	form := url.Values{}
	form.Set("version", "2")
	form.Set("body", code)
	form.Set("withVet", "true")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "gophersocial-learn/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("playground request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("playground read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("playground responded %s", resp.Status)
	}

	var payload playgroundResponse
	if err := json.Unmarshal(bytes.TrimSpace(raw), &payload); err != nil {
		return "", fmt.Errorf("playground decode: %w", err)
	}

	var out strings.Builder
	if strings.TrimSpace(payload.Errors) != "" {
		out.WriteString(strings.TrimSpace(payload.Errors))
		out.WriteString("\n")
	}
	for _, event := range payload.Events {
		out.WriteString(event.Message)
	}
	result := out.String()
	if strings.TrimSpace(result) == "" {
		result = "The program compiled and produced no output."
	}
	return result, nil
}

func (s *Server) handlePlayground(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "Playground", "Run a small Go program without leaving the lesson. The compiler is the official Go playground service.")
	page.Data = struct {
		Code    string
		Enabled bool
		Output  string
		Error   string
	}{Code: defaultPlaygroundCode, Enabled: s.settings.PlaygroundEnabled}
	s.fullPage(w, r, "playground", page)
}

func (s *Server) handlePlaygroundRun(w http.ResponseWriter, r *http.Request) {
	code := r.PostFormValue("code")
	data := struct {
		Output  string
		Error   string
		Enabled bool
	}{Enabled: s.settings.PlaygroundEnabled}

	switch {
	case !s.settings.PlaygroundEnabled:
		data.Error = "The playground is disabled in this deployment."
	case strings.TrimSpace(code) == "":
		data.Error = "Write a program first."
	case len(code) > 64*1024:
		data.Error = "Programs are limited to 64 KB."
	default:
		allowed, retry := s.runLimit.Allow(clientKey(r))
		if !allowed {
			data.Error = fmt.Sprintf("Too many runs. Try again in %s.", retry.Round(time.Second))
		} else {
			output, err := s.playground.run(r.Context(), code)
			if err != nil {
				s.logger.Error("playground run", "error", err)
				data.Error = "The playground service could not run that program. Try again."
			} else {
				data.Output = output
			}
		}
	}
	s.renderFragment(w, r, "playground-output", data)
}

const defaultPlaygroundCode = `package main

import "fmt"

func main() {
	counts := map[string]int{"go": 1}
	counts["go"]++
	fmt.Println(counts)
}`
