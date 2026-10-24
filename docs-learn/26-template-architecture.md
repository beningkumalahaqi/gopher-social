# 26. Templates that stay maintainable

## Goal

Understand how the learning interface turns data into HTML, and why the
template layer is separate from the HTTP handlers.

## One parse at startup

Every template is compiled before the server accepts a request. The renderer
lives in `internal/learn/web/render.go`:

```go
base, err := template.New("layout").Funcs(funcs).ParseFS(contentFS, baseFiles...)
...
for _, name := range pageNames {
    clone, err := base.Clone()
    if err != nil {
        return nil, err
    }
    if _, err := clone.ParseFS(contentFS, "templates/pages/"+name+".html"); err != nil {
        return nil, fmt.Errorf("parse page %s: %w", name, err)
    }
    r.pages[name] = clone
}
```

There is no template lookup on the request path. No file is read from disk, and
no template is parsed twice. A broken template fails startup, which is the
correct time to discover it.

The alternative, parsing inside a handler, has two costs. It hides errors until
the first request for that page, and it serializes parsing on whichever request
arrives first. Startup parsing removes both.

## A page is data, not behaviour

Handlers do not build strings. They fill a `pageData`:

```go
type pageData struct {
    Title        string
    Description  string
    Path         string
    Theme        string
    FontScale    string
    CSRF         string
    Profile      *learn.Profile
    Summary      learn.ProgressSummary
    Data         any
}
```

The outer fields are identical on every page, so the layout can rely on them.
`Data` is page-specific: a course page carries modules, a quiz page carries
questions and attempts. This keeps one stable contract for chrome and a typed
contract for the body.

The `Data any` field does lose compile-time safety. The trade is deliberate:
the outer layout is shared by fifteen pages, and the alternative is fifteen
page-specific layouts that drift. Keep the number of dynamically typed fields
small and render tests become practical.

## Autoescaping is the security boundary

`html/template` treats every interpolated value as untrusted text. A title
containing `<script>` is escaped, not executed. Lesson bodies are the one place
that must render HTML, and the type says so:

```go
type Doc struct {
    HTML template.HTML
    // ...
}
```

That value is produced by `markdown.go` after Goldmark renders and Bluemonday
sanitizes the result. The template never receives raw Markdown. If you add a
new field that contains author-provided HTML, run it through the same
sanitizer before giving it the `template.HTML` type. Do not cast a string in a
handler to silence the escaper.

## Helpers should be pure

The function map contains formatting only:

- `statusClass` and `statusLabel` choose a CSS class and readable text.
- `percent` avoids a division by zero.
- `initials`, `shortDate`, `timeAgo`, and `plural` format values.
- `asset` returns a content-hashed URL.

None of them query the database, read a cookie, or log. A helper that needs
request context belongs in the handler, where its failure can be handled once.

## Render to a buffer

A response that fails halfway through is worse than a clean error. The server
renders into a buffer first:

```go
var out strings.Builder
if err := s.renderer.page(&out, name, page); err != nil {
    s.logger.Error("render page", "page", name, "error", err)
    http.Error(w, "This page could not be rendered.", http.StatusInternalServerError)
    return
}
w.Header().Set("Content-Type", "text/html; charset=utf-8")
_, _ = w.Write([]byte(out.String()))
```

The cost is one allocation of the finished page. The benefit is that a template
bug produces a 500 with no partial markup, no leaked half-written `<html>`, and
a log line naming the page.

## Content hashing closes the cache problem

Static assets are served with a one-year immutable cache. That is only safe
because the URL changes when the file changes:

```go
raw, _ := fs.ReadFile(contentFS, "static/"+entry.Name())
sum := sha256.Sum256(raw)
r.assets[entry.Name()] = "/learn/static/" + entry.Name() + "?v=" + hex.EncodeToString(sum[:])[:10]
```

Templates ask for the URL with `{{asset "app.css"}}`. A deploy that changes the
stylesheet changes the URL, so no browser keeps the old file. The alternative,
`Cache-Control: no-cache`, sends a conditional request for every asset on every
page view.

## Exercises

1. Add a page template and register it in `pageNames`. Render it with an empty
   `pageData` and confirm the layout still works.
2. Break a template on purpose, start the server, and read the startup error.
   Then break a data field at render time and confirm the buffer prevents a
   partial response.
3. Find one helper in `render.go` that could fail if given bad data, and make
   it total.

## What to remember

Parse once, keep page data explicit, let the template engine escape, sanitize
the one field that needs HTML, and never stream a page you have not finished
rendering.
