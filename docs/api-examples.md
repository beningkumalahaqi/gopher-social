# API examples

Every example assumes the server is running on `http://localhost:8080` and the
seed has been applied (`make seed`). Responses are abbreviated where noted;
timestamps and IDs will differ.

Prefer a browser? Open **<http://localhost:8080/swagger/>** for the interactive
Swagger UI. It uses the same contract as these examples. See
[`swagger.md`](swagger.md).

Set a base URL once:

```bash
BASE=http://localhost:8080
```

## Health

```bash
curl -s $BASE/healthz
# {"status":"ok"}

curl -s $BASE/readyz
# {"status":"ready"}
```

## Register

```bash
curl -s -X POST $BASE/api/v1/auth/register \
  -H 'content-type: application/json' \
  -d '{
    "username": "zoe",
    "email": "zoe@example.com",
    "password": "Password123!",
    "display_name": "Zoe"
  }'
```

```json
{
  "data": {
    "user": {
      "id": "0d9c1a6e-...",
      "username": "zoe",
      "display_name": "Zoe",
      "bio": "",
      "avatar_url": null,
      "created_at": "2026-10-24T10:00:00Z",
      "email": "zoe@example.com"
    },
    "token_type": "Bearer",
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "rt_...",
    "expires_in": 899,
    "refresh_expires_at": "2026-11-23T10:00:00Z"
  }
}
```

Validation failure (`422`):

```json
{
  "error": {
    "code": "validation_error",
    "message": "validation failed",
    "details": [
      { "field": "email", "message": "must be a valid email address" }
    ],
    "request_id": "..."
  }
}
```

## Login

`login` accepts a username or an email.

```bash
curl -s -X POST $BASE/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"login":"alice","password":"Password123!"}'
```

Store the tokens:

```bash
ACCESS=$(curl -s -X POST $BASE/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"login":"alice","password":"Password123!"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["access_token"])')

REFRESH=$(curl -s -X POST $BASE/api/v1/auth/login \
  -H 'content-type: application/json' \
  -d '{"login":"alice","password":"Password123!"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["refresh_token"])')

AUTH="Authorization: Bearer $ACCESS"
```

Wrong credentials return `401` with `{"error":{"code":"unauthorized",...}}`.
The message does not distinguish unknown user from wrong password.

## Refresh and rotation

```bash
curl -s -X POST $BASE/api/v1/auth/refresh \
  -H 'content-type: application/json' \
  -d "{\"refresh_token\":\"$REFRESH\"}"
```

Returns a new access token **and** a new refresh token. The old refresh token is
now invalid. Presenting it again returns `401` and revokes the whole session
family — this is reuse detection. If you script this, update `$REFRESH` after
every call.

## Logout

```bash
curl -s -X POST $BASE/api/v1/auth/logout \
  -H 'content-type: application/json' \
  -d "{\"refresh_token\":\"$REFRESH\"}" -i
# HTTP/1.1 204 No Content
```

Logout does not require a bearer token; possession of the refresh token is the
credential. Access tokens remain valid until they expire.

## Profiles

```bash
curl -s $BASE/api/v1/users/alice
```

```json
{
  "data": {
    "id": "...",
    "username": "alice",
    "display_name": "Alice",
    "bio": "Building things in Go.",
    "avatar_url": null,
    "created_at": "...",
    "stats": { "posts": 3, "followers": 2, "following": 3 }
  }
}
```

Private profile (includes email):

```bash
curl -s $BASE/api/v1/users/me -H "$AUTH"
```

Update profile:

```bash
curl -s -X PATCH $BASE/api/v1/users/me -H "$AUTH" \
  -H 'content-type: application/json' \
  -d '{"display_name":"Alice G.","bio":"Gopher","avatar_url":"https://example.com/a.png"}'
```

Only provided fields change. Send `null` to clear `avatar_url`? No — the API
treats `null` and omission the same for optional strings. To clear, send an
empty string.

## Change password

```bash
curl -s -X PATCH $BASE/api/v1/users/me/password -H "$AUTH" \
  -H 'content-type: application/json' \
  -d '{"current_password":"Password123!","new_password":"NewPassword456!"}' -i
# HTTP/1.1 204 No Content
```

All refresh sessions for the user are revoked. Existing access tokens still work
until expiry.

## Follow graph

