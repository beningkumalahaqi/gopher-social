// Package apispec turns the embedded OpenAPI document into the learning
// platform's endpoint catalogue. The YAML remains the single source of truth:
// paths, parameters, schemas, and security are read, never re-declared.
package apispec

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Param is one path, query, header, or cookie parameter.
type Param struct {
	Name        string
	In          string
	Required    bool
	Description string
	Schema      string
}

// Response is one documented status code.
type Response struct {
	Status      string
	Description string
	Schema      string
}

// Operation is one method on one path.
type Operation struct {
	ID          string
	Method      string
	Path        string
	Summary     string
	Description string
	Tag         string
	Security    string
	Params      []Param
	Request     string
	Responses   []Response

	HandlerFile string
	HandlerNote string
	LessonSlugs []string
}

// Spec is the parsed catalogue.
type Spec struct {
	Title       string
	Version     string
	Description string
	Tags        []string
	Operations  []Operation
	byID        map[string]*Operation
}

// Operation returns one operation by operationId.
func (s *Spec) Operation(id string) (*Operation, bool) {
	op, ok := s.byID[id]
	return op, ok
}

// Group returns operations with the given tag, or every operation when the tag
// is empty.
func (s *Spec) Group(tag string) []Operation {
	if tag == "" {
		return s.Operations
	}
	var out []Operation
	for _, op := range s.Operations {
		if op.Tag == tag {
			out = append(out, op)
		}
	}
	return out
}

// TagCount is used by the filter list.
type TagCount struct {
	Tag   string
	Count int
}

// Counts returns tag counts in documented order, then any extra tags.
func (s *Spec) Counts() []TagCount {
	counts := make(map[string]int, len(s.Tags))
	for _, op := range s.Operations {
		counts[op.Tag]++
	}
	seen := make(map[string]bool, len(s.Tags))
	var out []TagCount
	for _, tag := range s.Tags {
		seen[tag] = true
		out = append(out, TagCount{Tag: tag, Count: counts[tag]})
	}
	var extra []string
	for tag := range counts {
		if !seen[tag] {
			extra = append(extra, tag)
		}
	}
	sort.Strings(extra)
	for _, tag := range extra {
		out = append(out, TagCount{Tag: tag, Count: counts[tag]})
	}
	return out
}

// lessonLinks maps a tag to the lessons that explain its implementation.
var lessonLinks = map[string][]string{
	"Operations":    {"23-shutdown-and-health"},
	"Auth":          {"13-password-hashing", "14-jwt-access-tokens", "15-refresh-rotation", "16-auth-middleware"},
	"Users":         {"11-models-and-dtos", "12-validation", "16-auth-middleware"},
	"Posts":         {"17-handlers", "18-pagination"},
	"Comments":      {"21-comments-and-reactions"},
	"Reactions":     {"21-comments-and-reactions"},
	"Discovery":     {"19-search", "20-feed-and-social-graph"},
	"Notifications": {"22-background-workers"},
}

