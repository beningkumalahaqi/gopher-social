# 27. Layouts, fragments, and the render pipeline

## Goal

See how a single HTML document is assembled from a layout, a page, and named
fragments, and why fragments matter for interactive pages.

## Three template sets

The renderer keeps three parsed sets:

```go
base    *template.Template            // layout + partials
pages   map[string]*template.Template // base + one page
frag    *template.Template            // base + all fragments
```

`base.html` defines the shell: document head, skip link, sidebar, main region,
settings form, and script tags. `partials.html` defines named blocks such as
`lesson-content`, `progress-control`, `quiz-result`, and `search-results`.
Every page template defines `content` and is cloned onto a fresh copy of base.

Cloning matters. A page that defines a helper with a common name would
otherwise overwrite another page's helper. Each page gets an isolated
namespace built from the same base.

## The shell owns the invariants

Look at the layout in `templates/layout.html`. It always renders:

- the current path in the sidebar, with `aria-current="page"`;
- a skip link as the first focusable element;
- the CSRF token in a meta tag;
- the signed-in identity, or a guest label;
- the settings form, so preferences work from any page.

Pages cannot forget these. That is the point of the layout. A page only owns
its heading, breadcrumbs, and `content` block.

The layout receives the same `pageData` as the page. It uses only the outer
fields. If a page needs a badge in the header, add an outer field rather than
reaching into `Data` from the layout. The layout must not know the shape of
fifteen different page payloads.

## Fragments are responses, not includes

A fragment is a named template that a handler can execute on its own:

```go
func (s *Server) renderFragment(w http.ResponseWriter, name string, data any) {
    var out strings.Builder
    if err := s.renderer.fragment(&out, name, data); err != nil {
        s.logger.Error("render fragment", "fragment", name, "error", err)
        http.Error(w, "This section could not be rendered.", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    _, _ = w.Write([]byte(out.String()))
}
```

The lesson page includes `{{template "lesson-content" .}}` on first load. The
reading controls later request `/learn/lessons/{slug}/content` and receive the
same fragment alone. There is one source of markup for both paths.

This is the difference between a fragment and a partial. A partial is included
into a larger document. A fragment is a complete response that happens to also
be included. Designing for the second use keeps the first correct.

## What goes in a fragment

`lesson-content` contains the lesson prose, table of contents, previous and
next links, and the note form. It does not contain the sidebar or the page
header, because those do not change when the lesson body changes.

`progress-control` contains the status buttons and reading percentage. It is
returned after a progress update so the control reflects the new state without
a reload.

`quiz-result` contains the score, per-question feedback, and the sign-in prompt
for guests. It replaces the form after submission.

Each fragment has one job and one data shape. When a fragment starts needing
conditionals for unrelated pages, split it.

## Data shapes at the boundary

Handlers build small wrappers before rendering:

```go
type lessonContentData struct {
    learn.LessonState
    CSRF    string
    Profile *learn.Profile
}

type exercisePageData struct {
    Exercise *learn.Exercise
    State    learn.LessonState
    Result   *learn.ExerciseResult
    CSRF     string
}
```

Embedding `LessonState` means the fragment can use `.Doc`, `.Prev`, `.Next`,
and `.Status` without the handler copying fields. The wrapper adds only what
the fragment needs beyond that state. Keep wrappers close to the handler that
builds them, and keep them free of methods that touch the database.

## Rendering errors are request failures

Both page and fragment rendering use a buffer. A template execution error is
logged with the page or fragment name and returns a plain 500. The error is not
written into the HTML, because a half-rendered page with an error message in
the middle is harder to debug than a clean failure.

Test templates at startup with `TestTemplatesParse`. It parses every page
through the same code path as production. It will not catch a missing field in
a rarely visited page, so smoke-test new pages with real data.

## Exercises

1. Trace a request for `/learn/lessons/01-orientation` from `handleLesson`
   through `pageData` to `layout.html` and `lesson-content`.
2. Find a fragment that is used only as an include and decide whether it should
   become a real response. If not, why not?
3. Add a page-specific header badge without letting the layout read `Data`.

## What to remember

The layout owns chrome. Pages own content. Fragments are standalone responses
that can also be included. One markup source per concept, rendered through a
buffer, with errors handled before anything reaches the client.