```bash
curl -s -X POST $BASE/api/v1/users/alice/follow -H "$AUTH" -i
# 204; idempotent, so repeating it is still 204

curl -s $BASE/api/v1/users/alice/follow-status -H "$AUTH"
# {"data":{"following":true,"followed_by":false}}

curl -s "$BASE/api/v1/users/alice/followers?limit=10&offset=0"
# {"data":[{...}],"meta":{"limit":10,"offset":0,"has_more":false,"next_offset":null}}

curl -s -X DELETE $BASE/api/v1/users/alice/follow -H "$AUTH" -i
# 204; unfollowing a non-followed user is also 204

curl -s "$BASE/api/v1/users/alice/suggestions?limit=5"
# {"data":[...],"meta":{"limit":5}}
```

Following yourself returns `422`. Unknown usernames return `404`.

## Posts

Create a published post:

```bash
curl -s -X POST $BASE/api/v1/posts -H "$AUTH" \
  -H 'content-type: application/json' \
  -d '{"content":"Hello from GopherSocial","image_url":"https://example.com/pic.png"}'
```

```json
{
  "data": {
    "id": "...",
    "content": "Hello from GopherSocial",
    "image_url": "https://example.com/pic.png",
    "status": "published",
    "author": {
      "id": "...",
      "username": "alice",
      "display_name": "Alice",
      "avatar_url": null
    },
    "comment_count": 0,
    "reaction_count": 0,
    "viewer_reaction": null,
    "created_at": "...",
    "updated_at": "...",
    "published_at": "..."
  }
}
```

Draft:

```bash
curl -s -X POST $BASE/api/v1/posts -H "$AUTH" \
  -H 'content-type: application/json' \
  -d '{"content":"Not ready yet","status":"draft"}'
```

A draft is visible only to its author. Another user gets `404`, not `403`.

Publish later:

```bash
curl -s -X PATCH $BASE/api/v1/posts/$POST_ID -H "$AUTH" \
  -H 'content-type: application/json' \
  -d '{"status":"published","content":"Ready now"}'
```

Edit rules: only the author may PATCH/DELETE; the handler returns `403` for a
post the caller can see but does not own, and `404` for a post they cannot see.

List with filters:

```bash
curl -s "$BASE/api/v1/posts?limit=20&offset=0&status=published&author=alice"
```

`status` is `draft` or `published`; drafts are only returned to their author.
Anonymous requests never see drafts.

```json
{
  "data": [ { "...": "..." } ],
  "meta": { "limit": 20, "offset": 0, "has_more": true, "next_offset": 20 }
}
```

Get one:

```bash
curl -s $BASE/api/v1/posts/$POST_ID -H "$AUTH"
```

Delete:

```bash
curl -s -X DELETE $BASE/api/v1/posts/$POST_ID -H "$AUTH" -i
# 204
```

Deleting a post cascades comments, reactions, and notifications.

## Feed and explore

Feed requires auth and includes followed users plus yourself:

```bash
curl -s "$BASE/api/v1/feed?limit=20" -H "$AUTH"
```

```json
{
  "data": [ { "...": "..." } ],
  "meta": { "limit": 20, "next_cursor": "MjAyNi0xMC0yNFQxMDowMDowMFp8..." }
}
```

Next page:

```bash
curl -s "$BASE/api/v1/feed?limit=20&cursor=$CURSOR" -H "$AUTH"
```

Stop when `next_cursor` is `null`. Cursors are opaque; do not construct them.

Explore is public and contains published posts only:

```bash
curl -s "$BASE/api/v1/explore?limit=20"
```

An invalid cursor returns `400`; `limit` outside `1..100` returns `422`.

## Comments

```bash
curl -s -X POST $BASE/api/v1/posts/$POST_ID/comments -H "$AUTH" \
  -H 'content-type: application/json' \
  -d '{"content":"Great post!"}'
```

```json
{
  "data": {
    "id": "...",
    "post_id": "...",
    "content": "Great post!",
    "author": { "id": "...", "username": "bob", "display_name": "Bob", "avatar_url": null },
    "created_at": "...",
    "updated_at": "..."
  }
}
```

List (public):

```bash
curl -s "$BASE/api/v1/posts/$POST_ID/comments?limit=20&offset=0"
```

Edit your own:

```bash
curl -s -X PATCH $BASE/api/v1/comments/$COMMENT_ID -H "$AUTH" \
  -H 'content-type: application/json' -d '{"content":"Edited"}'
```

Delete your own comment, or any comment on your own post:

```bash
curl -s -X DELETE $BASE/api/v1/comments/$COMMENT_ID -H "$AUTH" -i
# 204
```

Other users get `403`.

## Reactions

One reaction per user per post. `PUT` is an upsert, so changing the type replaces
the previous one.

```bash
curl -s -X PUT $BASE/api/v1/posts/$POST_ID/reaction -H "$AUTH" \
  -H 'content-type: application/json' -d '{"type":"love"}'
```

```json
{
  "data": {
    "total": 1,
    "counts": { "like": 0, "love": 1, "laugh": 0 },
    "viewer_reaction": "love"
  }
}
```