// operationNotes explains what the Go handler does. The file paths are real
// source files; the note is intentionally one sentence, not a second contract.
var operationNotes = map[string]struct {
	File    string
	Note    string
	Lessons []string
}{
	"live":              {"internal/api/handlers/health.go", "Returns 200 without touching dependencies, so an orchestrator can restart a wedged process.", nil},
	"ready":             {"internal/api/handlers/health.go", "Pings PostgreSQL with a short timeout and returns 503 when the dependency is down.", nil},
	"register":          {"internal/api/handlers/auth.go", "Strict-decodes the body, validates fields, hashes the password, and issues the first refresh session in one transaction.", []string{"13-password-hashing", "17-handlers"}},
	"login":             {"internal/api/handlers/auth.go", "Verifies the password with a dummy hash for unknown users, then rotates a new session.", []string{"13-password-hashing", "15-refresh-rotation"}},
	"refresh":           {"internal/api/handlers/auth.go", "Rotates the refresh token inside a transaction and revokes the old row; replay revokes every session.", []string{"15-refresh-rotation"}},
	"logout":            {"internal/api/handlers/auth.go", "Revokes the presented refresh token and stays idempotent.", []string{"15-refresh-rotation"}},
	"me":                {"internal/api/handlers/users.go", "Reads the identity attached by auth middleware and loads the viewer's profile.", []string{"16-auth-middleware"}},
	"updateMe":          {"internal/api/handlers/users.go", "Merges partial profile fields, validates them, and writes the merged row.", []string{"12-validation"}},
	"changePassword":    {"internal/api/handlers/auth.go", "Verifies the current password, hashes the new one, and revokes every refresh session.", []string{"13-password-hashing"}},
	"getUser":           {"internal/api/handlers/users.go", "Loads a public profile by normalized username and hides credentials in the DTO.", []string{"11-models-and-dtos"}},
	"userPosts":         {"internal/api/handlers/posts.go", "Reuses the paginated post store with an author filter and viewer-specific fields.", []string{"18-pagination"}},
	"followers":         {"internal/api/handlers/users.go", "Lists follower profiles through the social graph store.", []string{"20-feed-and-social-graph"}},
	"following":         {"internal/api/handlers/users.go", "Lists followed profiles through the social graph store.", []string{"20-feed-and-social-graph"}},
	"followStatus":      {"internal/api/handlers/users.go", "Answers one boolean for the viewer, which keeps the button state cheap to refresh.", []string{"20-feed-and-social-graph"}},
	"suggestions":       {"internal/api/handlers/users.go", "Suggests accounts the viewer does not already follow.", []string{"20-feed-and-social-graph"}},
	"follow":            {"internal/api/handlers/posts.go", "Creates the follow edge and enqueues a best-effort notification.", []string{"20-feed-and-social-graph", "22-background-workers"}},
	"unfollow":          {"internal/api/handlers/posts.go", "Deletes the follow edge; missing rows are treated as success.", []string{"20-feed-and-social-graph"}},
	"createPost":        {"internal/api/handlers/posts.go", "Validates content length, writes the post, and never trusts a client-supplied author.", []string{"17-handlers"}},
	"listPosts":         {"internal/api/handlers/posts.go", "Decodes limit and cursor, then delegates ordering to the keyset store.", []string{"18-pagination"}},
	"getPost":           {"internal/api/handlers/posts.go", "Loads one visible post and adds viewer-specific reaction fields.", []string{"17-handlers"}},
	"updatePost":        {"internal/api/handlers/posts.go", "Checks ownership, applies partial updates, and returns the row.", []string{"10-stores-and-transactions"}},
	"deletePost":        {"internal/api/handlers/posts.go", "Deletes by id after an ownership check; dependent rows cascade.", []string{"10-stores-and-transactions"}},
	"createComment":     {"internal/api/handlers/comments.go", "Verifies the post is visible, inserts the comment, and enqueues a notification.", []string{"21-comments-and-reactions"}},
	"listComments":      {"internal/api/handlers/comments.go", "Paginates comments in a stable order for one post.", []string{"18-pagination"}},
	"updateComment":     {"internal/api/handlers/comments.go", "Checks comment ownership before updating the body.", []string{"21-comments-and-reactions"}},
	"deleteComment":     {"internal/api/handlers/comments.go", "Checks comment ownership before deleting.", []string{"21-comments-and-reactions"}},
	"setReaction":       {"internal/api/handlers/reactions.go", "Upserts the viewer's reaction on the composite key, so switching type is one write.", []string{"21-comments-and-reactions"}},
	"deleteReaction":    {"internal/api/handlers/reactions.go", "Removes the viewer's reaction if present.", []string{"21-comments-and-reactions"}},
	"listReactions":     {"internal/api/handlers/reactions.go", "Lists reactions for a post with the actor profile.", []string{"21-comments-and-reactions"}},
	"reactionSummary":   {"internal/api/handlers/reactions.go", "Aggregates counts by type and adds the viewer's own reaction when authenticated.", []string{"21-comments-and-reactions"}},
	"getFeed":           {"internal/api/handlers/feed.go", "Joins followed authors to posts and paginates by keyset.", []string{"20-feed-and-social-graph"}},
	"explore":           {"internal/api/handlers/feed.go", "Ranks public posts for discovery without requiring a session.", []string{"20-feed-and-social-graph"}},
	"searchUsers":       {"internal/api/handlers/users.go", "Runs a prefix search on normalized usernames.", []string{"19-search"}},
	"searchPosts":       {"internal/api/handlers/posts.go", "Uses the GIN search index and returns highlighted snippets.", []string{"19-search"}},
	"listNotifications": {"internal/api/handlers/notifications.go", "Lists the viewer's notifications, newest first.", []string{"22-background-workers"}},
	"unreadCount":       {"internal/api/handlers/notifications.go", "Counts unread rows for the viewer.", []string{"22-background-workers"}},
	"markRead":          {"internal/api/handlers/notifications.go", "Marks one notification read after an ownership check.", []string{"22-background-workers"}},
	"markAllRead":       {"internal/api/handlers/notifications.go", "Marks every unread notification for the viewer in one statement.", []string{"22-background-workers"}},
}

