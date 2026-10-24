# Interactive API docs (Swagger UI)

The API serves its own Swagger UI. With the server running:

```text
http://localhost:8080/swagger/
```

`/swagger` redirects to `/swagger/`. The page loads the contract from
`/swagger/openapi.yaml`, so the UI and the raw spec always match.

## What you can do

- Browse every endpoint grouped by tag (Auth, Users, Posts, ...).
- Expand a request to see parameters, request bodies, and response schemas.
- Click **Try it out**, fill the fields, and execute against the running API.
- Click **Authorize**, paste an access token from `/api/v1/auth/login`, and
  authenticated requests will send `Authorization: Bearer <token>`.
- Download the spec from `/swagger/openapi.yaml`.

Endpoints with optional auth show the lock icon because they accept a bearer
token, but they also work anonymously. The `security` list in the spec contains
both `bearerAuth` and an empty requirement.

The docs routes are public and are not rate limited. They do not touch the
database.

## Where the spec lives

`docs/openapi.yaml` is the source of truth. It is embedded into the binary at
build time by `docs/embed.go` and served by `internal/api/swagger.go`. Editing
the YAML requires a rebuild (or a restart under `go run`) before the UI changes.

```text
docs/openapi.yaml          contract (edit this)
docs/embed.go              //go:embed openapi.yaml
internal/api/swagger.go    UI handler + spec route
internal/api/router.go     GET /swagger/, GET /swagger/openapi.yaml
```

Swagger UI assets are embedded too, so the page works without internet access
and without a CDN.

## Keeping the contract honest

When you add or change an endpoint:

1. Update the handler and DTOs.
2. Update `docs/openapi.yaml` (path, schemas, status codes, security).
3. Update `docs/api-examples.md` with a curl example.
4. Run `go test ./...`; `internal/api/swagger_test.go` checks that the UI, its
   assets, and the spec are served.
5. Rebuild and reload `/swagger/`.

The contract is hand-written. There is no annotation generator, so a stale spec
is possible. Treat the YAML as part of the change, not an afterthought.

## Production

Many teams expose docs publicly; others restrict them. If you do not want them
in production, block `/swagger/` at the edge or add a config flag. This project
keeps them on so the learning environment is easy to explore.

Swagger UI sends real requests. Point it at a development database, not
production data.

## Other ways to call the API

| Tool | Where | Use it for |
| --- | --- | --- |
| Swagger UI | `/swagger/` | exploring schemas and one-off calls |
| Bruno | `docs/bruno/gophersocial` | saved, git-friendly request collections |
| curl | `docs/api-examples.md` | copy-paste examples and scripts |
| Go tests | `tests/`, `internal/**/*_test.go` | automated regression coverage |
