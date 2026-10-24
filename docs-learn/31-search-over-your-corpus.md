# 31. Search over your own corpus

## Goal

Build a search that is fast, explainable, and honest about its limits. The
platform searches roughly fifty documents, so the right design is an in-memory
index, not an external search cluster.

## Index once, at startup

`NewSearchIndex` walks lessons, exercise collections, interactive exercises,
quizzes, and glossary terms. Solutions are excluded so a learner cannot find an
answer by searching for its wording.

Each indexed document is flattened into a small struct:

```go
type searchDoc struct {
    slug, kind, title, titleLower string
    summary, text, textLower      string
    codeLower, headingsLow        string
    course, module                string
    number, readMinutes           int
    snippetText                   string
}
```

The index is immutable after construction. No lock is needed on reads, and a
request cannot observe a half-built index. Rebuilding happens on deploy, which
is exactly when the content changed.

## Scoring you can explain

The scorer is intentionally simple:

```go
if d.titleLower == token {
    score += 25
} else if strings.Contains(d.titleLower, token) {
    score += 15
}
if strings.Contains(d.headingsLow, token) {
    score += 7
}
if count := strings.Count(d.textLower, token); count > 0 {
    if count > 5 {
        count = 5
    }
    score += count * 2
}
if strings.Contains(d.codeLower, token) {
    score++
}
```

Every query token must match something, or the document is dropped. A title
match outranks a heading match, which outranks body text, which outranks code.
Repeated body matches are capped so a long lesson cannot win by repetition.

There is no TF-IDF, no stemming, and no synonym table. Those are reasonable
features for a larger corpus, but each one adds a tuning parameter that must be
evaluated with real queries. At this size, a reader can predict why a result
ranked where it did, which is worth more than a small relevance gain.

The known limitation is inflection: searching for `caching` will not match
`cache` unless the text contains the exact token. The tokenizer lowercases and
splits on punctuation, but does not stem. Document that limit instead of
pretending the search is semantic.

## Filters are applied before scoring

Course, module, kind, and progress status are checked before the scorer runs:

```go
if filters.Course != "" && doc.course != filters.Course {
    continue
}
```

Filtering first keeps the scorer's work proportional to the candidate set. It
also makes the result count meaningful: the number shown is the number after
filters, not a total that the user cannot reach.

Status is not stored in the index. It depends on the current profile, so it is
passed in as a map and resolved per request. An index that baked in one user's
progress would be wrong for everyone else.

## Snippets need escaping, then highlighting

A snippet is built from plain text. The highlighter must escape HTML before it
wraps matches in `<mark>`:

```go
escaped := html.EscapeString(text)
for _, token := range tokens {
    escaped = markToken(escaped, token)
}
return template.HTML(escaped)
```

If you mark first and escape second, the `<mark>` tags are escaped too. If you
escape first and interpolate the raw query into a regex, you have an injection
bug. The current helper escapes the text, then wraps only the matched ranges
with a fixed tag. The type is `template.HTML` because the function constructed
the markup from escaped input.

## Group results by kind

A mixed list of lessons, labs, and glossary terms is hard to scan. Results are
grouped in a fixed order: lessons, exercise collections, labs, quizzes, terms.
The flat `Hits` slice is still available for the total count and for callers
that need a single list.

The UI renders each group with its own heading and count. An empty group is
omitted. This is a presentation decision, but it belongs in the result shape so
handlers do not re-group on every render.

## Caching the query path

The search cache holds recent results for a short TTL, keyed by the normalized
query, filters, and a fingerprint of the profile's progress. The fingerprint
matters: status is part of the result, so a cache key that omits it would show
one learner's status to another.

Search is cheap here, so caching is an optimization, not a requirement. The
reason to include it is pedagogical: it shows a key that includes every input
that can change the output.

## Exercises

1. Search for a term that appears only in code and explain its score.
2. Add a `kind=quiz` filter and confirm lessons disappear.
3. Change progress on a lesson and confirm the cached result is not stale.

## What to remember

Index immutable data at startup, filter before scoring, use a ranking model you
can explain, escape before highlighting, and include every output-changing
input in a cache key.
