# 28. HTMX without a client framework

## Goal

Use HTMX to update parts of a page while keeping links and forms functional
without JavaScript.

## The contract

HTMX does not need a JSON API. It needs endpoints that return HTML. The
contract for every interactive control is:

1. A real URL that works as a normal link or form action.
2. A response that is valid HTML for the target element.
3. A target and swap mode that match the shape of that response.

The lesson page is a complete example. A normal request renders the whole
document. The table of contents and previous/next links are links, not
`hx-get` buttons. The reading control uses HTMX because updating one card does
not justify a full navigation:

```html
<button type="button"
        hx-post="/learn/progress"
        hx-vals='{"lesson_slug":"{{.Doc.Slug}}","status":"completed"}'
        hx-target="#progress-control"
        hx-swap="outerHTML">
  Mark complete
</button>
```

If JavaScript fails to load, the page still reads. The buttons do not work, but
every lesson is reachable by URL. That is the progressive-enhancement test.

## Same endpoint, two response modes

`handleProgressUpdate` inspects the `HX-Request` header:

```go
if s.isHTMX(r) {
    s.renderFragment(w, "progress-control", data)
    return
}
http.Redirect(w, r, "/learn/lessons/"+slug, http.StatusSeeOther)
```

An HTMX request gets a fragment. A plain form post gets a redirect. The handler
does not maintain two routes or two validation paths.

The redirect uses 303 so a refresh does not repeat the write. The fragment path
must never redirect, because HTMX would follow it and swap the entire lesson
page into the progress card.

## Targets should be stable

`#progress-control` is rendered by the page and by the fragment. That is not a
coincidence. The fragment's root element carries the id:

```html
<section id="progress-control" class="progress-control">
```

`hx-swap="outerHTML"` replaces the element and its id, so the next request
still finds the target. If the fragment returned only an inner list, the id
would disappear after the first swap.

The same rule applies to `#lesson-content`, `#note-status`, and `#search-results`.
Either keep the id on a stable parent and swap `innerHTML`, or return the
parent with `outerHTML`. Do not mix the two by accident.

## Push the URL only when navigation changed

`hx-push-url="true"` belongs on controls that behave like navigation, such as a
table-of-contents link that loads a new lesson body. It does not belong on a
bookmark toggle, because the user has not navigated and Back should not undo a
bookmark.

For search, the form uses `hx-get` with `hx-push-url="true"` so the query
appears in the address bar and can be shared. The server also handles a normal
GET, so the same URL works without HTMX.

## Loading and error states

`learn.js` listens for HTMX events and shows a small status line:

```js
document.body.addEventListener("htmx:responseError", () => {
  toast("That action did not go through. Try again.");
});
```

HTMX adds `htmx-request` to the requesting element. CSS can use it to reduce
opacity or show a spinner. Do not replace the whole page with a spinner. The
user needs the surrounding context while one control updates.

A 500 response is not swapped by default. That is good: a broken fragment
should not erase the working page. Show a message and keep the old markup.

## Security details that do not change

HTMX requests are still HTTP requests. They carry cookies, so they need CSRF
protection. The server sets a meta tag with the token, and `learn.js` adds it to
every non-GET request:

```js
document.body.addEventListener("htmx:configRequest", (event) => {
  event.detail.headers["X-CSRF-Token"] = csrfToken;
});
```

The form posts also include a hidden `csrf_token` field, so they work without
JavaScript. The middleware accepts either the header or the form value and
rejects anything else.

Authorization is unchanged. A fragment handler checks ownership exactly like a
page handler. Returning HTML does not make an endpoint less sensitive.

## When not to use HTMX

Use a normal link when the whole page changes and the state is in the URL. Use
a normal form when the result is a redirect. Use HTMX when the server can
return a small, self-contained fragment and the rest of the page should stay
put.

Do not use it to build a client-side application one fragment at a time. If the
page needs complex local state, optimistic updates, or offline behaviour, a
small dedicated script is clearer than a pile of `hx-` attributes.

## Exercises

1. Convert the bookmark button to a normal form post and compare the UX. Which
   behaviour is better for a toggle?
2. Add `hx-indicator` to the note form and style the pending state.
3. Trigger a fragment 500 and confirm the page keeps its old content.

## What to remember

HTMX is HTML over HTTP with a target contract. Keep normal URLs working, return
fragments with stable roots, use 303 for plain posts, and protect every write
with CSRF.
