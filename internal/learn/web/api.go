package web

import (
	"net/http"

	"github.com/haqi/gophersocial/internal/learn"
	"github.com/haqi/gophersocial/internal/learn/apispec"
)

type apiData struct {
	Counts     []apispec.TagCount
	Operations []apispec.Operation
	Tag        string
	Op         *apispec.Operation
	Lessons    []*learn.Doc
	Total      int
}

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	state := stateFrom(r)
	page := s.newPage(r, state, "API reference", "Every endpoint in the running service, generated from the same OpenAPI document that ships with the code.")
	tag := r.URL.Query().Get("tag")
	data := apiData{
		Counts:     s.spec.Counts(),
		Operations: s.spec.Group(tag),
		Tag:        tag,
		Total:      len(s.spec.Operations),
	}
	if id := r.URL.Query().Get("op"); id != "" {
		if op, ok := s.spec.Operation(id); ok {
			data.Op = op
			data.Lessons = s.lessonDocs(op.LessonSlugs)
		}
	}
	page.Data = data
	s.fullPage(w, r, "api", page)
}

func (s *Server) handleAPIOperation(w http.ResponseWriter, r *http.Request) {
	if !s.isHTMX(r) {
		s.redirect(w, r, "/learn/api?op="+r.PathValue("id"), http.StatusSeeOther)
		return
	}
	op, ok := s.spec.Operation(r.PathValue("id"))
	if !ok {
		s.fail(w, r, http.StatusNotFound, "Endpoint not found", "That operation is not in the OpenAPI document.")
		return
	}
	s.renderFragment(w, r, "api-detail", struct {
		Op      *apispec.Operation
		Lessons []*learn.Doc
	}{Op: op, Lessons: s.lessonDocs(op.LessonSlugs)})
}

func (s *Server) lessonDocs(slugs []string) []*learn.Doc {
	var docs []*learn.Doc
	for _, slug := range slugs {
		if doc, ok := s.curriculum.Lesson(slug); ok {
			docs = append(docs, doc)
		}
	}
	return docs
}