// Parse builds the catalogue from the OpenAPI bytes.
func Parse(document []byte) (*Spec, error) {
	var root map[string]any
	if err := yaml.Unmarshal(document, &root); err != nil {
		return nil, fmt.Errorf("parse openapi: %w", err)
	}

	spec := &Spec{byID: make(map[string]*Operation)}
	if info := asMap(root["info"]); info != nil {
		spec.Title = asString(info["title"])
		spec.Version = asString(info["version"])
		spec.Description = strings.TrimSpace(asString(info["description"]))
	}
	for _, raw := range asSlice(root["tags"]) {
		tag := asMap(raw)
		if name := asString(tag["name"]); name != "" {
			spec.Tags = append(spec.Tags, name)
		}
	}
	rootSecurity := securityLabel(root["security"], true)

	for path, rawMethods := range asMap(root["paths"]) {
		methods := asMap(rawMethods)
		pathParams := paramsFrom(methods["parameters"])
		for method, rawOp := range methods {
			method = strings.ToUpper(method)
			if !httpMethod(method) {
				continue
			}
			opMap := asMap(rawOp)
			if opMap == nil {
				continue
			}
			tag := ""
			if tags := asSlice(opMap["tags"]); len(tags) > 0 {
				tag = asString(tags[0])
			}
			op := Operation{
				ID:          asString(opMap["operationId"]),
				Method:      method,
				Path:        path,
				Summary:     asString(opMap["summary"]),
				Description: strings.TrimSpace(asString(opMap["description"])),
				Tag:         tag,
				Security:    securityLabel(opMap["security"], rootSecurity == "Public"),
				Params:      append(pathParams, paramsFrom(opMap["parameters"])...),
				Request:     requestSchema(opMap["requestBody"]),
				Responses:   responsesFrom(opMap["responses"]),
				HandlerFile: "internal/api/router.go",
				LessonSlugs: append([]string{}, lessonLinks[tag]...),
			}
			if note, ok := operationNotes[op.ID]; ok {
				op.HandlerFile = note.File
				op.HandlerNote = note.Note
				if len(note.Lessons) > 0 {
					op.LessonSlugs = note.Lessons
				}
			}
			spec.Operations = append(spec.Operations, op)
		}
	}

	sort.Slice(spec.Operations, func(i, j int) bool {
		a, b := spec.Operations[i], spec.Operations[j]
		if a.Tag != b.Tag {
			return tagRank(spec.Tags, a.Tag) < tagRank(spec.Tags, b.Tag)
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Method < b.Method
	})
	for i := range spec.Operations {
		if spec.Operations[i].HandlerNote == "" {
			spec.Operations[i].HandlerNote = "The handler decodes, validates, and delegates to the domain layer; business rules stay out of the transport package."
		}
		if spec.Operations[i].ID != "" {
			spec.byID[spec.Operations[i].ID] = &spec.Operations[i]
		}
	}
	return spec, nil
}

func tagRank(tags []string, tag string) int {
	for i, t := range tags {
		if t == tag {
			return i
		}
	}
	return len(tags)
}

func paramsFrom(raw any) []Param {
	var out []Param
	for _, item := range asSlice(raw) {
		p := asMap(item)
		if p == nil {
			continue
		}
		out = append(out, Param{
			Name:        asString(p["name"]),
			In:          asString(p["in"]),
			Required:    asBool(p["required"]),
			Description: strings.TrimSpace(asString(p["description"])),
			Schema:      snippet(p["schema"]),
		})
	}
	return out
}

func requestSchema(raw any) string {
	body := asMap(raw)
	if body == nil {
		return ""
	}
	content := asMap(body["content"])
	if content == nil {
		return ""
	}
	for _, media := range []string{"application/json", "application/x-www-form-urlencoded"} {
		if mediaType := asMap(content[media]); mediaType != nil {
			return snippet(mediaType["schema"])
		}
	}
	return ""
}

func responsesFrom(raw any) []Response {
	responses := asMap(raw)
	var out []Response
	for status, rawResponse := range responses {
		response := asMap(rawResponse)
		if response == nil {
			continue
		}
		schema := ""
		if content := asMap(response["content"]); content != nil {
			if media := asMap(content["application/json"]); media != nil {
				schema = snippet(media["schema"])
			}
		}
		out = append(out, Response{
			Status:      status,
			Description: strings.TrimSpace(asString(response["description"])),
			Schema:      schema,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Status < out[j].Status })
	return out
}

func securityLabel(raw any, inheritPublic bool) string {
	requirements := asSlice(raw)
	if len(requirements) == 0 {
		if len(requirements) == 0 && raw == nil && inheritPublic {
			return "Public"
		}
		if raw == nil {
			return "Inherited"
		}
		return "Public"
	}
	requiresAuth := false
	allowsAnonymous := false
	for _, requirement := range requirements {
		req := asMap(requirement)
		if len(req) == 0 {
			allowsAnonymous = true
			continue
		}
		if _, ok := req["bearerAuth"]; ok {
			requiresAuth = true
		}
	}
	if requiresAuth && allowsAnonymous {
		return "Optional bearer token"
	}
	if requiresAuth {
		return "Bearer token required"
	}
	return "Public"
}

func httpMethod(method string) bool {
	switch method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return true
	}
	return false
}

func snippet(value any) string {
	if value == nil {
		return ""
	}
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(encoded), "\n"), "\n")
	if len(lines) > 24 {
		lines = append(lines[:24], "  # ... truncated for display")
	}
	return strings.Join(lines, "\n")
}

func asMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return nil
}

func asSlice(value any) []any {
	if s, ok := value.([]any); ok {
		return s
	}
	return nil
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asBool(value any) bool {
	if b, ok := value.(bool); ok {
		return b
	}
	return false
}