Allowed types: `like`, `love`, `laugh`.

```bash
curl -s -X DELETE $BASE/api/v1/posts/$POST_ID/reaction -H "$AUTH" -i
# 204; idempotent

curl -s "$BASE/api/v1/posts/$POST_ID/reactions?type=love&limit=20"
# {"data":[{ "post_id":"...", "type":"love", "user":{...}, ... }],"meta":{...}}

curl -s "$BASE/api/v1/posts/$POST_ID/reaction-summary" -H "$AUTH"
```

`viewer_reaction` is `null` for anonymous requests or when the viewer has not
reacted.

## Search

Users (username prefix and display-name match):

```bash
curl -s "$BASE/api/v1/search/users?q=ali&limit=10"
```

Posts (PostgreSQL full-text search over content):

```bash
curl -s "$BASE/api/v1/search/posts?q=golang&limit=10" -H "$AUTH"
```

`q` must be at least 2 characters (422 otherwise). Post search only returns
published posts; drafts are never searchable.

## Notifications

Follows, comments, and reactions create notifications for the target user.
Delivery is asynchronous: poll briefly if you script it.

```bash
curl -s "$BASE/api/v1/notifications?limit=20" -H "$AUTH"
```

```json
{
  "data": [
    {
      "id": "...",
      "type": "follow",
      "actor": { "id": "...", "username": "bob", "display_name": "Bob", "avatar_url": null },
      "post_id": null,
      "comment_id": null,
      "read_at": null,
      "created_at": "..."
    }
  ],
  "meta": { "limit": 20, "offset": 0, "has_more": false, "next_offset": null }
}
```

Unread only:

```bash
curl -s "$BASE/api/v1/notifications?unread=true" -H "$AUTH"
```

Unread count:

```bash
curl -s $BASE/api/v1/notifications/unread-count -H "$AUTH"
# {"data":{"unread":3}}
```

Mark one read:

```bash
curl -s -X PATCH $BASE/api/v1/notifications/$NOTIFICATION_ID/read -H "$AUTH" -i
# 204
```

Mark all read:

```bash
curl -s -X POST $BASE/api/v1/notifications/read-all -H "$AUTH"
# {"data":{"updated":3}}
```

Notification types: `follow`, `comment`, `reaction`.

## Error reference

| Status | Code | When |
| --- | --- | --- |
| 400 | `bad_request` | malformed JSON, unknown field, invalid UUID/cursor |
| 401 | `unauthorized` | missing/expired/invalid token, bad credentials |
| 403 | `forbidden` | authenticated but not allowed (edit someone else's post) |
| 404 | `not_found` | missing resource, or a draft the viewer cannot see |
| 405 | `method_not_allowed` | wrong method; `Allow` header is set |
| 409 | `conflict` | duplicate username/email |
| 413 | `payload_too_large` | body over `MAX_BODY_BYTES` |
| 422 | `validation_error` | field validation, bad limit/offset/status |
| 429 | `rate_limited` | auth endpoint rate limit exceeded; `Retry-After` set |
| 500 | `internal_error` | unexpected server error (details hidden) |

Every error has `request_id`; use it to correlate with logs.

## Full shell walkthrough

```bash
BASE=http://localhost:8080

login() {
  curl -s -X POST $BASE/api/v1/auth/login \
    -H 'content-type: application/json' \
    -d "{\"login\":\"$1\",\"password\":\"Password123!\"}"
}

ALICE_JSON=$(login alice)
BOB_JSON=$(login bob)
ALICE=$(echo "$ALICE_JSON" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["access_token"])')
BOB=$(echo "$BOB_JSON" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["access_token"])')

POST=$(curl -s -X POST $BASE/api/v1/posts -H "Authorization: Bearer $ALICE" \
  -H 'content-type: application/json' \
  -d '{"content":"Walkthrough post"}' \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["id"])')

curl -s -X POST $BASE/api/v1/users/alice/follow -H "Authorization: Bearer $BOB" -o /dev/null -w '%{http_code}\n'
curl -s -X POST $BASE/api/v1/posts/$POST/comments -H "Authorization: Bearer $BOB" \
  -H 'content-type: application/json' -d '{"content":"First!"}' -o /dev/null -w '%{http_code}\n'
curl -s -X PUT $BASE/api/v1/posts/$POST/reaction -H "Authorization: Bearer $BOB" \
  -H 'content-type: application/json' -d '{"type":"like"}' -o /dev/null -w '%{http_code}\n'

sleep 1
curl -s $BASE/api/v1/notifications/unread-count -H "Authorization: Bearer $ALICE"
```
